package pcsc

import (
	"context"
	"errors"

	"github.com/telesma-app/iso7816"
	nativepcsc "github.com/telesma-app/pcsc"
	"github.com/telesma-app/token2"
)

// InformationCard supplies the APDU and status operations used by vendor reads.
type InformationCard interface {
	iso7816.Card
	Status() (*nativepcsc.CardStatus, error)
}

// ReadDeviceInfo reads the exact borrowed card, preserving independent results.
// The caller serializes access, manages the transaction and subsequent applet
// selection, and retains ownership of the card.
func ReadDeviceInfo(ctx context.Context, card InformationCard) (info token2.DeviceInfo, err error) {
	serial, readErr := ReadSerialNumber(ctx, card)
	info.SerialNumber.Err = readErr
	if readErr == nil {
		info.SerialNumber.Value = &serial
		if model, known := token2.Identify(serial); known {
			info.Model = &model
		}
	}
	err = readErr
	if ctx.Err() != nil {
		return info, errors.Join(err, ctx.Err())
	}

	status, readErr := card.Status()
	if readErr == nil {
		atr, parseErr := token2.ParseATR(status.ATR)
		readErr = parseErr
		if parseErr == nil {
			info.ATR.Value = &atr
		}
	}
	info.ATR.Err = readErr
	err = errors.Join(err, readErr)
	if ctx.Err() != nil {
		return info, errors.Join(err, ctx.Err())
	}

	configuration, readErr := ReadConfiguration(ctx, card)
	info.Configuration.Err = readErr
	if readErr == nil {
		info.Configuration.Value = &configuration
	}
	return info, errors.Join(err, readErr)
}

// DeviceInfo reads vendor information in one managed PC/SC transaction.
func (d *Device) DeviceInfo(ctx context.Context) (info token2.DeviceInfo, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.card.BeginTransaction(ctx); err != nil {
		return info, err
	}
	defer func() { err = errors.Join(err, d.card.EndTransaction(nativepcsc.DispositionLeaveCard)) }()
	return ReadDeviceInfo(ctx, d.card)
}
