package pcsc

import (
	"context"
	"fmt"

	"github.com/go-ctap/token2/apdu"
	"github.com/go-ctap/token2/internal/protocol"
)

// ReaderSoundLevel is the sound level configured on a Token2 reader.
type ReaderSoundLevel byte

const (
	ReaderSoundOff ReaderSoundLevel = iota
	ReaderSoundLevel1
	ReaderSoundLevel2
	ReaderSoundLevel3
	ReaderSoundLevel4
	ReaderSoundLevel5
)

// SetReaderSoundLevel configures the Token2 reader sound level. A nil error
// confirms only that the reader returned 9000; unsupported readers may accept
// and ignore the command. Power-cycle the reader to apply the setting.
func (d *Device) SetReaderSoundLevel(ctx context.Context, level ReaderSoundLevel) error {
	if level > ReaderSoundLevel5 {
		return fmt.Errorf("invalid Token2 reader sound level %d", level)
	}

	return d.setReaderSetting(ctx, protocol.ReaderSoundCommand(byte(level)), "set Token2 reader sound level")
}

// SetReaderNFC enables or disables the Token2 reader NFC interface. A nil error
// confirms only that the reader returned 9000; unsupported readers may accept
// and ignore the command. Power-cycle the reader to apply the setting.
func (d *Device) SetReaderNFC(ctx context.Context, enabled bool) error {
	return d.setReaderSetting(ctx, protocol.ReaderNFCCommand(enabled), "set Token2 reader NFC")
}

func (d *Device) setReaderSetting(ctx context.Context, command apdu.Command, operation string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	response, err := apdu.Exchange(ctx, d.card, command)
	if err != nil {
		return err
	}

	return response.Err(operation)
}
