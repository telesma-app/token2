package piv

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/telesma-app/iso7816"
	"github.com/telesma-app/piv/protocol"
)

type serialTransport struct {
	t        *testing.T
	response iso7816.Response
	err      error
}

func (transport serialTransport) Exchange(_ context.Context, command iso7816.Command) (iso7816.Response, error) {
	transport.t.Helper()

	want := iso7816.Command{INS: 0xf8, Le: 256, Encoding: iso7816.EncodingShort}
	if !reflect.DeepEqual(command, want) {
		transport.t.Fatalf("command = %#v, want %#v", command, want)
	}

	return transport.response, transport.err
}

func TestReadSerialSuffix(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
		want string
	}{
		{name: "packed decimal rather than binary", data: []byte{0x39, 0x53, 0x96, 0x12}, want: "39539612"},
		{name: "leading zeros", data: []byte{0x00, 0x01, 0x23, 0x45}, want: "00012345"},
		{name: "all zero digits", data: []byte{0, 0, 0, 0}, want: "00000000"},
	} {
		t.Run(test.name, func(t *testing.T) {
			transport := serialTransport{t: t, response: iso7816.Response{Data: test.data, Status: 0x9000}}

			got, err := ReadSerialSuffix(t.Context(), transport)

			if err != nil || got != test.want {
				t.Fatalf("ReadSerialSuffix = %q, %v, want %q, nil", got, err, test.want)
			}
		})
	}
}

func TestReadSerialSuffixInvalidResponse(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "short", data: []byte{0, 0, 0}},
		{name: "trailing bytes", data: []byte{0, 0, 0, 0, 0}},
		{name: "nondecimal high digit", data: []byte{0xa1, 0, 0, 0}},
		{name: "nondecimal low digit", data: []byte{0, 0, 0, 0x1f}},
	} {
		t.Run(test.name, func(t *testing.T) {
			transport := serialTransport{t: t, response: iso7816.Response{Data: test.data, Status: 0x9000}}

			got, err := ReadSerialSuffix(t.Context(), transport)

			if got != "" || !errors.Is(err, protocol.ErrInvalidResponse) {
				t.Fatalf("ReadSerialSuffix = %q, %v, want invalid response", got, err)
			}
		})
	}
}

func TestReadSerialSuffixError(t *testing.T) {
	t.Run("transport cause", func(t *testing.T) {
		transport := serialTransport{t: t, err: context.Canceled}

		_, err := ReadSerialSuffix(t.Context(), transport)

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ReadSerialSuffix error = %v, want context.Canceled", err)
		}
	})

	t.Run("card status before payload", func(t *testing.T) {
		transport := serialTransport{t: t, response: iso7816.Response{Status: 0x6d00}}

		_, err := ReadSerialSuffix(t.Context(), transport)

		var apduError *iso7816.APDUError
		if !errors.As(err, &apduError) || apduError.StatusWord() != 0x6d00 {
			t.Fatalf("ReadSerialSuffix error = %v, want APDU 6D00", err)
		}
	})
}
