package token2

import (
	"errors"
	"testing"
)

func TestParseSerialNumber(t *testing.T) {
	serial, err := ParseSerialNumber([]byte{0xd1, 0x0e, '7', '2', '1', '0', '2', '9', '3', '5', '7', '8', '0', '5', '2', '8'})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	{
		want, got := "72102935780528", serial
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
}

func TestParseSerialNumberRejectsMalformedData(t *testing.T) {
	tests := [][]byte{
		nil,
		{0xd1},
		{0xd2, 0},
		{0xd1, 2, '1'},
	}

	for _, response := range tests {
		_, err := ParseSerialNumber(response)
		if err == nil {
			t.Fatalf("expected an error")
		}
		if got := errors.Is(err, ErrInvalidSerialResponse); !got {
			t.Errorf("got false, want true")
		}
	}
}
