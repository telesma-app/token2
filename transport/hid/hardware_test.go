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

	info, err := device.DeviceInfo(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := info.SerialNumber; len(got) == 0 {
		t.Fatalf("got empty value %#v, want non-empty", got)
	}
	if got := info.ModelName(""); len(got) == 0 {
		t.Fatalf("got empty value %#v, want non-empty", got)
	}
}
