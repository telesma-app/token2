package ctaphid

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHardware(t *testing.T) {
	path := os.Getenv("TOKEN2_CTAPHID_TEST_PATH")
	if path == "" {
		t.Skip("set TOKEN2_CTAPHID_TEST_PATH to run the CTAPHID hardware test")
	}

	device, err := Open(t.Context(), path)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, device.Close())
	})

	atr, err := device.ATR(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, atr.Raw)
	t.Logf("ATR: %x", atr.Raw)
	t.Logf("product ID: %04x, serial suffix: %s", atr.ProductID, atr.SerialSuffix)
}
