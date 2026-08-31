package ctaphid

import (
	"bytes"
	"context"
	"errors"
	"testing"

	lowlevel "github.com/telesma-app/ctap/transport/ctaphid"
	"github.com/telesma-app/token2"
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := info.ProductID, uint16(0x0016); got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
	if got, want := info.SerialSuffix, "35780528"; got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
	if got, want := info.Raw, atr; (got == nil) != (want == nil) || !bytes.Equal(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	written := device.writes.Bytes()
	if got, want := len(written), reportSize; got != want {
		t.Fatalf("got length %d, want %d", got, want)
	}
	if got, want := written[5], byte(CommandGetATR)|initPacketBit; got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
	if got, want := written[1:5], cid[:]; (got == nil) != (want == nil) || !bytes.Equal(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
	if got, want := len(device.writeContexts), 1; got != want {
		t.Fatalf("got length %d, want %d", got, want)
	}
	{
		want, got := "atr", device.writeContexts[0].Value(contextKey{})
		gotValue, ok := got.(string)

		if !ok || gotValue != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
}

func TestATRRejectsMalformedResponse(t *testing.T) {
	cid := lowlevel.ChannelID{1, 2, 3, 4}
	device := newScriptedDevice(t, responseBytes(t, cid, CommandGetATR, []byte{1, 2, 3}))
	transport := &Device{transport: lowlevel.NewTransport(device, cid)}

	_, err := transport.ATR(t.Context())

	if err, target := err, token2.ErrInvalidATR; !errors.Is(err, target) {
		t.Errorf("got error %v, want errors.Is(error, %#v)", err, target)
	}
}

func TestClose(t *testing.T) {
	device := newScriptedDevice(t)
	transport := &Device{transport: lowlevel.NewTransport(device, lowlevel.ChannelID{})}

	if err := transport.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := device.closed; !got {
		t.Errorf("got false, want true")
	}
}

func newScriptedDevice(t testing.TB, responses ...[]byte) *scriptedDevice {
	t.Helper()
	return &scriptedDevice{reads: bytes.NewReader(bytes.Join(responses, nil))}
}

func responseBytes(t testing.TB, cid lowlevel.ChannelID, command lowlevel.Command, data []byte) []byte {
	t.Helper()

	message, err := lowlevel.NewMessage(cid, command, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var reports bytes.Buffer
	_, err = message.WriteTo(&reports)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	encoded := reports.Bytes()
	var response []byte
	for len(encoded) > 0 {
		{
			got, limit := len(encoded), reportSize
			if got < limit {
				t.Fatalf("got %v, want greater than or equal to %v", got, limit)
			}
		}
		response = append(response, encoded[1:reportSize]...)
		encoded = encoded[reportSize:]
	}

	return response
}

var _ lowlevel.Device = (*scriptedDevice)(nil)
