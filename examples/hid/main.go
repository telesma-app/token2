package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	githubhid "github.com/telesma-app/hid"
	token2hid "github.com/telesma-app/token2/transport/hid"
)

const token2VendorID = 0x349e

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) (err error) {
	device, path, serial, err := openDevice(ctx, os.Getenv("TOKEN2_HID_PATH"))
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, device.Close())
	}()

	fmt.Printf("HID path: %s\n", path)
	fmt.Printf("Serial number: %s\n", serial)

	return nil
}

func openDevice(
	ctx context.Context,
	path string,
) (*token2hid.Device, string, string, error) {
	if path != "" {
		device, err := token2hid.Open(path)
		if err != nil {
			return nil, "", "", fmt.Errorf("open HID path %q: %w", path, err)
		}

		info, err := device.SerialNumber(ctx)
		if err != nil {
			return nil, "", "", errors.Join(
				fmt.Errorf("read device information from HID path %q: %w", path, err),
				device.Close(),
			)
		}

		return device, path, info, nil
	}

	for info, err := range githubhid.Enumerate(githubhid.WithVendorID(token2VendorID)) {
		if err != nil {
			return nil, "", "", fmt.Errorf("enumerate HID devices: %w", err)
		}

		if err := ctx.Err(); err != nil {
			return nil, "", "", err
		}

		device, err := token2hid.Open(info.Path)
		if err != nil {
			continue
		}

		serial, infoErr := device.SerialNumber(ctx)
		if infoErr != nil {
			_ = device.Close()
			continue
		}

		return device, info.Path, serial, nil
	}

	return nil, "", "", errors.New("no Token2 feature HID interface found")
}
