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
	if got, want := config.DeviceConfiguration, DeviceConfiguration(0x2a); got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
	if got, want := config.DeviceExtension, DeviceExtension(0x37); got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestParseConfigurationLegacy(t *testing.T) {
	config, err := ParseConfiguration([]byte{0x02})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := config.TransferType, TransferType(0x02); got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestConfigurationJSONContainsRawMasks(t *testing.T) {
	encoded, err := json.Marshal(Configuration{
		TransferType:        TransferTypeHOTPKeystrokeDisabledMask,
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
			"transferType":2,
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

func TestConfigurationDecodedMasks(t *testing.T) {
	c, err := ParseConfiguration([]byte{2, 0xaa, 0x86, 1, 0x10, 0, 2, 1, 2, 0xd5})
	if err != nil {
		t.Fatal(err)
	}
	state := c.InterfaceState()
	if !state.FIDOEnabled || state.HOTPKeystrokeEnabled || !state.CCIDEnabled {
		t.Fatalf("state = %#v", state)
	}
	capabilities, ok := c.Capabilities()
	if !ok || !capabilities.FIDOPINSet || capabilities.FIDOPINLocked || capabilities.SupportsHOTP ||
		!capabilities.SupportsTOTP || capabilities.SupportsNFC || !capabilities.SupportsCCID || capabilities.SupportsFIDO21 ||
		!capabilities.HasFingerprintSensor || !capabilities.SupportsFingerprintRegistration ||
		!capabilities.SupportsMandatoryFingerprint || !capabilities.OTPRequiresFingerprint || !capabilities.SupportsButtonHOTP ||
		!capabilities.ButtonHOTPConfigured || !capabilities.ButtonHOTPSendsEnter || !capabilities.ButtonHOTPRequiresLongPress ||
		capabilities.ButtonHOTPUsesNumericKeypad {
		t.Fatalf("capabilities = %#v, known = %t", capabilities, ok)
	}
}

func TestConfigurationDecodesNegativeBits(t *testing.T) {
	c, err := ParseConfiguration([]byte{7, byte(DeviceConfigurationHOTPSuppressEnterMask), 0, 0, 0, 0, 0, 0, 0, byte(DeviceExtensionButtonHOTPUnsupportedMask)})
	if err != nil {
		t.Fatal(err)
	}
	if got := c.InterfaceState(); got != (InterfaceState{}) {
		t.Fatalf("state = %#v", got)
	}
	caps, ok := c.Capabilities()
	if !ok || caps.ButtonHOTPSendsEnter || caps.SupportsButtonHOTP {
		t.Fatalf("capabilities = %#v, known = %t", caps, ok)
	}
}

func TestLegacyConfigurationHasNoCapabilities(t *testing.T) {
	c, err := ParseConfiguration([]byte{2})
	if err != nil {
		t.Fatal(err)
	}
	if got := c.InterfaceState(); !got.FIDOEnabled || got.HOTPKeystrokeEnabled || !got.CCIDEnabled {
		t.Fatalf("state = %#v", got)
	}
	if caps, ok := c.Capabilities(); ok || caps != (Capabilities{}) {
		t.Fatalf("capabilities = %#v, known = %t", caps, ok)
	}
}
