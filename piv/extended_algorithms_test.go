package piv

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdh"
	"crypto/ed25519"
	"fmt"
	"testing"

	"github.com/telesma-app/iso7816"
	"github.com/telesma-app/piv/card"
	"github.com/telesma-app/piv/protocol"
)

func TestCurve25519KeyWorkflows(t *testing.T) {
	seed := bytes.Repeat([]byte{1}, 32)
	ed := ed25519.NewKeyFromSeed(seed)

	x, err := ecdh.X25519().NewPrivateKey(seed)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name      string
		algorithm protocol.Algorithm
		wireID    byte
		importTag byte
		private   crypto.PrivateKey
		public    []byte
	}{
		{name: "Ed25519", algorithm: protocol.Ed25519, wireID: 0xe0, importTag: 0x07, private: ed, public: ed.Public().(ed25519.PublicKey)},
		{name: "X25519", algorithm: protocol.X25519, wireID: 0xe1, importTag: 0x08, private: x, public: x.PublicKey().Bytes()},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			current := card.AdministrationKey{Algorithm: protocol.AES192, Key: bytes.Repeat([]byte{0x31}, 24)}
			calls := 0

			publicData := append([]byte{0x86, 0x20}, test.public...)
			transport := administrationTransportFunc(func(gotCtx context.Context, command iso7816.Command, key card.AdministrationKey, authentication card.AuthenticationSource) (iso7816.Response, error) {
				calls++

				if gotCtx != ctx || key.Algorithm != current.Algorithm || !bytes.Equal(key.Key, current.Key) || authentication != nil {
					t.Fatal("administration inputs changed")
				}

				switch calls {
				case 1:
					want := append([]byte{test.importTag, 0x20}, seed...)

					want = append(want, 0xaa, 0x01, 0x02, 0xab, 0x01, 0x02)
					if command.INS != 0xfe || command.P1 != test.wireID || command.P2 != 0x9d || !bytes.Equal(command.Data, want) {
						t.Fatalf("IMPORT KEY command = %#v", command)
					}

					return iso7816.Response{Status: 0x9000}, nil
				case 2:
					want := []byte{0xac, 0x09, 0x80, 0x01, test.wireID, 0xaa, 0x01, 0x02, 0xab, 0x01, 0x02}
					if command.INS != 0x47 || command.P1 != 0 || command.P2 != 0x9d || !bytes.Equal(command.Data, want) {
						t.Fatalf("GENERATE KEY command = %#v", command)
					}

					return iso7816.Response{Status: 0x9000, Data: append([]byte{0x7f, 0x49, 0x22}, publicData...)}, nil
				default:
					return iso7816.Response{}, fmt.Errorf("unexpected administration command %d", calls)
				}
			})
			policy := KeyPolicy{PIN: PINPolicyOnce, Touch: TouchPolicyAlways}

			if err := ImportKey(ctx, transport, protocol.KeyManagement, test.private, policy, current); err != nil {
				t.Fatal(err)
			}

			public, err := GenerateKey(ctx, transport, GenerateKeyRequest{Key: protocol.KeyManagement, Algorithm: test.algorithm, Policy: policy}, current)
			if err != nil {
				t.Fatal(err)
			}

			checkCurve25519PublicKey(t, test.algorithm, public, test.public)

			metadata, err := ReadMetadata(ctx, vendorTransportFunc(func(_ context.Context, command iso7816.Command) (iso7816.Response, error) {
				if command.INS != 0xf7 || command.P1 != 0 || command.P2 != 0x9d || len(command.Data) != 0 {
					t.Fatalf("GET METADATA command = %#v", command)
				}

				data := append([]byte{0x01, 0x01, test.wireID, 0x04, 0x22}, publicData...)

				return iso7816.Response{Status: 0x9000, Data: data}, nil
			}), protocol.KeyManagement)
			if err != nil {
				t.Fatal(err)
			}

			public, err = metadata.PublicKey()
			if err != nil {
				t.Fatal(err)
			}

			checkCurve25519PublicKey(t, test.algorithm, public, test.public)

			if calls != 2 {
				t.Errorf("administration calls = %d, want 2", calls)
			}
		})
	}
}

func checkCurve25519PublicKey(t testing.TB, algorithm protocol.Algorithm, public crypto.PublicKey, want []byte) {
	t.Helper()

	if algorithm == protocol.Ed25519 {
		key, ok := public.(ed25519.PublicKey)
		if !ok || !bytes.Equal(key, want) {
			t.Fatalf("Ed25519 public key = %v", public)
		}

		return
	}

	key, ok := public.(*ecdh.PublicKey)
	if !ok || key.Curve() != ecdh.X25519() || !bytes.Equal(key.Bytes(), want) {
		t.Fatalf("X25519 public key = %v", public)
	}
}
