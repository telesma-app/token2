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

// Open connects to the Token2 device in reader and returns a caller-owned device.
// Options are passed to the native PC/SC connection; Close leaves the card
// powered by default. Use nativepcsc.WithDisconnectDisposition to request a reset.
func Open(reader string, opts ...nativepcsc.OpenOption) (*Device, error) {
	card, err := nativepcsc.Open(reader, opts...)
	if err != nil {
		return nil, err
	}

	return &Device{card: card}, nil
}

// Close closes the PC/SC connection, canceling an active card operation when
// supported by the native implementation.
func (d *Device) Close() error {
	return d.card.Close()
}

// ATR returns information encoded in the device answer-to-reset.
func (d *Device) ATR(ctx context.Context) (token2.ATR, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return token2.ATR{}, err
	}

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

	return ReadConfiguration(ctx, d.card)
}

// ReadConfiguration selects Token2 OTP on a borrowed card and leaves it selected.
// The caller serializes access and manages the transaction.
func ReadConfiguration(ctx context.Context, card iso7816.Card) (token2.Configuration, error) {
	if err := selectOTP(ctx, card); err != nil {
		return token2.Configuration{}, err
	}

	response, err := exchange(ctx, card, protocol.ConfigurationCommand())
	if err != nil {
		return token2.Configuration{}, err
	}

	if err := responseError("read Token2 configuration", response); err != nil {
		return token2.Configuration{}, err
	}

	return token2.ParseConfiguration(response.Data)
}

func selectOTP(ctx context.Context, card iso7816.Card) error {
	response, err := exchange(ctx, card, protocol.SelectOTPCommand())
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

func selectFIDO(ctx context.Context, card iso7816.Card) error {
	response, err := exchange(ctx, card, protocol.SelectFIDOCommand())
	if err != nil {
		return err
	}

	return responseError("select FIDO application", response)
}

func readSerialNumber(ctx context.Context, card iso7816.Card) (iso7816.Response, error) {
	return exchange(
		ctx,
		card,
		protocol.SerialNumberCommand(iso7816.EncodingShort),
	)
}

// SerialNumber reads the full serial number reported by the device.
func (d *Device) SerialNumber(
	ctx context.Context,
) (serial string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.card.BeginTransaction(ctx); err != nil {
		return "", err
	}
	defer func() {
		err = errors.Join(err, d.card.EndTransaction(nativepcsc.DispositionLeaveCard))
	}()

	return ReadSerialNumber(ctx, d.card)
}

// ReadSerialNumber reads the full serial on a borrowed card, changing its applet.
// The caller serializes access and manages the transaction; no applet is restored.
func ReadSerialNumber(ctx context.Context, card iso7816.Card) (string, error) {
	if err := selectOTP(ctx, card); err != nil {
		return "", err
	}

	response, err := readSerialNumber(ctx, card)
	if err != nil {
		return "", err
	}

	if response.Status == statusInstructionNotSupported {
		prelude, err := exchange(ctx, card, protocol.LegacySerialNumberPreludeCommand())
		if err != nil {
			return "", err
		}

		if prelude.Status == iso7816.StatusSuccess {
			response, err = readSerialNumber(ctx, card)
			if err != nil {
				return "", err
			}
		}

		if response.Status == statusInstructionNotSupported {
			if err := selectFIDO(ctx, card); err != nil {
				return "", errors.Join(responseError("prepare legacy serial-number command", prelude), err)
			}

			response, err = readSerialNumber(ctx, card)
			if err != nil {
				return "", err
			}
		}
	}

	if err := responseError("read serial number", response); err != nil {
		return "", err
	}

	return token2.ParseSerialNumber(response.Data)
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
