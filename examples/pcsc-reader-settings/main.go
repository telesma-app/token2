package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	nativepcsc "github.com/go-ctap/pcsc"
	token2pcsc "github.com/go-ctap/token2/transport/pcsc"
)

const defaultReaderName = "Token2 Smart Reader"

func main() {
	err := run(context.Background(), os.Args[1:], os.Stdout)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string, output io.Writer) (err error) {
	flags := flag.NewFlagSet("pcsc-reader-settings", flag.ContinueOnError)
	flags.SetOutput(output)

	readerFilter := flags.String("reader", defaultReaderName, "PC/SC reader-name substring")
	soundValue := flags.String("sound", "", "sound level: off or 0-5 (unchanged if omitted)")
	nfcValue := flags.String("nfc", "", "NFC state: on or off (unchanged if omitted)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	sound, setSound, err := parseSoundLevel(*soundValue)
	if err != nil {
		return err
	}
	nfcEnabled, setNFC, err := parseNFC(*nfcValue)
	if err != nil {
		return err
	}
	if !setSound && !setNFC {
		return errors.New("at least one of -sound or -nfc is required")
	}

	readerExplicit := false
	flags.Visit(func(current *flag.Flag) {
		if current.Name == "reader" {
			readerExplicit = true
		}
	})

	reader, err := findReader(*readerFilter, !readerExplicit)
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

	fmt.Fprintf(output, "PC/SC reader: %s\n", reader)

	acknowledged := false
	defer func() {
		if acknowledged {
			fmt.Fprintln(output, "Power-cycle the reader to apply the setting. Acknowledgement does not prove that the reader supports it.")
		}
	}()

	if setSound {
		if err := device.SetReaderSoundLevel(ctx, sound); err != nil {
			return fmt.Errorf("set reader sound level: %w", err)
		}
		acknowledged = true
		fmt.Fprintf(output, "Sound command acknowledged: level %d\n", sound)
	}

	if setNFC {
		if err := device.SetReaderNFC(ctx, nfcEnabled); err != nil {
			return fmt.Errorf("set reader NFC: %w", err)
		}
		acknowledged = true
		fmt.Fprintf(output, "NFC command acknowledged: %s\n", onOff(nfcEnabled))
	}

	return nil
}

func parseSoundLevel(value string) (token2pcsc.ReaderSoundLevel, bool, error) {
	if value == "" {
		return 0, false, nil
	}
	if strings.EqualFold(value, "off") {
		return token2pcsc.ReaderSoundOff, true, nil
	}

	level, err := strconv.Atoi(value)
	if err != nil || level < 0 || level > 5 {
		return 0, false, fmt.Errorf("invalid -sound value %q: want off or 0-5", value)
	}

	return token2pcsc.ReaderSoundLevel(level), true, nil
}

func parseNFC(value string) (bool, bool, error) {
	switch strings.ToLower(value) {
	case "":
		return false, false, nil
	case "on":
		return true, true, nil
	case "off":
		return false, true, nil
	default:
		return false, false, fmt.Errorf("invalid -nfc value %q: want on or off", value)
	}
}

func findReader(filter string, fallback bool) (string, error) {
	var readers []*nativepcsc.ReaderInfo
	for reader, err := range nativepcsc.Enumerate() {
		if err != nil {
			return "", fmt.Errorf("enumerate PC/SC readers: %w", err)
		}
		readers = append(readers, reader)
	}

	if reader := chooseReader(readers, filter, fallback); reader != "" {
		return reader, nil
	}
	if fallback {
		return "", errors.New("no PC/SC readers with a card present")
	}
	return "", fmt.Errorf("no PC/SC reader matching %q with a card present", filter)
}

func chooseReader(readers []*nativepcsc.ReaderInfo, filter string, fallback bool) string {
	fallbackReader := ""
	for _, reader := range readers {
		if reader.State&nativepcsc.ReaderStatePresent == 0 {
			continue
		}
		if strings.Contains(reader.Name, filter) {
			return reader.Name
		}
		if fallback && fallbackReader == "" {
			fallbackReader = reader.Name
		}
	}

	return fallbackReader
}

func onOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}
