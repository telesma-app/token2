package hid

import (
	"context"
	"errors"

	"github.com/telesma-app/token2"
	token2ctaphid "github.com/telesma-app/token2/transport/ctaphid"
)

// ReadDeviceInfo opens and closes the HID feature interface at path.
// vendor may be nil when no CTAPHID channel is available. When supplied,
// it is borrowed for the ATR read and remains owned by the caller.
func ReadDeviceInfo(ctx context.Context, path string, vendor token2ctaphid.VendorTransport) (info token2.DeviceInfo, err error) {
	device, openErr := Open(path)
	if openErr != nil {
		info.SerialNumber.Err = openErr
		err = openErr
	} else {
		serial, readErr := device.SerialNumber(ctx)
		info.SerialNumber.Err = readErr
		if readErr == nil {
			info.SerialNumber.Value = &serial
			if model, known := token2.Identify(serial); known {
				info.Model = &model
			}
		}
		err = errors.Join(readErr, device.Close())
	}
	if vendor != nil && ctx.Err() == nil {
		atr, readErr := token2ctaphid.ReadATR(ctx, vendor)
		info.ATR.Err = readErr
		if readErr == nil {
			info.ATR.Value = &atr
		}
		err = errors.Join(err, readErr)
	}
	return info, errors.Join(err, ctx.Err())
}
