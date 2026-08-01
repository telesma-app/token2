package token2

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfiguration(t *testing.T) {
	config, err := ParseConfiguration([]byte{0x02, 0x2a, 0x86, 0x01, 0x10, 0x00, 0x02, 0x01, 0x02, 0x37})
	require.NoError(t, err)
	assert.Equal(t, Appearance{0x86, 0x01, 0x10, 0x00}, config.Appearance)
	assert.Equal(t, FIDOVersion{2, 1, 2}, config.FIDOVersion)
	assert.Equal(t, byte(0x2a), config.DeviceConfiguration)
	assert.Equal(t, byte(0x37), config.DeviceExtension)
}

func TestParseConfigurationLegacy(t *testing.T) {
	config, err := ParseConfiguration([]byte{0x02})
	require.NoError(t, err)
	assert.Equal(t, byte(0x02), config.TransferType)
}

func TestConfigurationJSONContainsRawMasks(t *testing.T) {
	encoded, err := json.Marshal(Configuration{
		DeviceConfiguration: 0x2a,
		DeviceExtension:     0x37,
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"raw":null,
		"transferType":0,
		"deviceConfiguration":42,
		"appearance":[0,0,0,0],
		"fidoVersion":{"major":0,"minor":0,"patch":0},
		"deviceExtension":55
	}`, string(encoded))
}

func TestParseConfigurationRejectsTruncatedResponses(t *testing.T) {
	for length := 0; length < 10; length++ {
		if length == 1 {
			continue
		}

		_, err := ParseConfiguration(make([]byte, length))
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrInvalidConfiguration))
	}
}
