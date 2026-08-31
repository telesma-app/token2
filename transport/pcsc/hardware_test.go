package pcsc

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/telesma-app/iso7816"
	"github.com/telesma-app/token2"
)

func TestHardware(t *testing.T) {
	reader := os.Getenv("TOKEN2_PCSC_TEST_READER")
	if reader == "" {
		t.Skip("set TOKEN2_PCSC_TEST_READER to run the PC/SC hardware test")
	}

	device, err := Open(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() {
		if err := device.Close(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Identity must remain readable even on generations that do not expose
	// the optional configuration command.
	info, err := device.DeviceInfo(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := info.SerialNumber; len(got) == 0 {
		t.Fatalf("got empty value %#v, want non-empty", got)
	}

	config, err := device.Configuration(t.Context())
	if err != nil {
		var status *iso7816.APDUError
		if err := err; !errors.As(err, &status) {
			t.Fatalf("error %v does not match requested type", err)
		}
		{
			container, element := []iso7816.StatusWord{0x6a86, 0x6d00}, status.StatusWord()
			if !slices.Contains(container, element) {
				t.Fatalf("value does not contain %#v", element)
			}
		}
		t.Logf("configuration is not supported: %v", err)
	} else {
		if got := config.Raw; len(got) == 0 {
			t.Fatalf("got empty value %#v, want non-empty", got)
		}
	}

	atr, err := device.ATR(t.Context())
	if errors.Is(err, token2.ErrInvalidATR) {
		t.Logf("ATR does not contain Token2 identity data: %v", err)
	} else {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := atr.Raw; len(got) == 0 {
			t.Fatalf("got empty value %#v, want non-empty", got)
		}
	}
}
