package ctaphid

import (
	"os"
	"testing"
)

func TestHardware(t *testing.T) {
	path := os.Getenv("TOKEN2_CTAPHID_TEST_PATH")
	if path == "" {
		t.Skip("set TOKEN2_CTAPHID_TEST_PATH to run the CTAPHID hardware test")
	}

	device, err := Open(t.Context(), path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() {
		if err := device.Close(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	atr, err := device.ATR(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atr.Raw; len(got) == 0 {
		t.Fatalf("got empty value %#v, want non-empty", got)
	}
	t.Logf("ATR: %x", atr.Raw)
	t.Logf("product ID: %04x, serial suffix: %s", atr.ProductID, atr.SerialSuffix)
}
