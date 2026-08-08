package pcsc

import (
	"errors"
	"os"
	"testing"

	"github.com/telesma-app/iso7816"
	"github.com/telesma-app/token2"
	"github.com/stretchr/testify/require"
)

func TestHardware(t *testing.T) {
	reader := os.Getenv("TOKEN2_PCSC_TEST_READER")
	if reader == "" {
		t.Skip("set TOKEN2_PCSC_TEST_READER to run the PC/SC hardware test")
	}

	device, err := Open(reader)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, device.Close())
	})

	// Identity must remain readable even on generations that do not expose
	// the optional configuration command.
	info, err := device.DeviceInfo(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, info.SerialNumber)

	config, err := device.Configuration(t.Context())
	if err != nil {
		var status *iso7816.APDUError
		require.ErrorAs(t, err, &status)
		require.Contains(t, []iso7816.StatusWord{0x6a86, 0x6d00}, status.StatusWord())
		t.Logf("configuration is not supported: %v", err)
	} else {
		require.NotEmpty(t, config.Raw)
	}

	atr, err := device.ATR(t.Context())
	if errors.Is(err, token2.ErrInvalidATR) {
		t.Logf("ATR does not contain Token2 identity data: %v", err)
	} else {
		require.NoError(t, err)
		require.NotEmpty(t, atr.Raw)
	}
}
