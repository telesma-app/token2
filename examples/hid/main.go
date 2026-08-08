package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	githubhid "github.com/telesma-app/hid"
	"github.com/telesma-app/token2"
	token2hid "github.com/telesma-app/token2/transport/hid"
)

const (
	token2VendorID = 0x349e
	fidoUsagePage  = 0xf1d0
	fidoUsage      = 0x01
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) (err error) {
	device, path, info, err := openDevice(ctx, os.Getenv("TOKEN2_HID_PATH"))
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, device.Close())
	}()

	fmt.Printf("HID path: %s\n", path)
	fmt.Printf("Serial number: %s\n", info.SerialNumber)
	if name := info.ModelName(""); name != "" {
		fmt.Printf("Model: %s\n", name)
	}

	return nil
}

func openDevice(
	ctx context.Context,
	path string,
) (*token2hid.Device, string, token2.DeviceInfo, error) {
	if path != "" {
		device, err := token2hid.Open(path)
		if err != nil {
			return nil, "", token2.DeviceInfo{}, fmt.Errorf("open HID path %q: %w", path, err)
		}

		info, err := device.DeviceInfo(ctx)
		if err != nil {
			return nil, "", token2.DeviceInfo{}, errors.Join(
				fmt.Errorf("read device information from HID path %q: %w", path, err),
				device.Close(),
			)
		}
		return device, path, info, nil
	}

	for info, err := range githubhid.Enumerate(githubhid.WithVendorID(token2VendorID)) {
		if err != nil {
			return nil, "", token2.DeviceInfo{}, fmt.Errorf("enumerate HID devices: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return nil, "", token2.DeviceInfo{}, err
		}
		if info.UsagePage == fidoUsagePage && info.Usage == fidoUsage {
			continue
		}

		device, err := token2hid.Open(info.Path)
		if err != nil {
			continue
		}
		deviceInfo, infoErr := device.DeviceInfo(ctx)
		if infoErr != nil {
			_ = device.Close()
			continue
		}

		return device, info.Path, deviceInfo, nil
	}

	return nil, "", token2.DeviceInfo{}, errors.New("no Token2 feature HID interface found")
}
