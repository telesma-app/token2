package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	githubhid "github.com/telesma-app/hid"
	token2ctaphid "github.com/telesma-app/token2/transport/ctaphid"
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
	path, err := findPath(ctx, os.Getenv("TOKEN2_CTAPHID_PATH"))
	if err != nil {
		return err
	}

	device, err := token2ctaphid.Open(ctx, path)
	if err != nil {
		return fmt.Errorf("open CTAPHID path %q: %w", path, err)
	}
	defer func() {
		err = errors.Join(err, device.Close())
	}()

	info, err := device.ATR(ctx)
	if err != nil {
		return fmt.Errorf("read ATR: %w", err)
	}

	fmt.Printf("CTAPHID path: %s\n", path)
	fmt.Printf("ATR: %x\n", info.Raw)
	fmt.Printf("Product ID: %04x\n", info.ProductID)
	fmt.Printf("Serial suffix: %s\n", info.SerialSuffix)

	return nil
}

func findPath(ctx context.Context, path string) (string, error) {
	if path != "" {
		return path, nil
	}

	for info, err := range githubhid.Enumerate(
		githubhid.WithVendorID(token2VendorID),
		githubhid.WithUsagePage(fidoUsagePage),
		githubhid.WithUsage(fidoUsage),
	) {
		if err != nil {
			return "", fmt.Errorf("enumerate HID devices: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		return info.Path, nil
	}

	return "", errors.New("no Token2 CTAPHID interface found")
}
