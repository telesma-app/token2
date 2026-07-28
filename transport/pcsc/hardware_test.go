package pcsc

import (
	"errors"
	"os"
	"testing"

	"github.com/go-ctap/token2"
	"github.com/go-ctap/token2/apdu"
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
	serialNumber, err := device.SerialNumber(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, serialNumber)

	config, err := device.Config(t.Context())
	if err != nil {
		var status *apdu.StatusError
		require.ErrorAs(t, err, &status)
		require.Contains(t, []uint16{0x6a86, 0x6d00}, status.SW)
		t.Logf("configuration is not supported: %v", err)
	} else {
		require.NotEmpty(t, config.Raw)
	}

	atr, err := device.ATRInfo(t.Context())
	if errors.Is(err, token2.ErrInvalidATR) {
		t.Logf("ATR does not contain Token2 identity data: %v", err)
	} else {
		require.NoError(t, err)
		require.NotEmpty(t, atr.Raw)
	}
}
