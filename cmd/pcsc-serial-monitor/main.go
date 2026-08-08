package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/telesma-app/iso7816"
	nativepcsc "github.com/telesma-app/pcsc"
	"github.com/telesma-app/token2"
	"github.com/telesma-app/token2/internal/protocol"
	token2pcsc "github.com/telesma-app/token2/transport/pcsc"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string, output io.Writer) (err error) {
	flags := flag.NewFlagSet("pcsc-serial-monitor", flag.ContinueOnError)
	flags.SetOutput(output)

	readerFilter := flags.String("reader", "token2", "case-insensitive PC/SC reader-name substring; empty matches every reader")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	receiver, err := nativepcsc.Watch()
	if err != nil {
		return fmt.Errorf("listen for PC/SC events: %w", err)
	}
	defer func() {
		err = errors.Join(err, receiver.Close())
	}()

	results := monitorResults{serials: make(map[string]token2.DeviceInfo)}
	defer results.print(output)

	fmt.Fprintf(output, "Waiting for cards in PC/SC readers matching %q. Press Ctrl-C to finish.\n", *readerFilter)
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-receiver.Listen():
			if !ok {
				return nil
			}
			if !containsFold(event.ReaderInfo.Name, *readerFilter) {
				continue
			}

			switch event.Type {
			case nativepcsc.DeviceEventCardInserted:
				results.inspect(ctx, output, event.ReaderInfo)
			case nativepcsc.DeviceEventCardRemoved:
				fmt.Fprintf(output, "REMOVED reader=%q\n", event.ReaderInfo.Name)
			}
		}
	}
}

func containsFold(value, substring string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(substring))
}

type monitorResults struct {
	attempts int
	failures int
	serials  map[string]token2.DeviceInfo
}

func (results *monitorResults) inspect(
	ctx context.Context,
	output io.Writer,
	reader *nativepcsc.ReaderInfo,
) {
	results.attempts++
	fmt.Fprintf(output, "INSERTED reader=%q atr=%x\n", reader.Name, reader.ATR)

	info, err := readDeviceInfo(ctx, reader.Name)
	if err != nil {
		results.failures++
		fmt.Fprintf(output, "FAIL reader=%q error=%v\n", reader.Name, err)
		traceDevice(ctx, output, reader.Name)
		return
	}

	results.serials[info.SerialNumber] = info
	fmt.Fprintf(
		output,
		"OK serial=%s release=%s model=%q\n",
		info.SerialNumber,
		info.Release,
		info.ModelName("unknown"),
	)
}

func (results *monitorResults) print(output io.Writer) {
	fmt.Fprintf(
		output,
		"SUMMARY attempts=%d unique_serials=%d failures=%d\n",
		results.attempts,
		len(results.serials),
		results.failures,
	)
}

func traceDevice(ctx context.Context, output io.Writer, reader string) {
	card, err := nativepcsc.Open(reader)
	if err != nil {
		fmt.Fprintf(output, "TRACE open error=%v\n", err)
		return
	}
	defer card.Close()

	if err := card.BeginTransaction(ctx); err != nil {
		fmt.Fprintf(output, "TRACE begin-transaction error=%v\n", err)
		return
	}
	defer card.EndTransaction(nativepcsc.DispositionLeaveCard)

	commands := []struct {
		name    string
		command iso7816.Command
	}{
		{name: "select-otp", command: protocol.SelectOTPCommand()},
		{name: "serial-before-prelude", command: protocol.SerialNumberCommand(iso7816.EncodingShort)},
		{name: "legacy-prelude", command: protocol.LegacySerialNumberPreludeCommand()},
		{name: "serial-after-prelude", command: protocol.SerialNumberCommand(iso7816.EncodingShort)},
		{name: "select-fido", command: protocol.SelectFIDOCommand()},
		{name: "serial-after-fido", command: protocol.SerialNumberCommand(iso7816.EncodingShort)},
	}

	for _, current := range commands {
		response, err := iso7816.Exchange(
			ctx,
			card,
			current.command,
			iso7816.WithMoreDataStatusBytes(0x61, 0x9f),
		)
		if err != nil {
			fmt.Fprintf(output, "TRACE %s transport_error=%v\n", current.name, err)
			return
		}
		fmt.Fprintf(
			output,
			"TRACE %s status=%s data_len=%d data_prefix=%x\n",
			current.name,
			response.Status,
			len(response.Data),
			response.Data[:min(len(response.Data), 32)],
		)
	}
}

func readDeviceInfo(ctx context.Context, reader string) (info token2.DeviceInfo, err error) {
	device, err := token2pcsc.Open(reader)
	if err != nil {
		return token2.DeviceInfo{}, fmt.Errorf("open PC/SC reader %q: %w", reader, err)
	}
	defer func() {
		err = errors.Join(err, device.Close())
	}()

	return device.DeviceInfo(ctx)
}
