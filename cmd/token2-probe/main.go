// token2-probe records read-only observations from one manually labelled token.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/telesma-app/hid"
	"github.com/telesma-app/pcsc"
)

type options struct {
	label, connection, output, reader string
	worker, endpoint, hidPath         string
	repeat, attempt                   int
	timeout                           time.Duration
	list, readerExact                 bool
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil && !errors.Is(err, flag.ErrHelp) && ctx.Err() == nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	var o options
	f := flag.NewFlagSet("token2-probe", flag.ContinueOnError)
	f.SetOutput(output)
	f.StringVar(&o.label, "label", "", "manual firmware/sample label; omit for interactive mode")
	f.StringVar(&o.connection, "connection", "usb", "usb or nfc")
	f.StringVar(&o.output, "out", filepath.Join(os.TempDir(), "token2-probe"), "directory for timestamped JSONL captures")
	f.StringVar(&o.reader, "reader", "", "PC/SC reader-name substring; default: Token2 USB readers or Token2 Smart Reader for NFC")
	f.BoolVar(&o.readerExact, "reader-exact", false, "match -reader as a complete name")
	f.StringVar(&o.hidPath, "hid-path", "", "probe only this HID path; default: all Token2 HID interfaces")
	f.IntVar(&o.repeat, "repeat", 3, "read repetitions per interface, without power cycling")
	f.DurationVar(&o.timeout, "timeout", 45*time.Second, "hard deadline per interface/repetition, including native calls")
	f.BoolVar(&o.list, "list", false, "list interfaces without sending token commands")
	f.StringVar(&o.worker, "worker", "", "internal: pcsc, feature or ctaphid")
	f.StringVar(&o.endpoint, "endpoint", "", "internal: reader name or HID path")
	f.IntVar(&o.attempt, "attempt", 1, "internal: repetition number")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 || o.repeat < 1 || o.timeout <= 0 || (o.connection != "usb" && o.connection != "nfc") {
		return errors.New("expected -connection usb|nfc, positive -repeat and -timeout, and no positional arguments")
	}
	if o.worker != "" {
		r := recorder{output: output, label: o.label, connection: o.connection, endpoint: o.endpoint, attempt: o.attempt}
		return runWorker(ctx, o, &r)
	}
	if o.list {
		_, err := inventory(o, &recorder{output: output})
		return err
	}
	if o.label != "" {
		return capture(ctx, o, output)
	}
	fmt.Fprintln(output, "Оставь подключённым только исследуемый токен. Для NFC отключи его USB и положи на ридер.")
	fmt.Fprintln(output, "Вводи метку и подключение: R3.1 usb, R3.1 nfc, R3.2 usb, ...; q — выход.")
	scanner := bufio.NewScanner(input)
	// A terminal read can remain blocked after cancellation (even on Close).
	// Let the CLI return on Ctrl-C while the input goroutine waits for input.
	lines := make(chan string)
	go func() {
		defer close(lines)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
	}()
	for {
		fmt.Fprint(output, "Образец> ")
		var line string
		select {
		case <-ctx.Done():
			return nil
		case text, ok := <-lines:
			if !ok {
				return scanner.Err()
			}
			line = strings.TrimSpace(text)
		}
		if line == "q" || ctx.Err() != nil {
			return nil
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || (fields[1] != "usb" && fields[1] != "nfc") {
			fmt.Fprintln(output, "Нужны метка и подключение, например: R3.1 usb")
			continue
		}
		o.label, o.connection = fields[0], fields[1]
		if err := capture(ctx, o, output); err != nil {
			return err
		}
	}
}

type target struct{ kind, endpoint string }

func inventory(o options, r *recorder) ([]target, error) {
	var targets []target
	var inventoryErr error
	reader := o.reader
	if reader == "" {
		reader = "TOKEN2 FIDO2 Security Key"
		if o.connection == "nfc" {
			reader = "Token2 Smart Reader"
		}
	}
	// PC/SC goes first so CTAPHID GetInfo cannot prime a legacy card before
	// the initial OTP serial read. No correlation between interfaces is inferred.
	for info, err := range pcsc.Enumerate() {
		if err != nil {
			r.emit("inventory.pcsc", map[string]any{"error": err.Error()})
			inventoryErr = errors.Join(inventoryErr, err)
			break
		}
		r.emit("inventory.pcsc", map[string]any{"reader": info, "atrHex": fmt.Sprintf("%x", info.ATR)})
		matches := strings.Contains(strings.ToLower(info.Name), strings.ToLower(reader))
		if o.readerExact {
			matches = info.Name == reader
		}
		if info.State&pcsc.ReaderStatePresent != 0 && matches {
			targets = append(targets, target{"pcsc", info.Name})
		}
	}
	if o.connection == "usb" {
		for info, err := range hid.Enumerate(hid.WithVendorID(0x349e)) {
			if err != nil {
				r.emit("inventory.hid", map[string]any{"error": err.Error()})
				inventoryErr = errors.Join(inventoryErr, err)
				break
			}
			r.emit("inventory.hid", map[string]any{"device": info})
			if o.hidPath != "" && o.hidPath != info.Path {
				continue
			}
			targets = append(targets, target{"feature", info.Path})
			if info.UsagePage == 0xf1d0 && info.Usage == 1 {
				targets = append(targets, target{"ctaphid", info.Path})
			}
		}
	}
	return targets, inventoryErr
}

func capture(ctx context.Context, o options, output io.Writer) (err error) {
	if err := os.MkdirAll(o.output, 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(o.output, time.Now().UTC().Format("20060102T150405Z-")+"*.jsonl")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Sync(), file.Close()) }()
	r := &recorder{output: file, label: o.label, connection: o.connection}
	build, _ := debug.ReadBuildInfo()
	r.emit("session.start", map[string]any{"build": build, "repeat": o.repeat, "timeout": o.timeout.String(), "readOnly": true})
	fmt.Fprintf(output, "\n%s %s → %s\n", o.label, o.connection, file.Name())
	targets, inventoryErr := inventory(o, r)
	if inventoryErr != nil {
		fmt.Fprintf(output, "Ошибка перечисления интерфейсов: %v\n", inventoryErr)
	}
	if len(targets) == 0 {
		r.emit("session.end", map[string]any{"targets": 0})
		fmt.Fprintln(output, "Доступных интерфейсов нет. Проверь подключение и повтори замер.")
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	for _, target := range targets {
		for attempt := 1; attempt <= o.repeat; attempt++ {
			if ctx.Err() != nil {
				r.emit("session.interrupted", nil)
				return nil
			}
			fmt.Fprintf(output, "  %s [%d/%d] %s\n", target.kind, attempt, o.repeat, target.endpoint)
			workerCtx, cancel := context.WithTimeout(ctx, o.timeout)
			cmd := exec.CommandContext(workerCtx, executable, "-worker", target.kind, "-endpoint", target.endpoint,
				"-label", o.label, "-connection", o.connection, "-attempt", fmt.Sprint(attempt))
			cmd.Stdout, cmd.Stderr = file, output
			workerErr := cmd.Run()
			deadlineErr := workerCtx.Err()
			cancel()
			r.emit("worker.end", map[string]any{"kind": target.kind, "target": target.endpoint, "repetition": attempt,
				"error": errorText(workerErr), "deadlineError": errorText(deadlineErr)})
			if workerErr != nil {
				fmt.Fprintf(output, "    Процесс проверки завершился с ошибкой: %v (%v)\n", workerErr, deadlineErr)
			}
			if err := file.Sync(); err != nil {
				return err
			}
		}
	}
	r.emit("session.end", map[string]any{"targets": len(targets)})
	path, err := filepath.Abs(file.Name())
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "Замер сохранён: %s\nТеперь можно отключать токен или менять USB на NFC.\n", path)
	return nil
}

func runWorker(ctx context.Context, o options, r *recorder) error {
	r.emit("worker.start", map[string]any{"kind": o.worker})
	switch o.worker {
	case "pcsc":
		return probePCSC(ctx, o.endpoint, r)
	case "feature":
		return probeFeature(ctx, o.endpoint, r)
	case "ctaphid":
		return probeCTAPHID(ctx, o.endpoint, r)
	default:
		return fmt.Errorf("unknown worker %q", o.worker)
	}
}
