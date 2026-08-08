package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	githubpcsc "github.com/telesma-app/pcsc"
	"github.com/telesma-app/token2"
	token2pcsc "github.com/telesma-app/token2/transport/pcsc"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) (err error) {
	reader, err := findReader(os.Getenv("PCSC_READER"))
	if err != nil {
		return err
	}

	device, err := token2pcsc.Open(reader)
	if err != nil {
		return fmt.Errorf("open PC/SC reader %q: %w", reader, err)
	}
	defer func() {
		err = errors.Join(err, device.Close())
	}()

	fmt.Printf("PC/SC reader: %s\n", reader)

	info, err := device.DeviceInfo(ctx)
	if err != nil {
		fmt.Printf("Device information: unsupported or failed: %v\n", err)
	} else {
		printDeviceInfo(info)
	}

	atr, err := device.ATR(ctx)
	if err != nil {
		fmt.Printf("ATR identity: unsupported or failed: %v\n", err)
	} else {
		printATR(atr)
	}

	config, err := device.Configuration(ctx)
	if err != nil {
		fmt.Printf("Configuration: unsupported or failed: %v\n", err)
	} else {
		printConfiguration(config)
	}

	return nil
}

func findReader(filter string) (string, error) {
	for reader, err := range githubpcsc.Enumerate() {
		if err != nil {
			return "", fmt.Errorf("enumerate PC/SC readers: %w", err)
		}
		if filter == "" || strings.Contains(reader.Name, filter) {
			return reader.Name, nil
		}
	}

	if filter == "" {
		return "", errors.New("no PC/SC readers found")
	}
	return "", fmt.Errorf("no PC/SC reader matching %q", filter)
}

func printATR(info token2.ATR) {
	fmt.Printf("ATR: %x\n", info.Raw)
	fmt.Printf("Product ID: %04x\n", info.ProductID)
	fmt.Printf("Serial suffix: %s\n", info.SerialSuffix)
}

func printDeviceInfo(info token2.DeviceInfo) {
	fmt.Printf("Serial number: %s\n", info.SerialNumber)
	if name := info.ModelName(""); name != "" {
		fmt.Printf("Model: %s\n", name)
	}
}

func printConfiguration(config token2.Configuration) {
	if len(config.Raw) == 1 {
		fmt.Printf("Configuration: legacy transfer type=%02x\n", config.TransferType)
		return
	}

	fmt.Printf("Appearance: %x\n", config.Appearance)
	fmt.Printf(
		"FIDO version: %d.%d.%d\n",
		config.FIDOVersion.Major,
		config.FIDOVersion.Minor,
		config.FIDOVersion.Patch,
	)
	fmt.Printf(
		"Capability masks: device=%02x extension=%02x\n",
		config.DeviceConfiguration,
		config.DeviceExtension,
	)
}
