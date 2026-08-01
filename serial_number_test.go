package token2

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSerialNumber(t *testing.T) {
	serial, err := ParseSerialNumber([]byte{0xd1, 0x0e, '7', '2', '1', '0', '2', '9', '3', '5', '7', '8', '0', '5', '2', '8'})
	require.NoError(t, err)
	assert.Equal(t, "72102935780528", serial)
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
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrInvalidSerialResponse))
	}
}
