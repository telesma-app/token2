package pcsc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/telesma-app/iso7816"
)

func TestSetReaderSoundLevel(t *testing.T) {
	tests := []struct {
		name  string
		level ReaderSoundLevel
		value byte
	}{
		{name: "off", level: ReaderSoundOff, value: 0x00},
		{name: "level 1", level: ReaderSoundLevel1, value: 0x01},
		{name: "level 2", level: ReaderSoundLevel2, value: 0x02},
		{name: "level 3", level: ReaderSoundLevel3, value: 0x03},
		{name: "level 4", level: ReaderSoundLevel4, value: 0x04},
		{name: "level 5", level: ReaderSoundLevel5, value: 0x05},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type contextKey struct{}
			ctx := context.WithValue(t.Context(), contextKey{}, "sound")
			command := []byte{
				0x90, 0xff, 0x55, 0x77, 0x08,
				0x80, 0x33, 0x00, 0x00, 0x03, 0x02, 0x01, tt.value,
			}
			card := &scriptedCard{steps: []cardStep{
				{command: command, response: successfulResponse(nil)},
			}}

			err := (&Device{card: card}).SetReaderSoundLevel(ctx, tt.level)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := card.steps; len(got) != 0 {
				t.Errorf("got non-empty value %#v", got)
			}
			if got := card.begins; !(got == 0) {
				t.Errorf("got %#v, want zero value", got)
			}
			if got := card.ends; !(got == 0) {
				t.Errorf("got %#v, want zero value", got)
			}
			if got, want := len(card.contexts), 1; got != want {
				t.Fatalf("got length %d, want %d", got, want)
			}
			{
				want, got := "sound", card.contexts[0].Value(contextKey{})
				gotValue, ok := got.(string)

				if !ok || gotValue != want {
					t.Errorf("got %#v, want %#v", got, want)
				}
			}
		})
	}
}

func TestSetReaderNFC(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		value   byte
	}{
		{name: "enabled", enabled: true, value: 0x00},
		{name: "disabled", enabled: false, value: 0x01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := []byte{
				0x90, 0xff, 0x55, 0x77, 0x08,
				0x80, 0x33, 0x00, 0x00, 0x03, 0x03, 0x01, tt.value,
			}
			card := &scriptedCard{steps: []cardStep{
				{command: command, response: successfulResponse(nil)},
			}}

			err := (&Device{card: card}).SetReaderNFC(t.Context(), tt.enabled)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := card.steps; len(got) != 0 {
				t.Errorf("got non-empty value %#v", got)
			}
			if got := card.begins; !(got == 0) {
				t.Errorf("got %#v, want zero value", got)
			}
			if got := card.ends; !(got == 0) {
				t.Errorf("got %#v, want zero value", got)
			}
		})
	}
}

func TestSetReaderSoundLevelRejectsInvalidLevel(t *testing.T) {
	card := &scriptedCard{}

	err := (&Device{card: card}).SetReaderSoundLevel(t.Context(), ReaderSoundLevel5+1)

	if err == nil {
		t.Fatalf("expected an error")
	}
	{
		container, element := err.Error(), "sound level 6"
		if !strings.Contains(container, element) {
			t.Errorf("value does not contain %#v", element)
		}
	}
	if got := card.sent; len(got) != 0 {
		t.Errorf("got non-empty value %#v", got)
	}
}

func TestSetReaderSettingErrors(t *testing.T) {
	command := []byte{
		0x90, 0xff, 0x55, 0x77, 0x08,
		0x80, 0x33, 0x00, 0x00, 0x03, 0x02, 0x01, 0x00,
	}
	transmitErr := errors.New("transmit failed")

	tests := []struct {
		name     string
		response []byte
		err      error
		check    func(*testing.T, error)
	}{
		{
			name: "transport",
			err:  transmitErr,
			check: func(t *testing.T, err error) {
				{
					err, target := err, transmitErr
					if !errors.Is(err, target) {
						t.Fatalf("got error %v, want errors.Is(error, %#v)", err, target)
					}
				}
			},
		},
		{
			name:     "malformed response",
			response: []byte{0x90},
			check: func(t *testing.T, err error) {
				if err == nil {
					t.Fatalf("expected an error")
				}
				{
					container, element := err.Error(), "invalid response APDU"
					if !strings.Contains(container, element) {
						t.Errorf("value does not contain %#v", element)
					}
				}
			},
		},
		{
			name:     "status",
			response: statusResponse(0x6d00),
			check: func(t *testing.T, err error) {
				var statusErr *iso7816.APDUError
				if err := err; !errors.As(err, &statusErr) {
					t.Fatalf("error %v does not match requested type", err)
				}
				{
					want, got := iso7816.StatusWord(0x6d00), statusErr.StatusWord()
					if got != want {
						t.Errorf("got %#v, want %#v", got, want)
					}
				}
				{
					container, element := err.Error(), "set Token2 reader sound level"
					if !strings.Contains(container, element) {
						t.Errorf("value does not contain %#v", element)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &scriptedCard{steps: []cardStep{
				{command: command, response: tt.response, err: tt.err},
			}}

			err := (&Device{card: card}).SetReaderSoundLevel(t.Context(), ReaderSoundOff)

			tt.check(t, err)
			if got := card.steps; len(got) != 0 {
				t.Errorf("got non-empty value %#v", got)
			}
		})
	}
}
