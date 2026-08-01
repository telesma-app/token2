package token2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentify(t *testing.T) {
	info, ok := Identify("72103654095303")
	require.True(t, ok)

	assert.Equal(t, "72103654095303", info.SerialNumber)
	assert.Equal(t, "R3.2", info.Release)
	assert.Equal(t, "Bio3 Dual A+C PIN+", info.FormFactor)
	assert.Equal(t, "Token2", info.Branding)
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
	require.NoError(t, err)
	assert.JSONEq(t, `{
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
	}`, string(encoded))
}

func TestIdentifyCustomCard(t *testing.T) {
	info, ok := Identify("70000042")
	require.True(t, ok)

	assert.Equal(t, "R3.1", info.Release)
	assert.Equal(t, "Custom system access card", info.FormFactor)
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

			assert.False(t, ok)
			assert.Equal(t, serialNumber, info.SerialNumber)
			assert.Empty(t, info.Release)
			assert.Empty(t, info.FormFactor)
			assert.Empty(t, info.Branding)
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
			assert.Equal(t, tt.want, tt.info.ModelName(tt.fallback))
		})
	}
}
