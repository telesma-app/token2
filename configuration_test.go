package token2

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestParseConfiguration(t *testing.T) {
	config, err := ParseConfiguration([]byte{0x02, 0x2a, 0x86, 0x01, 0x10, 0x00, 0x02, 0x01, 0x02, 0x37})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	{
		want, got := Appearance{0x86, 0x01, 0x10, 0x00}, config.Appearance
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	{
		want, got := FIDOVersion{2, 1, 2}, config.FIDOVersion
		if got != want {
			t.Errorf("got %#v, want %#v", got, want)
		}
	}
	if got, want := config.DeviceConfiguration, byte(0x2a); got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
	if got, want := config.DeviceExtension, byte(0x37); got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestParseConfigurationLegacy(t *testing.T) {
	config, err := ParseConfiguration([]byte{0x02})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := config.TransferType, byte(0x02); got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestConfigurationJSONContainsRawMasks(t *testing.T) {
	encoded, err := json.Marshal(Configuration{
		DeviceConfiguration: 0x2a,
		DeviceExtension:     0x37,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	{
		var want, got any
		if err := json.Unmarshal([]byte(`{
			"raw":null,
			"transferType":0,
			"deviceConfiguration":42,
			"appearance":[0,0,0,0],
			"fidoVersion":{"major":0,"minor":0,"patch":0},
			"deviceExtension":55
		}`), &want); err != nil {
			t.Fatalf("decode expected JSON: %v", err)
		}
		if err := json.Unmarshal([]byte(string(encoded)), &got); err != nil {
			t.Errorf("decode actual JSON: %v", err)
		} else if !jsonValuesEqual(t, got, want) {
			t.Errorf("got JSON %#v, want %#v", got, want)
		}
	}
}

func TestParseConfigurationRejectsTruncatedResponses(t *testing.T) {
	for length := 0; length < 10; length++ {
		if length == 1 {
			continue
		}

		_, err := ParseConfiguration(make([]byte, length))
		if err == nil {
			t.Fatalf("expected an error")
		}
		if got := errors.Is(err, ErrInvalidConfiguration); !got {
			t.Errorf("got false, want true")
		}
	}
}
