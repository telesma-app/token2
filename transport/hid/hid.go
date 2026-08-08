// Package hid accesses Token2 devices through USB HID feature reports.
package hid

import (
	"context"
	"fmt"
	"sync"

	hidapi "github.com/telesma-app/hid"
	"github.com/telesma-app/iso7816"
	"github.com/telesma-app/token2"
	"github.com/telesma-app/token2/internal/protocol"
)

const (
	reportSize  = 65
	chunkSize   = 61
	reportMagic = 0x21

	reportMore    = 0x20
	reportPending = 0xc0
)

type featureDevice interface {
	SendFeatureReport([]byte) error
	GetFeatureReport([]byte) (int, error)
	Close() error
}

// Device is a Token2 device connected through USB HID.
type Device struct {
	mu     sync.Mutex
	device featureDevice
}

// Open opens the HID device at path.
func Open(path string) (*Device, error) {
	device, err := hidapi.OpenPath(path)
	if err != nil {
		return nil, err
	}

	return &Device{device: device}, nil
}

// Close closes the underlying HID device.
func (d *Device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.device.Close()
}

// DeviceInfo returns transport-neutral information derived from the device
// serial number.
func (d *Device) DeviceInfo(ctx context.Context) (token2.DeviceInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	response, err := iso7816.Exchange(
		ctx,
		transceiver{device: d.device},
		protocol.SerialNumberCommand(iso7816.EncodingExtended),
		iso7816.WithMoreDataStatusBytes(0x61, 0x9f),
	)
	if err != nil {
		return token2.DeviceInfo{}, err
	}
	if err := response.APDUError(); err != nil {
		return token2.DeviceInfo{}, fmt.Errorf("read serial number: %w", err)
	}

	serialNumber, err := token2.ParseSerialNumber(response.Data)
	if err != nil {
		return token2.DeviceInfo{}, err
	}

	info, _ := token2.Identify(serialNumber)
	return info, nil
}

type transceiver struct {
	device featureDevice
}

var _ iso7816.Card = transceiver{}

func (t transceiver) Transmit(_ context.Context, command []byte) ([]byte, error) {
	for offset, sequence := 0, byte(0); offset < len(command); sequence++ {
		length := min(chunkSize, len(command)-offset)
		report := make([]byte, reportSize)

		report[1] = reportMagic
		report[2] = sequence & 0x0f
		if offset+length < len(command) {
			report[2] |= reportMore
		}
		report[3] = byte(length)
		copy(report[4:], command[offset:offset+length])

		if err := t.device.SendFeatureReport(report); err != nil {
			return nil, err
		}

		offset += length
	}

	var response []byte
	for sequence := byte(0); ; {
		report := make([]byte, reportSize)
		n, err := t.device.GetFeatureReport(report)
		if err != nil {
			return nil, err
		}
		if n < 4 {
			return nil, fmt.Errorf("Token2 HID report is %d bytes; need at least 4", n)
		}
		if n > len(report) {
			return nil, fmt.Errorf("Token2 HID report length %d exceeds buffer size %d", n, len(report))
		}
		if report[1] != reportMagic {
			return nil, fmt.Errorf("unexpected Token2 HID report magic: %02x", report[1])
		}

		flags := report[2] & 0xf0
		if flags == reportPending {
			continue
		}
		if got := report[2] & 0x0f; got != sequence&0x0f {
			return nil, fmt.Errorf("unexpected Token2 HID report sequence: got %d, want %d", got, sequence&0x0f)
		}

		length := int(report[3])
		if length > chunkSize {
			return nil, fmt.Errorf("Token2 HID report payload length %d exceeds %d", length, chunkSize)
		}
		if 4+length > n {
			return nil, fmt.Errorf("Token2 HID report payload length %d exceeds received report size %d", length, n)
		}

		response = append(response, report[4:4+length]...)
		sequence++

		if flags&reportMore == 0 {
			return response, nil
		}
	}
}
