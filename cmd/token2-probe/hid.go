package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/telesma-app/ctap/transport/ctaphid"
	"github.com/telesma-app/hid"
	"github.com/telesma-app/iso7816"
	"github.com/telesma-app/token2/internal/protocol"
	token2ctaphid "github.com/telesma-app/token2/transport/ctaphid"
)

type featureIO interface {
	SendFeatureReport([]byte) error
	GetFeatureReport([]byte) (int, error)
}

// The library hides its feature-report transceiver. This diagnostic adapter
// records framing before validation, so malformed firmware replies are retained.
type featureCard struct {
	device featureIO
	r      *recorder
}

func (c featureCard) Transmit(ctx context.Context, command []byte) ([]byte, error) {
	for offset, sequence := 0, byte(0); offset < len(command); sequence++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		n := min(61, len(command)-offset)
		report := make([]byte, 65)

		report[1], report[2], report[3] = 0x21, sequence&15, byte(n)
		if offset+n < len(command) {
			report[2] |= 0x20
		}

		copy(report[4:], command[offset:offset+n])
		c.r.emit("feature.send", map[string]any{"requestHex": fmt.Sprintf("%x", report)})

		if err := c.device.SendFeatureReport(report); err != nil {
			c.r.emit("feature.send-error", map[string]any{"error": err.Error()})
			return nil, err
		}

		offset += n
	}

	var raw []byte

	for sequence := byte(0); ; {
		if err := ctx.Err(); err != nil {
			return raw, err
		}

		report := make([]byte, 65)
		n, err := c.device.GetFeatureReport(report)
		c.r.emit("feature.receive", map[string]any{"bufferHex": fmt.Sprintf("%x", report), "returnedLength": n, "error": errorText(err)})

		if err != nil {
			return raw, err
		}

		if n < 4 || n > len(report) || report[1] != 0x21 {
			return raw, fmt.Errorf("invalid feature header: length=%d magic=%02x", n, report[1])
		}

		flags := report[2] & 0xf0
		if flags == 0xc0 {
			continue
		}

		if report[2]&15 != sequence&15 || report[3] > 61 || int(report[3])+4 > n {
			return raw, fmt.Errorf("invalid feature fragment: sequence=%d expected=%d payload=%d length=%d", report[2]&15, sequence&15, report[3], n)
		}

		raw = append(raw, report[4:4+int(report[3])]...)
		sequence++

		if flags&0x20 == 0 {
			return raw, nil
		}
	}
}

func probeFeature(ctx context.Context, path string, r *recorder) (err error) {
	device, err := hid.OpenPath(path)
	r.emit("feature.open", map[string]any{"error": errorText(err)})

	if err != nil {
		return err
	}

	defer func() {
		closeErr := device.Close()
		r.emit("feature.close", map[string]any{"error": errorText(closeErr)})
		err = errors.Join(err, closeErr)
	}()

	exchange(ctx, featureCard{device: device, r: r}, r, "feature.serial", "serial", protocol.SerialNumberCommand(iso7816.EncodingExtended))

	return nil
}

type tracedHID struct {
	*hid.Device
	r *recorder
}

func (d tracedHID) Read(ctx context.Context, p []byte) (int, error) {
	n, err := d.Device.Read(ctx, p)
	d.r.emit("ctaphid.receive", map[string]any{"responseHex": fmt.Sprintf("%x", p[:n]), "error": errorText(err)})
	// INIT fits in one input report. Retain its observed version bytes even
	// when the transport subsequently rejects a nonce or channel mismatch.
	if n >= 24 && p[4] == 0x86 && p[5] == 0 && p[6] >= 17 {
		d.r.emit("ctaphid.init-observed", map[string]any{"protocol": p[19], "major": p[20], "minor": p[21], "build": p[22], "capabilities": p[23]})
	}

	return n, err
}

func (d tracedHID) Write(ctx context.Context, p []byte) (int, error) {
	d.r.emit("ctaphid.send", map[string]any{"requestHex": fmt.Sprintf("%x", p)})

	n, err := d.Device.Write(ctx, p)
	if err != nil {
		d.r.emit("ctaphid.send-error", map[string]any{"written": n, "error": err.Error()})
	}

	return n, err
}

func probeCTAPHID(ctx context.Context, path string, r *recorder) (err error) {
	device, err := hid.OpenPath(path)
	r.emit("ctaphid.open", map[string]any{"error": errorText(err)})

	if err != nil {
		return err
	}

	traced := tracedHID{Device: device, r: r}
	initCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	transport, err := ctaphid.Open(initCtx, traced)

	cancel()
	r.emit("ctaphid.init", map[string]any{"error": errorText(err)})

	if err != nil {
		return errors.Join(err, device.Close())
	}

	defer func() {
		closeErr := transport.Close()
		r.emit("ctaphid.close", map[string]any{"error": errorText(closeErr)})
		err = errors.Join(err, closeErr)
	}()

	atrCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	atr, atrErr := transport.Vendor(atrCtx, token2ctaphid.CommandGetATR, nil)

	cancel()
	r.emit("ctaphid.atr", map[string]any{"rawHex": fmt.Sprintf("%x", atr.Data), "error": errorText(atrErr)})
	fmt.Fprintf(os.Stderr, "    ctaphid.atr: %d bytes, error=%v\n", len(atr.Data), atrErr)

	if atrErr == nil {
		r.decoded("ctaphid.atr", "atr", atr.Data)
	}

	infoCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	info, infoErr := transport.CBOR(infoCtx, []byte{0x04})

	cancel()
	r.emit("ctaphid.get-info", map[string]any{"rawHex": fmt.Sprintf("%x", info.Data), "error": errorText(infoErr)})
	fmt.Fprintf(os.Stderr, "    ctaphid.get-info: %d bytes, error=%v\n", len(info.Data), infoErr)

	if infoErr == nil {
		r.decoded("ctaphid.get-info", "get-info", info.Data)
	}

	return nil
}
