package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/telesma-app/iso7816"
	"github.com/telesma-app/pcsc"
	"github.com/telesma-app/token2/internal/protocol"
	token2pcsc "github.com/telesma-app/token2/transport/pcsc"
)

type tracedCard struct {
	*pcsc.Card
	r *recorder
}

func (c tracedCard) Transmit(ctx context.Context, command []byte) ([]byte, error) {
	c.r.emit("pcsc.send", map[string]any{"requestHex": fmt.Sprintf("%x", command)})
	start := time.Now()
	raw, err := c.Card.Transmit(ctx, command)
	c.r.emit("pcsc.receive", map[string]any{"responseHex": fmt.Sprintf("%x", raw), "error": errorText(err), "elapsedMs": time.Since(start).Milliseconds()})
	return raw, err
}

func probePCSC(ctx context.Context, reader string, r *recorder) (err error) {
	card, err := pcsc.Open(reader)
	r.emit("pcsc.open", map[string]any{"error": errorText(err)})
	if err != nil {
		return err
	}
	defer func() {
		closeErr := card.Close()
		r.emit("pcsc.close", map[string]any{"error": errorText(closeErr)})
		err = errors.Join(err, closeErr)
	}()
	beginCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = card.BeginTransaction(beginCtx)
	cancel()
	r.emit("pcsc.begin", map[string]any{"error": errorText(err)})
	if err != nil {
		return err
	}
	defer func() {
		endErr := card.EndTransaction(pcsc.DispositionLeaveCard)
		r.emit("pcsc.end", map[string]any{"error": errorText(endErr)})
		err = errors.Join(err, endErr)
	}()
	status, statusErr := card.Status()
	r.emit("pcsc.status", map[string]any{"value": status, "error": errorText(statusErr)})
	if statusErr == nil {
		r.decoded("pcsc.atr", "atr", status.ATR)
	}
	traced := tracedCard{Card: card, r: r}
	probeAppletCommands(ctx, traced, r)
	// Measure the current library independently after the explicit protocol
	// probes. This is a warm read, not evidence of power-on behaviour.
	infoCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	info, infoErr := token2pcsc.ReadDeviceInfo(infoCtx, traced)
	cancel()
	r.emit("pcsc.library.warm", map[string]any{
		"value": info, "error": errorText(infoErr),
		"readErrors": map[string]string{
			"serialNumber":  errorText(info.SerialNumber.Err),
			"atr":           errorText(info.ATR.Err),
			"configuration": errorText(info.Configuration.Err),
		},
	})
	return nil
}

func probeAppletCommands(ctx context.Context, card iso7816.Card, r *recorder) {
	if exchange(ctx, card, r, "otp.select", "", protocol.SelectOTPCommand()) {
		exchange(ctx, card, r, "otp.serial.initial", "serial", protocol.SerialNumberCommand(iso7816.EncodingShort))
		exchange(ctx, card, r, "otp.configuration.initial", "configuration", protocol.ConfigurationCommand())
		exchange(ctx, card, r, "otp.legacy-get-info", "ctap", protocol.LegacySerialNumberPreludeCommand())
		exchange(ctx, card, r, "otp.serial.after-prelude", "serial", protocol.SerialNumberCommand(iso7816.EncodingShort))
		exchange(ctx, card, r, "otp.configuration.after-prelude", "configuration", protocol.ConfigurationCommand())
	}
	if exchange(ctx, card, r, "fido.select", "", protocol.SelectFIDOCommand()) {
		exchange(ctx, card, r, "fido.serial", "serial", protocol.SerialNumberCommand(iso7816.EncodingShort))
		// Synchronous NFCCTAP_MSG: GetInfo cannot require user interaction.
		exchange(ctx, card, r, "fido.get-info", "ctap", iso7816.Command{
			CLA: 0x80, INS: 0x10, Data: []byte{0x04}, Le: 256, Encoding: iso7816.EncodingShort,
		})
	}
}

func exchange(ctx context.Context, card iso7816.Card, r *recorder, operation, kind string, command iso7816.Command) bool {
	if ctx.Err() != nil {
		return false
	}
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	r.emit(operation+".start", nil)
	response, err := iso7816.Exchange(callCtx, card, command, iso7816.WithMoreDataStatusBytes(0x61, 0x9f))
	fields := map[string]any{"transportError": errorText(err)}
	if err == nil {
		fields["status"], fields["rawHex"] = response.Status.String(), fmt.Sprintf("%x", response.Data)
		fields["statusError"] = errorText(response.APDUError())
		fmt.Fprintf(os.Stderr, "    %s: SW=%s, %d bytes\n", operation, response.Status, len(response.Data))
	} else {
		fmt.Fprintf(os.Stderr, "    %s: %v\n", operation, err)
	}
	r.emit(operation, fields)
	if err != nil || response.Status != iso7816.StatusSuccess {
		return false
	}
	if kind == "ctap" {
		if len(response.Data) == 0 {
			r.emit(operation+".ctap", map[string]any{"error": "missing CTAP status byte"})
		} else {
			r.emit(operation+".ctap", map[string]any{"status": response.Data[0]})
			if response.Data[0] == 0 {
				r.decoded(operation, "get-info", response.Data[1:])
			}
		}
	} else if kind != "" {
		r.decoded(operation, kind, response.Data)
	}
	return true
}
