package pcsc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/telesma-app/iso7816"
	nativepcsc "github.com/telesma-app/pcsc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	selectOTPAPDU = []byte{
		0x00, 0xa4, 0x04, 0x00, 0x08,
		0xf0, 0x00, 0x00, 0x01, 0x4f, 0x74, 0x70, 0x01,
	}
	selectFIDOAPDU = []byte{
		0x00, 0xa4, 0x04, 0x00, 0x08,
		0xa0, 0x00, 0x00, 0x06, 0x47, 0x2f, 0x00, 0x01,
	}
	configAPDU = []byte{
		0x80, 0xc5, 0x02, 0x00, 0x0a,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	legacySerialPreludeAPDU = []byte{0x80, 0xc5, 0x03, 0x00, 0x01, 0x04}
	serialAPDU              = []byte{
		0x80, 0x33, 0x00, 0x00, 0x12,
		0xd1, 0x10,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	getLegacyPreludeResponseAPDU = []byte{0x80, 0xc0, 0x00, 0x00, 0x01}
	getSerialResponseAPDU        = []byte{0x80, 0xc0, 0x00, 0x00, 0x10}
)

type cardStep struct {
	command  []byte
	response []byte
	err      error
}

type scriptedCard struct {
	steps               []cardStep
	sent                [][]byte
	status              *nativepcsc.CardStatus
	statusErr           error
	closeErr            error
	closed              bool
	contexts            []context.Context
	transactionContexts []context.Context
	beginErr            error
	endErr              error
	begins              int
	ends                int
}

func (c *scriptedCard) BeginTransaction(ctx context.Context) error {
	c.transactionContexts = append(c.transactionContexts, ctx)
	c.begins++

	return c.beginErr
}

func (c *scriptedCard) EndTransaction(nativepcsc.Disposition) error {
	c.ends++

	return c.endErr
}

func (c *scriptedCard) Transmit(ctx context.Context, command []byte) ([]byte, error) {
	c.contexts = append(c.contexts, ctx)
	c.sent = append(c.sent, append([]byte(nil), command...))

	if len(c.steps) == 0 {
		return nil, fmt.Errorf("unexpected APDU: %x", command)
	}

	step := c.steps[0]
	c.steps = c.steps[1:]
	if !bytes.Equal(command, step.command) {
		return nil, fmt.Errorf("unexpected APDU: got %x, want %x", command, step.command)
	}

	return append([]byte(nil), step.response...), step.err
}

func (c *scriptedCard) Status() (*nativepcsc.CardStatus, error) {
	return c.status, c.statusErr
}

func (c *scriptedCard) Close() error {
	c.closed = true
	return c.closeErr
}

func successfulResponse(data []byte) []byte {
	return append(append([]byte(nil), data...), 0x90, 0x00)
}

func statusResponse(status uint16) []byte {
	return []byte{byte(status >> 8), byte(status)}
}

func TestConfiguration(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "config")
	data := []byte{0x02, 0x2a, 0x86, 0x01, 0x10, 0x00, 0x02, 0x01, 0x02, 0x37}
	card := &scriptedCard{steps: []cardStep{
		{command: selectOTPAPDU, response: successfulResponse(nil)},
		{command: configAPDU, response: successfulResponse(data)},
	}}
	device := &Device{card: card}

	config, err := device.Configuration(ctx)
	require.NoError(t, err)

	assert.Equal(t, data, config.Raw)
	assert.Equal(t, byte(0x02), config.TransferType)
	assert.Equal(t, byte(0x2a), config.DeviceConfiguration)
	assert.Empty(t, card.steps)
	assert.Equal(t, 1, card.begins)
	assert.Equal(t, 1, card.ends)
	require.Len(t, card.contexts, 2)
	for _, got := range card.contexts {
		assert.Equal(t, "config", got.Value(contextKey{}))
	}
}

func TestConfigurationRejectsFailedSelect(t *testing.T) {
	card := &scriptedCard{steps: []cardStep{
		{command: selectOTPAPDU, response: statusResponse(0x6a82)},
	}}
	device := &Device{card: card}

	_, err := device.Configuration(t.Context())

	var statusErr *iso7816.APDUError
	require.ErrorAs(t, err, &statusErr)
	require.ErrorIs(t, err, ErrOTPApplicationNotAvailable)
	assert.Contains(t, err.Error(), "select Token2 OTP application")
	assert.Empty(t, card.steps)
	assert.Len(t, card.sent, 1)
}

func TestConfigurationRejectsFailedTransaction(t *testing.T) {
	want := errors.New("transaction unavailable")
	card := &scriptedCard{beginErr: want}

	_, err := (&Device{card: card}).Configuration(t.Context())

	require.ErrorIs(t, err, want)
	assert.Equal(t, 1, card.begins)
	assert.Zero(t, card.ends)
	assert.Empty(t, card.sent)
}

func TestConfigurationPropagatesEndTransactionFailure(t *testing.T) {
	want := errors.New("end transaction failed")
	card := &scriptedCard{
		steps: []cardStep{
			{command: selectOTPAPDU, response: successfulResponse(nil)},
			{command: configAPDU, response: successfulResponse([]byte{0x02})},
		},
		endErr: want,
	}

	_, err := (&Device{card: card}).Configuration(t.Context())

	require.ErrorIs(t, err, want)
	assert.Equal(t, 1, card.begins)
	assert.Equal(t, 1, card.ends)
}

