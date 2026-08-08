// Package pcsc provides access to Token2 devices over PC/SC.
package pcsc

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/telesma-app/iso7816"
	nativepcsc "github.com/telesma-app/pcsc"
	"github.com/telesma-app/token2"
	"github.com/telesma-app/token2/internal/protocol"
)

const statusInstructionNotSupported iso7816.StatusWord = 0x6d00

// ErrOTPApplicationNotAvailable reports that the selected card does not expose
// the Token2 OTP application used for device information.
var ErrOTPApplicationNotAvailable = errors.New(
	"token2 pcsc: OTP application not available",
)

type card interface {
	iso7816.Card
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

// ATR returns information encoded in the device answer-to-reset.
func (d *Device) ATR(_ context.Context) (token2.ATR, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	status, err := d.card.Status()
	if err != nil {
		return token2.ATR{}, err
	}

	return token2.ParseATR(status.ATR)
}

// Configuration returns the Token2 device configuration.
func (d *Device) Configuration(
	ctx context.Context,
) (config token2.Configuration, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.card.BeginTransaction(ctx); err != nil {
		return token2.Configuration{}, err
	}
	defer func() {
		err = errors.Join(err, d.card.EndTransaction(nativepcsc.DispositionLeaveCard))
	}()

	return d.configuration(ctx)
}

func (d *Device) configuration(ctx context.Context) (token2.Configuration, error) {
	if err := d.selectOTP(ctx); err != nil {
		return token2.Configuration{}, err
	}

	return d.readConfiguration(ctx)
}

func (d *Device) selectOTP(ctx context.Context) error {
	response, err := exchange(ctx, d.card, protocol.SelectOTPCommand())
	if err != nil {
		return err
	}
	if response.Status == 0x6a82 {
		return fmt.Errorf(
			"select Token2 OTP application: %w: %w",
			ErrOTPApplicationNotAvailable,
			response.APDUError(),
		)
	}

	return responseError("select Token2 OTP application", response)
}

func (d *Device) selectFIDO(ctx context.Context) error {
	response, err := exchange(ctx, d.card, protocol.SelectFIDOCommand())
	if err != nil {
		return err
	}
	return responseError("select FIDO application", response)
}

func (d *Device) readConfiguration(ctx context.Context) (token2.Configuration, error) {
	response, err := exchange(ctx, d.card, protocol.ConfigurationCommand())
	if err != nil {
		return token2.Configuration{}, err
	}
	if err := responseError("read Token2 configuration", response); err != nil {
		return token2.Configuration{}, err
	}

	return token2.ParseConfiguration(response.Data)
}

func (d *Device) readSerialNumber(ctx context.Context) (iso7816.Response, error) {
	return exchange(
		ctx,
		d.card,
		protocol.SerialNumberCommand(iso7816.EncodingShort),
	)
}

func (d *Device) prepareLegacySerialNumber(ctx context.Context) error {
	response, err := exchange(
		ctx,
		d.card,
		protocol.LegacySerialNumberPreludeCommand(),
	)
	if err != nil {
		return err
	}

	return responseError("prepare legacy serial-number command", response)
}

// DeviceInfo returns transport-neutral information derived from the device
// serial number.
func (d *Device) DeviceInfo(
	ctx context.Context,
) (info token2.DeviceInfo, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.card.BeginTransaction(ctx); err != nil {
		return token2.DeviceInfo{}, err
	}
	defer func() {
		err = errors.Join(err, d.card.EndTransaction(nativepcsc.DispositionLeaveCard))
	}()

	if err := d.selectOTP(ctx); err != nil {
		return token2.DeviceInfo{}, err
	}

	response, err := d.readSerialNumber(ctx)
	if err != nil {
		return token2.DeviceInfo{}, err
	}
	if response.Status == statusInstructionNotSupported {
		legacyErr := d.prepareLegacySerialNumber(ctx)
		if legacyErr == nil {
			response, err = d.readSerialNumber(ctx)
			if err != nil {
				return token2.DeviceInfo{}, err
			}
		} else {
			var statusErr *iso7816.APDUError
			if !errors.As(legacyErr, &statusErr) {
				return token2.DeviceInfo{}, legacyErr
			}
		}

		if response.Status == statusInstructionNotSupported {
			if err := d.selectFIDO(ctx); err != nil {
				return token2.DeviceInfo{}, errors.Join(legacyErr, err)
			}

			response, err = d.readSerialNumber(ctx)
			if err != nil {
				return token2.DeviceInfo{}, err
			}
		}
	}
	if err := responseError("read serial number", response); err != nil {
		return token2.DeviceInfo{}, err
	}

	serialNumber, err := token2.ParseSerialNumber(response.Data)
	if err != nil {
		return token2.DeviceInfo{}, err
	}

	info, _ = token2.Identify(serialNumber)
	return info, nil
}

func exchange(
	ctx context.Context,
	card iso7816.Card,
	command iso7816.Command,
) (iso7816.Response, error) {
	return iso7816.Exchange(
		ctx,
		card,
		command,
		iso7816.WithMoreDataStatusBytes(0x61, 0x9f),
	)
}

func responseError(operation string, response iso7816.Response) error {
	if err := response.APDUError(); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}

	return nil
}
