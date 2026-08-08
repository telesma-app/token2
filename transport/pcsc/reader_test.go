package pcsc

import (
	"context"
	"errors"
	"testing"

	"github.com/telesma-app/iso7816"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetReaderSoundLevel(t *testing.T) {
	tests := []struct {
		name  string
		level ReaderSoundLevel
		value byte
	}{
		{name: "off", level: ReaderSoundOff, value: 0x00},
		{name: "level 1", level: ReaderSoundLevel1, value: 0x01},
		{name: "level 2", level: ReaderSoundLevel2, value: 0x02},
		{name: "level 3", level: ReaderSoundLevel3, value: 0x03},
		{name: "level 4", level: ReaderSoundLevel4, value: 0x04},
		{name: "level 5", level: ReaderSoundLevel5, value: 0x05},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type contextKey struct{}
			ctx := context.WithValue(t.Context(), contextKey{}, "sound")
			command := []byte{
				0x90, 0xff, 0x55, 0x77, 0x08,
				0x80, 0x33, 0x00, 0x00, 0x03, 0x02, 0x01, tt.value,
			}
			card := &scriptedCard{steps: []cardStep{
				{command: command, response: successfulResponse(nil)},
			}}

			err := (&Device{card: card}).SetReaderSoundLevel(ctx, tt.level)

			require.NoError(t, err)
			assert.Empty(t, card.steps)
			assert.Zero(t, card.begins)
			assert.Zero(t, card.ends)
			require.Len(t, card.contexts, 1)
			assert.Equal(t, "sound", card.contexts[0].Value(contextKey{}))
		})
	}
}

func TestSetReaderNFC(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		value   byte
	}{
		{name: "enabled", enabled: true, value: 0x00},
		{name: "disabled", enabled: false, value: 0x01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := []byte{
				0x90, 0xff, 0x55, 0x77, 0x08,
				0x80, 0x33, 0x00, 0x00, 0x03, 0x03, 0x01, tt.value,
			}
			card := &scriptedCard{steps: []cardStep{
				{command: command, response: successfulResponse(nil)},
			}}

			err := (&Device{card: card}).SetReaderNFC(t.Context(), tt.enabled)

			require.NoError(t, err)
			assert.Empty(t, card.steps)
			assert.Zero(t, card.begins)
			assert.Zero(t, card.ends)
		})
	}
}

func TestSetReaderSoundLevelRejectsInvalidLevel(t *testing.T) {
	card := &scriptedCard{}

	err := (&Device{card: card}).SetReaderSoundLevel(t.Context(), ReaderSoundLevel5+1)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sound level 6")
	assert.Empty(t, card.sent)
}

func TestSetReaderSettingErrors(t *testing.T) {
	command := []byte{
		0x90, 0xff, 0x55, 0x77, 0x08,
		0x80, 0x33, 0x00, 0x00, 0x03, 0x02, 0x01, 0x00,
	}
	transmitErr := errors.New("transmit failed")

	tests := []struct {
		name     string
		response []byte
		err      error
		check    func(*testing.T, error)
	}{
		{
			name: "transport",
			err:  transmitErr,
			check: func(t *testing.T, err error) {
				require.ErrorIs(t, err, transmitErr)
			},
		},
		{
			name:     "malformed response",
			response: []byte{0x90},
			check: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid response APDU")
			},
		},
		{
			name:     "status",
			response: statusResponse(0x6d00),
			check: func(t *testing.T, err error) {
				var statusErr *iso7816.APDUError
				require.ErrorAs(t, err, &statusErr)
				assert.Equal(t, iso7816.StatusWord(0x6d00), statusErr.StatusWord())
				assert.Contains(t, err.Error(), "set Token2 reader sound level")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &scriptedCard{steps: []cardStep{
				{command: command, response: tt.response, err: tt.err},
			}}

			err := (&Device{card: card}).SetReaderSoundLevel(t.Context(), ReaderSoundOff)

			tt.check(t, err)
			assert.Empty(t, card.steps)
		})
	}
}