func TestStatusErrors(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		steps     []cardStep
		call      func(*Device) error
	}{
		{
			name:      "configuration",
			operation: "read Token2 configuration",
			steps: []cardStep{
				{command: selectOTPAPDU, response: successfulResponse(nil)},
				{command: configAPDU, response: statusResponse(0x6985)},
			},
			call: func(d *Device) error {
				_, err := d.Configuration(t.Context())
				return err
			},
		},
		{
			name:      "OTP application for serial number",
			operation: "select Token2 OTP application",
			steps: []cardStep{
				{command: selectOTPAPDU, response: statusResponse(0x6a82)},
			},
			call: func(d *Device) error {
				_, err := d.DeviceInfo(t.Context())
				return err
			},
		},
		{
			name:      "FIDO application",
			operation: "select FIDO application",
			steps: []cardStep{
				{command: selectOTPAPDU, response: successfulResponse(nil)},
				{command: serialAPDU, response: statusResponse(0x6d00)},
				{command: legacySerialPreludeAPDU, response: statusResponse(0x6d00)},
				{command: selectFIDOAPDU, response: statusResponse(0x6a82)},
			},
			call: func(d *Device) error {
				_, err := d.DeviceInfo(t.Context())
				return err
			},
		},
		{
			name:      "serial number",
			operation: "read serial number",
			steps: []cardStep{
				{command: selectOTPAPDU, response: successfulResponse(nil)},
				{command: serialAPDU, response: statusResponse(0x6a80)},
			},
			call: func(d *Device) error {
				_, err := d.DeviceInfo(t.Context())
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &scriptedCard{steps: tt.steps}
			err := tt.call(&Device{card: card})

			var statusErr *iso7816.APDUError
			require.ErrorAs(t, err, &statusErr)
			assert.Contains(t, err.Error(), tt.operation)
			assert.Empty(t, card.steps)
		})
	}
}

func TestDeviceInfoSequence(t *testing.T) {
	serialData := []byte{0xd1, 0x0e, '7', '2', '1', '0', '2', '9', '3', '5', '7', '8', '0', '5', '2', '8'}
	card := &scriptedCard{steps: []cardStep{
		{command: selectOTPAPDU, response: successfulResponse(nil)},
		{command: serialAPDU, response: successfulResponse(serialData)},
	}}
	device := &Device{card: card}

	info, err := device.DeviceInfo(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "72102935780528", info.SerialNumber)
	assert.Equal(t, "Mini USB-C PIN+", info.FormFactor)
	assert.Empty(t, card.steps)
	assert.Equal(t, 1, card.begins)
	assert.Equal(t, 1, card.ends)
}

func TestDeviceInfoSequenceFallsBackToLegacyPrelude(t *testing.T) {
	serialData := []byte{0xd1, 0x0e, '7', '6', '1', '0', '5', '0', '4', '4', '9', '3', '5', '3', '5', '6'}
	card := &scriptedCard{steps: []cardStep{
		{command: selectOTPAPDU, response: successfulResponse(nil)},
		{command: serialAPDU, response: statusResponse(0x6d00)},
		{command: legacySerialPreludeAPDU, response: statusResponse(0x6101)},
		{command: getLegacyPreludeResponseAPDU, response: successfulResponse([]byte{0x00})},
		{command: serialAPDU, response: statusResponse(0x6110)},
		{command: getSerialResponseAPDU, response: successfulResponse(serialData)},
	}}
	device := &Device{card: card}

	info, err := device.DeviceInfo(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "76105044935356", info.SerialNumber)
	assert.Equal(t, "USB-A PIN+ NFC", info.FormFactor)
	assert.Empty(t, card.steps)
	assert.Equal(t, 1, card.begins)
	assert.Equal(t, 1, card.ends)
}

func TestDeviceInfoSequenceFallsBackToFIDO(t *testing.T) {
	serialData := []byte{0xd1, 0x0e, '7', '6', '1', '0', '5', '0', '4', '4', '9', '3', '5', '3', '5', '6'}
	card := &scriptedCard{steps: []cardStep{
		{command: selectOTPAPDU, response: successfulResponse(nil)},
		{command: serialAPDU, response: statusResponse(0x6d00)},
		{command: legacySerialPreludeAPDU, response: statusResponse(0x6d00)},
		{command: selectFIDOAPDU, response: successfulResponse([]byte("U2F_V2"))},
		{command: serialAPDU, response: statusResponse(0x6110)},
		{command: getSerialResponseAPDU, response: successfulResponse(serialData)},
	}}
	device := &Device{card: card}

	info, err := device.DeviceInfo(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "76105044935356", info.SerialNumber)
	assert.Equal(t, "USB-A PIN+ NFC", info.FormFactor)
	assert.Empty(t, card.steps)
	assert.Equal(t, 1, card.begins)
	assert.Equal(t, 1, card.ends)
}

func TestATR(t *testing.T) {
	atr := []byte{
		0x3b, 0xff, 0x18, 0x00, 0x00, 0x10, 0x80,
		0x86, 0x8e, 0x00, 0x16, 0x60, 0x00, 0x60,
		'3', '5', '7', '8', '0', '5', '2', '8',
	}
	card := &scriptedCard{status: &nativepcsc.CardStatus{ATR: atr}}
	device := &Device{card: card}

	info, err := device.ATR(t.Context())
	require.NoError(t, err)

	assert.Equal(t, uint16(0x0016), info.ProductID)
	assert.Equal(t, "35780528", info.SerialSuffix)
}

func TestDeviceClose(t *testing.T) {
	closeErr := errors.New("close failed")
	card := &scriptedCard{closeErr: closeErr}
	device := &Device{card: card}

	err := device.Close()

	assert.ErrorIs(t, err, closeErr)
	assert.True(t, card.closed)
}
