package main

import (
	"io"
	"testing"

	nativepcsc "github.com/telesma-app/pcsc"
	token2pcsc "github.com/telesma-app/token2/transport/pcsc"
)

func TestRunRequiresExplicitSetting(t *testing.T) {
	err := run(t.Context(), nil, io.Discard)

	{
		err, want := err, "at least one of -sound or -nfc is required"
		if err == nil || err.Error() != want {
			t.Fatalf("got error %v, want %q", err, want)
		}
	}
}

func TestChooseReader(t *testing.T) {
	present := nativepcsc.ReaderStatePresent
	readers := []*nativepcsc.ReaderInfo{
		{Name: "Unrelated Reader 0", State: present},
		{Name: "Token2 Smart Reader 0", State: present},
		{Name: "Token2 Smart Reader 1"},
	}

	tests := []struct {
		name     string
		filter   string
		fallback bool
		want     string
	}{
		{
			name:     "prefer default reader",
			filter:   defaultReaderName,
			fallback: true,
			want:     "Token2 Smart Reader 0",
		},
		{
			name:     "fallback to first present reader",
			filter:   "Missing Reader",
			fallback: true,
			want:     "Unrelated Reader 0",
		},
		{
			name:   "explicit reader",
			filter: "Unrelated Reader",
			want:   "Unrelated Reader 0",
		},
		{
			name:   "explicit reader does not fallback",
			filter: "Missing Reader",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := chooseReader(readers, tt.filter, tt.fallback), tt.want; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}
		})
	}
}

func TestParseSoundLevel(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		level   token2pcsc.ReaderSoundLevel
		set     bool
		wantErr bool
	}{
		{name: "omitted"},
		{name: "off", value: "off", level: token2pcsc.ReaderSoundOff, set: true},
		{name: "off uppercase", value: "OFF", level: token2pcsc.ReaderSoundOff, set: true},
		{name: "zero", value: "0", level: token2pcsc.ReaderSoundOff, set: true},
		{name: "level 5", value: "5", level: token2pcsc.ReaderSoundLevel5, set: true},
		{name: "too high", value: "6", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
		{name: "not a number", value: "loud", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, set, err := parseSoundLevel(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got, want := level, tt.level; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}
			if got, want := set, tt.set; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}
		})
	}
}

func TestParseNFC(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		enabled bool
		set     bool
		wantErr bool
	}{
		{name: "omitted"},
		{name: "on", value: "on", enabled: true, set: true},
		{name: "on uppercase", value: "ON", enabled: true, set: true},
		{name: "off", value: "off", enabled: false, set: true},
		{name: "invalid", value: "enabled", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled, set, err := parseNFC(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got, want := enabled, tt.enabled; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}
			if got, want := set, tt.set; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}
		})
	}
}
