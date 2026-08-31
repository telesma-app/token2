package token2

import (
	"encoding/json"
	"testing"
)

func TestIdentify(t *testing.T) {
	info, ok := Identify("72103654095303")
	if got := ok; !got {
		t.Fatalf("got false, want true")
	}

	if got, want := info.SerialNumber, "72103654095303"; got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
	if got, want := info.Release, "R3.2"; got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
	if got, want := info.FormFactor, "Bio3 Dual A+C PIN+"; got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
	if got, want := info.Branding, "Token2"; got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestDeviceInfoJSON(t *testing.T) {
	appearance := Appearance{0x86, 0x01, 0x10, 0x00}
	version := FIDOVersion{Major: 2, Minor: 1, Patch: 2}
	info := DeviceInfo{
		SerialNumber:                 "72103654095303",
		Release:                      "R3.2",
		FormFactor:                   "Bio3 Dual A+C PIN+",
		Branding:                     "Token2",
		ProductID:                    0x1234,
		Appearance:                   &appearance,
		FIDOVersion:                  &version,
		InterfaceStateKnown:          true,
		FIDOEnabled:                  true,
		CCIDEnabled:                  true,
		CapabilitiesKnown:            true,
		FIDOPINSet:                   true,
		SupportsHOTP:                 true,
		SupportsTOTP:                 true,
		SupportsNFC:                  true,
		SupportsCCID:                 true,
		SupportsFIDO21:               true,
		HasFingerprintSensor:         true,
		SupportsMandatoryFingerprint: true,
		SupportsButtonHOTP:           true,
		ButtonHOTPConfigured:         true,
		ButtonHOTPRequiresLongPress:  true,
		ButtonHOTPUsesNumericKeypad:  true,
	}

	encoded, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	{
		var want, got any
		if err := json.Unmarshal([]byte(`{
			"serialNumber":"72103654095303",
			"release":"R3.2",
			"formFactor":"Bio3 Dual A+C PIN+",
			"branding":"Token2",
			"productId":4660,
			"appearance":[134,1,16,0],
			"fidoVersion":{"major":2,"minor":1,"patch":2},
			"interfaceStateKnown":true,
			"fidoEnabled":true,
			"hotpKeystrokeEnabled":false,
			"ccidEnabled":true,
			"capabilitiesKnown":true,
			"fidoPINSet":true,
			"fidoPINLocked":false,
			"supportsHOTP":true,
			"supportsTOTP":true,
			"supportsNFC":true,
			"supportsCCID":true,
			"supportsFIDO21":true,
			"hasFingerprintSensor":true,
			"supportsFingerprintRegistration":false,
			"supportsMandatoryFingerprint":true,
			"otpRequiresFingerprint":false,
			"supportsButtonHOTP":true,
			"buttonHOTPConfigured":true,
			"buttonHOTPSendsEnter":false,
			"buttonHOTPRequiresLongPress":true,
			"buttonHOTPUsesNumericKeypad":true
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

func TestIdentifyCustomCard(t *testing.T) {
	info, ok := Identify("70000042")
	if got := ok; !got {
		t.Fatalf("got false, want true")
	}

	if got, want := info.Release, "R3.1"; got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
	if got, want := info.FormFactor, "Custom system access card"; got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestIdentifyRejectsInvalidSerialNumber(t *testing.T) {
	for _, serialNumber := range []string{
		"",
		"72103",
		"721036",
		"72103x54095303",
		"+72103654095303",
		"18446744073709551616",
	} {
		t.Run(serialNumber, func(t *testing.T) {
			info, ok := Identify(serialNumber)

			if got := ok; got {
				t.Errorf("got true, want false")
			}
			if got, want := info.SerialNumber, serialNumber; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}
			if got := info.Release; len(got) != 0 {
				t.Errorf("got non-empty value %#v", got)
			}
			if got := info.FormFactor; len(got) != 0 {
				t.Errorf("got non-empty value %#v", got)
			}
			if got := info.Branding; len(got) != 0 {
				t.Errorf("got non-empty value %#v", got)
			}
		})
	}
}

func TestDeviceInfoModelName(t *testing.T) {
	tests := []struct {
		name     string
		info     DeviceInfo
		fallback string
		want     string
	}{
		{
			name: "complete model",
			info: DeviceInfo{
				Branding:   "Token2",
				FormFactor: "Bio3 Dual A+C PIN+",
				Release:    "R3.2",
			},
			want: "Token2 Bio3 Dual A+C PIN+",
		},
		{
			name: "model without branding",
			info: DeviceInfo{
				FormFactor: "Mini USB-C PIN+",
				Release:    "R3.1",
			},
			want: "Mini USB-C PIN+",
		},
		{
			name:     "unknown model",
			fallback: "USB security key",
			want:     "USB security key",
		},
		{
			name: "trims catalog fields",
			info: DeviceInfo{
				Branding:   " Token2 ",
				FormFactor: " USB-A NFC ",
				Release:    " R1 ",
			},
			want: "Token2 USB-A NFC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := tt.info.ModelName(tt.fallback), tt.want; got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}
		})
	}
}
