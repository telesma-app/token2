package pcsc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/telesma-app/iso7816"
	nativepcsc "github.com/telesma-app/pcsc"
	"github.com/telesma-app/token2"
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := config.Raw, data; (got == nil) != (want == nil) || !bytes.Equal(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	if got, want := config.TransferType, token2.TransferType(0x02); got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}

	if got, want := config.DeviceConfiguration, token2.DeviceConfiguration(0x2a); got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}

	if got := card.steps; len(got) != 0 {
		t.Errorf("got non-empty value %#v", got)
	}

	if got, want := card.begins, 1; got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}

	if got, want := card.ends, 1; got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}

	if got, want := len(card.contexts), 2; got != want {
		t.Fatalf("got length %d, want %d", got, want)
	}

	for _, got := range card.contexts {
		{
			want, got := "config", got.Value(contextKey{})
			gotValue, ok := got.(string)

			if !ok || gotValue != want {
				t.Errorf("got %#v, want %#v", got, want)
			}
		}
	}
}

func TestConfigurationRejectsFailedSelect(t *testing.T) {
	card := &scriptedCard{steps: []cardStep{
		{command: selectOTPAPDU, response: statusResponse(0x6a82)},
	}}
	device := &Device{card: card}

	_, err := device.Configuration(t.Context())

	var statusErr *iso7816.APDUError
	if err := err; !errors.As(err, &statusErr) {
		t.Fatalf("error %v does not match requested type", err)
	}

	if err, target := err, ErrOTPApplicationNotAvailable; !errors.Is(err, target) {
		t.Fatalf("got error %v, want errors.Is(error, %#v)", err, target)
	}

	if container, element := err.Error(), "select Token2 OTP application"; !strings.Contains(container, element) {
		t.Errorf("value does not contain %#v", element)
	}

	if got := card.steps; len(got) != 0 {
		t.Errorf("got non-empty value %#v", got)
	}

	if got, want := len(card.sent), 1; got != want {
		t.Errorf("got length %d, want %d", got, want)
	}
}

func TestConfigurationRejectsFailedTransaction(t *testing.T) {
	want := errors.New("transaction unavailable")
	card := &scriptedCard{beginErr: want}

	_, err := (&Device{card: card}).Configuration(t.Context())

	if err, target := err, want; !errors.Is(err, target) {
		t.Fatalf("got error %v, want errors.Is(error, %#v)", err, target)
	}

	if got, want := card.begins, 1; got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}

	if got := card.ends; got != 0 {
		t.Errorf("got %#v, want zero value", got)
	}

	if got := card.sent; len(got) != 0 {
		t.Errorf("got non-empty value %#v", got)
	}
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

	if err, target := err, want; !errors.Is(err, target) {
		t.Fatalf("got error %v, want errors.Is(error, %#v)", err, target)
	}

	if got, want := card.begins, 1; got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}

	if got, want := card.ends, 1; got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
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
				_, err := d.SerialNumber(t.Context())
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
				_, err := d.SerialNumber(t.Context())
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
				_, err := d.SerialNumber(t.Context())
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &scriptedCard{steps: tt.steps}
			err := tt.call(&Device{card: card})

			var statusErr *iso7816.APDUError
			if err := err; !errors.As(err, &statusErr) {
				t.Fatalf("error %v does not match requested type", err)
			}

			if container, element := err.Error(), tt.operation; !strings.Contains(container, element) {
				t.Errorf("value does not contain %#v", element)
			}

			if got := card.steps; len(got) != 0 {
				t.Errorf("got non-empty value %#v", got)
			}
		})
	}
}

func TestSerialNumberSequence(t *testing.T) {
	for _, borrowed := range []bool{false, true} {
		t.Run(fmt.Sprintf("borrowed=%t", borrowed), func(t *testing.T) {
			serialData := []byte{0xd1, 0x0e, '7', '2', '1', '0', '2', '9', '3', '5', '7', '8', '0', '5', '2', '8'}
			card := &scriptedCard{steps: []cardStep{
				{command: selectOTPAPDU, response: successfulResponse(nil)},
				{command: serialAPDU, response: successfulResponse(serialData)},
			}}
			device := &Device{card: card}

			var serial string

			var err error

			transactions := 1
			if borrowed {
				transactions = 0
				serial, err = ReadSerialNumber(t.Context(), card)
			} else {
				serial, err = device.SerialNumber(t.Context())
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got, want := serial, "72102935780528"; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}

			if got := card.steps; len(got) != 0 {
				t.Errorf("got non-empty value %#v", got)
			}

			if got, want := card.begins, transactions; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}

			if got, want := card.ends, transactions; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}

			if card.closed {
				t.Fatal("serial read closed the borrowed card")
			}
		})
	}
}

func TestSerialNumberSequenceFallsBackToLegacyPrelude(t *testing.T) {
	for _, borrowed := range []bool{false, true} {
		t.Run(fmt.Sprintf("borrowed=%t", borrowed), func(t *testing.T) {
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

			var serial string

			var err error

			transactions := 1
			if borrowed {
				transactions = 0
				serial, err = ReadSerialNumber(t.Context(), card)
			} else {
				serial, err = device.SerialNumber(t.Context())
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got, want := serial, "76105044935356"; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}

			if got := card.steps; len(got) != 0 {
				t.Errorf("got non-empty value %#v", got)
			}

			if got, want := card.begins, transactions; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}

			if got, want := card.ends, transactions; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}

			if card.closed {
				t.Fatal("serial read closed the borrowed card")
			}
		})
	}
}

