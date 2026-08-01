package hid

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHardware(t *testing.T) {
	path := os.Getenv("TOKEN2_HID_TEST_PATH")
	if path == "" {
		t.Skip("set TOKEN2_HID_TEST_PATH to run the HID hardware test")
	}

	device, err := Open(path)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, device.Close())
	})

	info, err := device.DeviceInfo(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, info.SerialNumber)
	require.NotEmpty(t, info.ModelName(""))
}
