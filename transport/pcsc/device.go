// Package pcsc provides access to Token2 devices over PC/SC.
package pcsc

import (
	"context"
	"errors"
	"sync"

	nativepcsc "github.com/go-ctap/pcsc"
	"github.com/go-ctap/token2"
	"github.com/go-ctap/token2/apdu"
	"github.com/go-ctap/token2/internal/protocol"
)

const statusInstructionNotSupported = 0x6d00

var (
	_ token2.SerialNumberDevice = (*Device)(nil)
	_ token2.ATRDevice          = (*Device)(nil)
)

type card interface {
	apdu.Transceiver
	BeginTransaction(context.Context) error
	EndTransaction(nativepcsc.Disposition) error
	Status() (*nativepcsc.CardStatus, error)
	Close() error
}

// Device is a Token2 device connected through a PC/SC reader.
type Device struct {
	mu   sync.Mutex
	card card
}

// Open connects to the Token2 device in reader.
func Open(reader string) (*Device, error) {
	card, err := nativepcsc.Open(reader)
	if err != nil {
		return nil, err
	}

	return &Device{card: card}, nil
}

// Close closes the PC/SC connection.
func (d *Device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.card.Close()
}

// ATRInfo returns information encoded in the device ATR.
func (d *Device) ATRInfo(_ context.Context) (token2.ATRInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	status, err := d.card.Status()
	if err != nil {
		return token2.ATRInfo{}, err
	}

	return token2.ParseATR(status.ATR)
}

// Config returns the Token2 device configuration.
func (d *Device) Config(ctx context.Context) (config token2.Config, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.card.BeginTransaction(ctx); err != nil {
		return token2.Config{}, err
	}
	defer func() {
		err = errors.Join(err, d.card.EndTransaction(nativepcsc.DispositionLeaveCard))
	}()

	return d.config(ctx)
}

func (d *Device) config(ctx context.Context) (token2.Config, error) {
	if err := d.selectOTP(ctx); err != nil {
		return token2.Config{}, err
	}

	return d.readConfig(ctx)
}

func (d *Device) selectOTP(ctx context.Context) error {
	response, err := apdu.Exchange(ctx, d.card, protocol.SelectOTPCommand())
	if err != nil {
		return err
	}
	if err := response.Err("select Token2 OTP application"); err != nil {
		return err
	}

	return nil
}

func (d *Device) readConfig(ctx context.Context) (token2.Config, error) {
	response, err := apdu.Exchange(ctx, d.card, protocol.ConfigCommand())
	if err != nil {
		return token2.Config{}, err
	}
	if err := response.Err("read Token2 configuration"); err != nil {
		return token2.Config{}, err
	}

	return token2.ParseConfig(response.Data)
}

func (d *Device) prepareLegacySerialNumber(ctx context.Context) error {
	response, err := apdu.Exchange(ctx, d.card, protocol.LegacySerialNumberPreludeCommand())
	if err != nil {
		return err
	}

	return response.Err("prepare legacy serial-number command")
}

func (d *Device) readSerialNumber(ctx context.Context) (apdu.Response, error) {
	return apdu.Exchange(ctx, d.card, protocol.SerialNumberCommand(false))
}

// SerialNumber returns the full device serial number.
func (d *Device) SerialNumber(ctx context.Context) (serialNumber string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.card.BeginTransaction(ctx); err != nil {
		return "", err
	}
	defer func() {
		err = errors.Join(err, d.card.EndTransaction(nativepcsc.DispositionLeaveCard))
	}()

	if err := d.selectOTP(ctx); err != nil {
		return "", err
	}

	response, err := d.readSerialNumber(ctx)
	if err != nil {
		return "", err
	}
	if response.SW == statusInstructionNotSupported {
		if err := d.prepareLegacySerialNumber(ctx); err != nil {
			return "", err
		}

		response, err = d.readSerialNumber(ctx)
		if err != nil {
			return "", err
		}
	}
	if err := response.Err("read serial number"); err != nil {
		return "", err
	}

	return token2.ParseSerialNumber(response.Data)
}
