package hid

import (
	"os"
	"testing"
)

func TestHardware(t *testing.T) {
	path := os.Getenv("TOKEN2_HID_TEST_PATH")
	if path == "" {
		t.Skip("set TOKEN2_HID_TEST_PATH to run the HID hardware test")
	}

	device, err := Open(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() {
		if err := device.Close(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	serial, err := device.SerialNumber(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := serial; len(got) == 0 {
		t.Fatalf("got empty value %#v, want non-empty", got)
	}
}
