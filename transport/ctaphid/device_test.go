package ctaphid

import (
	"bytes"
	"context"
	"testing"

	lowlevel "github.com/go-ctap/ctap/transport/ctaphid"
	"github.com/go-ctap/token2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	reportSize    = 65
	initPacketBit = 0x80
)

type scriptedDevice struct {
	reads         *bytes.Reader
	writes        bytes.Buffer
	writeContexts []context.Context
	closed        bool
}

func (d *scriptedDevice) Read(_ context.Context, p []byte) (int, error) {
	return d.reads.Read(p)
}

func (d *scriptedDevice) Write(ctx context.Context, p []byte) (int, error) {
	d.writeContexts = append(d.writeContexts, ctx)
	return d.writes.Write(p)
}

func (d *scriptedDevice) Close() error {
	d.closed = true
	return nil
}

func TestATR(t *testing.T) {
	type contextKey struct{}
	atrCtx := context.WithValue(t.Context(), contextKey{}, "atr")
	cid := lowlevel.ChannelID{1, 2, 3, 4}
	atr := []byte{
		0x3b, 0xff, 0x18, 0x00, 0x00, 0x10, 0x80,
		0x86, 0x8e, 0x00, 0x16, 0x60, 0x00, 0x60,
		'3', '5', '7', '8', '0', '5', '2', '8',
	}

	device := newScriptedDevice(t, responseBytes(t, cid, CommandGetATR, atr))
	transport := &Device{transport: lowlevel.NewTransport(device, cid)}

	info, err := transport.ATR(atrCtx)
	require.NoError(t, err)
	assert.Equal(t, uint16(0x0016), info.ProductID)
	assert.Equal(t, "35780528", info.SerialSuffix)
	assert.Equal(t, atr, info.Raw)

	written := device.writes.Bytes()
	require.Len(t, written, reportSize)
	assert.Equal(t, byte(CommandGetATR)|initPacketBit, written[5])
	assert.Equal(t, cid[:], written[1:5])
	require.Len(t, device.writeContexts, 1)
	assert.Equal(t, "atr", device.writeContexts[0].Value(contextKey{}))
}

func TestATRRejectsMalformedResponse(t *testing.T) {
	cid := lowlevel.ChannelID{1, 2, 3, 4}
	device := newScriptedDevice(t, responseBytes(t, cid, CommandGetATR, []byte{1, 2, 3}))
	transport := &Device{transport: lowlevel.NewTransport(device, cid)}

	_, err := transport.ATR(t.Context())

	assert.ErrorIs(t, err, token2.ErrInvalidATR)
}

func TestClose(t *testing.T) {
	device := newScriptedDevice(t)
	transport := &Device{transport: lowlevel.NewTransport(device, lowlevel.ChannelID{})}

	require.NoError(t, transport.Close())
	assert.True(t, device.closed)
}

func newScriptedDevice(t testing.TB, responses ...[]byte) *scriptedDevice {
	t.Helper()
	return &scriptedDevice{reads: bytes.NewReader(bytes.Join(responses, nil))}
}

func responseBytes(t testing.TB, cid lowlevel.ChannelID, command lowlevel.Command, data []byte) []byte {
	t.Helper()

	message, err := lowlevel.NewMessage(cid, command, data)
	require.NoError(t, err)

	var reports bytes.Buffer
	_, err = message.WriteTo(&reports)
	require.NoError(t, err)

	encoded := reports.Bytes()
	var response []byte
	for len(encoded) > 0 {
		require.GreaterOrEqual(t, len(encoded), reportSize)
		response = append(response, encoded[1:reportSize]...)
		encoded = encoded[reportSize:]
	}

	return response
}

var _ lowlevel.Device = (*scriptedDevice)(nil)