func TestSerialNumberSequenceFallsBackToFIDO(t *testing.T) {
	for _, borrowed := range []bool{false, true} {
		t.Run(fmt.Sprintf("borrowed=%t", borrowed), func(t *testing.T) {
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

			var serial string

			var err error

			transactions := 1
			if borrowed {
				transactions = 0
				serial, err = ReadSerialNumber(t.Context(), card)
			} else {
				serial, err = device.SerialNumber(t.Context())
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got, want := serial, "76105044935356"; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}

			if got := card.steps; len(got) != 0 {
				t.Errorf("got non-empty value %#v", got)
			}

			if got, want := card.begins, transactions; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}

			if got, want := card.ends, transactions; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}

			if card.closed {
				t.Fatal("serial read closed the borrowed card")
			}
		})
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := info.Identity.ProductID, uint16(0x0016); got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}

	if got, want := info.Identity.SerialSuffix, "35780528"; got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestDeviceClose(t *testing.T) {
	closeErr := errors.New("close failed")
	card := &scriptedCard{closeErr: closeErr}
	device := &Device{card: card}

	err := device.Close()

	if err, target := err, closeErr; !errors.Is(err, target) {
		t.Errorf("got error %v, want errors.Is(error, %#v)", err, target)
	}

	if got := card.closed; !got {
		t.Errorf("got false, want true")
	}
}

func TestReadConfigurationBorrowsCard(t *testing.T) {
	card := &scriptedCard{steps: []cardStep{
		{command: selectOTPAPDU, response: successfulResponse(nil)},
		{command: configAPDU, response: successfulResponse([]byte{2})},
	}}

	configuration, err := ReadConfiguration(t.Context(), card)
	if err != nil || configuration.TransferType != 2 {
		t.Fatalf("configuration = %#v, error = %v", configuration, err)
	}

	if card.begins != 0 || card.ends != 0 || card.closed || len(card.steps) != 0 {
		t.Fatalf("borrowed lifecycle: begins = %d, ends = %d, closed = %t, remaining = %d", card.begins, card.ends, card.closed, len(card.steps))
	}
}

func TestSerialNumberRetainsValueAfterEndTransactionFailure(t *testing.T) {
	want := errors.New("end transaction failed")
	serial := "66202208969539"
	card := &scriptedCard{endErr: want, steps: []cardStep{
		{command: selectOTPAPDU, response: successfulResponse(nil)},
		{command: serialAPDU, response: successfulResponse(append([]byte{0xd1, byte(len(serial))}, serial...))},
	}}

	value, err := (&Device{card: card}).SerialNumber(t.Context())
	if value != serial || !errors.Is(err, want) {
		t.Fatalf("serial = %q, error = %v", value, err)
	}
}

func TestReadSerialNumberFIDOAfterSuccessfulLegacyPrelude(t *testing.T) {
	// Some generations accept the prelude but still require the FIDO applet.
	serial := "66202208969539"
	card := &scriptedCard{steps: []cardStep{
		{command: selectOTPAPDU, response: successfulResponse(nil)},
		{command: serialAPDU, response: statusResponse(0x6d00)},
		{command: legacySerialPreludeAPDU, response: successfulResponse([]byte{0})},
		{command: serialAPDU, response: statusResponse(0x6d00)},
		{command: selectFIDOAPDU, response: successfulResponse([]byte("U2F_V2"))},
		{command: serialAPDU, response: successfulResponse(append([]byte{0xd1, byte(len(serial))}, serial...))},
	}}

	value, err := ReadSerialNumber(t.Context(), card)
	if err != nil || value != serial || len(card.steps) != 0 {
		t.Fatalf("serial = %q, error = %v, remaining = %d", value, err, len(card.steps))
	}
}

func TestReadSerialNumberStopsAfterLegacyExchangeFailure(t *testing.T) {
	transmitErr := errors.New("transmit failed")
	for _, tt := range []struct {
		name     string
		response []byte
		err      error
		wantErr  error
	}{
		{name: "transport", err: transmitErr, wantErr: transmitErr},
		{name: "malformed response", response: []byte{0x90}, wantErr: iso7816.ErrInvalidResponse},
	} {
		t.Run(tt.name, func(t *testing.T) {
			card := &scriptedCard{steps: []cardStep{
				{command: selectOTPAPDU, response: successfulResponse(nil)},
				{command: serialAPDU, response: statusResponse(0x6d00)},
				{command: legacySerialPreludeAPDU, response: tt.response, err: tt.err},
			}}

			serial, err := ReadSerialNumber(t.Context(), card)

			if serial != "" || !errors.Is(err, tt.wantErr) {
				t.Fatalf("serial = %q, error = %v, want %v", serial, err, tt.wantErr)
			}

			if len(card.sent) != 3 || len(card.steps) != 0 {
				t.Fatalf("sent %d commands, remaining %d", len(card.sent), len(card.steps))
			}
		})
	}
}

func TestReadSerialNumberPreservesLegacyAndFIDOFailures(t *testing.T) {
	transmitErr := errors.New("select FIDO failed")
	card := &scriptedCard{steps: []cardStep{
		{command: selectOTPAPDU, response: successfulResponse(nil)},
		{command: serialAPDU, response: statusResponse(0x6d00)},
		{command: legacySerialPreludeAPDU, response: statusResponse(0x6985)},
		{command: selectFIDOAPDU, err: transmitErr},
	}}

	serial, err := ReadSerialNumber(t.Context(), card)

	var statusErr *iso7816.APDUError
	if serial != "" || !errors.Is(err, transmitErr) || !errors.As(err, &statusErr) || statusErr.StatusWord() != 0x6985 {
		t.Fatalf("serial = %q, error = %v", serial, err)
	}

	if len(card.steps) != 0 {
		t.Fatalf("remaining commands: %d", len(card.steps))
	}
}
