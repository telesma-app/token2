package piv

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/telesma-app/iso7816"
	"github.com/telesma-app/piv/card"
	"github.com/telesma-app/piv/protocol"
)

type vendorTransportFunc func(context.Context, iso7816.Command) (iso7816.Response, error)

func (exchange vendorTransportFunc) Exchange(ctx context.Context, command iso7816.Command) (iso7816.Response, error) {
	return exchange(ctx, command)
}

type administrationTransportFunc func(context.Context, iso7816.Command, card.AdministrationKey, card.AuthenticationSource) (iso7816.Response, error)

func (exchange administrationTransportFunc) ExchangeAdministration(
	ctx context.Context,
	command iso7816.Command,
	key card.AdministrationKey,
	authentication card.AuthenticationSource,
) (iso7816.Response, error) {
	return exchange(ctx, command, key, authentication)
}

func TestReadVersion(t *testing.T) {
	transport := vendorTransportFunc(func(_ context.Context, command iso7816.Command) (iso7816.Response, error) {
		if command.CLA != 0 || command.INS != 0xfd || command.P1 != 0 || command.P2 != 0 || len(command.Data) != 0 {
			t.Fatalf("GET VERSION command = %#v", command)
		}

		return iso7816.Response{Data: []byte{5, 7, 3}, Status: 0x9000}, nil
	})

	version, err := ReadVersion(t.Context(), transport)

	if err != nil || version != (FirmwareVersion{Major: 5, Minor: 7, Build: 3}) {
		t.Fatalf("ReadVersion = %v, %v, want 5.7.3, nil", version, err)
	}
}

func TestSetManagementKey(t *testing.T) {
	ctx := t.Context()
	current := card.AdministrationKey{Algorithm: protocol.TripleDES, Key: bytes.Repeat([]byte{0x43}, 24)}
	replacement := card.AdministrationKey{Algorithm: protocol.AES192, Key: bytes.Repeat([]byte{0x76}, 24)}
	wantData := append([]byte{0x0a, 0x9b, 24}, bytes.Repeat([]byte{0x76}, 24)...)
	called := false
	transport := administrationTransportFunc(func(gotCtx context.Context, command iso7816.Command, key card.AdministrationKey, authentication card.AuthenticationSource) (iso7816.Response, error) {
		called = true

		if gotCtx != ctx || key.Algorithm != current.Algorithm || !bytes.Equal(key.Key, current.Key) || authentication != nil {
			t.Fatalf("administration inputs changed: key = %#v, authentication = %v", key, authentication != nil)
		}

		if command.INS != 0xff || command.P1 != 0xff || command.P2 != 0xfe || !bytes.Equal(command.Data, wantData) {
			t.Fatalf("SET MANAGEMENT KEY command = %#v", command)
		}

		return iso7816.Response{Status: 0x9000}, nil
	})

	err := SetManagementKey(ctx, transport, replacement, TouchPolicyAlways, current)

	if err != nil || !called {
		t.Fatalf("SetManagementKey = %v, called = %v", err, called)
	}

	if !bytes.Equal(replacement.Key, bytes.Repeat([]byte{0x76}, 24)) {
		t.Fatal("replacement management key was modified")
	}
}

func TestSetManagementKeyCachedPreservesCardRejection(t *testing.T) {
	calls := 0
	transport := administrationTransportFunc(func(_ context.Context, command iso7816.Command, _ card.AdministrationKey, _ card.AuthenticationSource) (iso7816.Response, error) {
		calls++

		if command.INS != 0xff || command.P1 != 0xff || command.P2 != 0xfd {
			t.Fatalf("cached SET MANAGEMENT KEY command = %#v", command)
		}

		return iso7816.Response{Status: 0x6a86}, nil
	})
	key := card.AdministrationKey{Algorithm: protocol.TripleDES, Key: bytes.Repeat([]byte{0x43}, 24)}

	err := SetManagementKey(t.Context(), transport, key, TouchPolicyCached, key)

	var apdu *iso7816.APDUError
	if !errors.As(err, &apdu) || apdu.StatusWord() != 0x6a86 || calls != 1 {
		t.Fatalf("SetManagementKey = %v, calls = %d, want card rejection 6A86 after one exchange", err, calls)
	}
}

func TestSetPINRetriesKeepsAuthenticationInAdministrationWorkflow(t *testing.T) {
	ctx := t.Context()
	key := card.AdministrationKey{Algorithm: protocol.AES128, Key: bytes.Repeat([]byte{0x91}, 16)}
	authenticationCalls := 0
	authentication := card.AuthenticationSource(func(gotCtx context.Context) (protocol.VerifyRequest, error) {
		authenticationCalls++

		if gotCtx != ctx {
			t.Fatal("authentication context changed")
		}

		return protocol.VerifyRequest{Reference: protocol.GlobalPIN, Data: []byte{0x86, 0x53}}, nil
	})
	transport := administrationTransportFunc(func(gotCtx context.Context, command iso7816.Command, administrationKey card.AdministrationKey, source card.AuthenticationSource) (iso7816.Response, error) {
		if authenticationCalls != 0 || source == nil {
			t.Fatal("authentication was not deferred to the administration workflow")
		}

		if command.INS != 0xfa || command.P1 != 5 || command.P2 != 7 || len(command.Data) != 0 {
			t.Fatalf("SET PIN RETRIES command = %#v", command)
		}

		if administrationKey.Algorithm != key.Algorithm || !bytes.Equal(administrationKey.Key, key.Key) {
			t.Fatal("management key was replaced or defaulted")
		}

		request, err := source(gotCtx)
		if err != nil || request.Reference != protocol.GlobalPIN || !bytes.Equal(request.Data, []byte{0x86, 0x53}) {
			t.Fatalf("authentication = %#v, %v", request, err)
		}

		return iso7816.Response{}, context.Canceled
	})

	err := SetPINRetries(ctx, transport, 5, 7, key, authentication)

	if !errors.Is(err, context.Canceled) || authenticationCalls != 1 {
		t.Fatalf("SetPINRetries = %v, authentication calls = %d", err, authenticationCalls)
	}
}
