package token2

import (
	"strconv"
	"strings"
)

// DeviceInfo contains normalized, transport-neutral information about a
// Token2 device. Interface state and capability fields are meaningful when
// their corresponding Known field is true.
type DeviceInfo struct {
	SerialNumber string `json:"serialNumber"`
	Release      string `json:"release"`
	FormFactor   string `json:"formFactor"`
	Branding     string `json:"branding"`
	ProductID    uint16 `json:"productId,omitempty"`

	Appearance  *Appearance  `json:"appearance,omitempty"`
	FIDOVersion *FIDOVersion `json:"fidoVersion,omitempty"`

	InterfaceStateKnown  bool `json:"interfaceStateKnown"`
	FIDOEnabled          bool `json:"fidoEnabled"`
	HOTPKeystrokeEnabled bool `json:"hotpKeystrokeEnabled"`
	CCIDEnabled          bool `json:"ccidEnabled"`

	CapabilitiesKnown               bool `json:"capabilitiesKnown"`
	FIDOPINSet                      bool `json:"fidoPINSet"`
	FIDOPINLocked                   bool `json:"fidoPINLocked"`
	SupportsHOTP                    bool `json:"supportsHOTP"`
	SupportsTOTP                    bool `json:"supportsTOTP"`
	SupportsNFC                     bool `json:"supportsNFC"`
	SupportsCCID                    bool `json:"supportsCCID"`
	SupportsFIDO21                  bool `json:"supportsFIDO21"`
	HasFingerprintSensor            bool `json:"hasFingerprintSensor"`
	SupportsFingerprintRegistration bool `json:"supportsFingerprintRegistration"`
	SupportsMandatoryFingerprint    bool `json:"supportsMandatoryFingerprint"`
	OTPRequiresFingerprint          bool `json:"otpRequiresFingerprint"`
	SupportsButtonHOTP              bool `json:"supportsButtonHOTP"`
	ButtonHOTPConfigured            bool `json:"buttonHOTPConfigured"`
	ButtonHOTPSendsEnter            bool `json:"buttonHOTPSendsEnter"`
	ButtonHOTPRequiresLongPress     bool `json:"buttonHOTPRequiresLongPress"`
	ButtonHOTPUsesNumericKeypad     bool `json:"buttonHOTPUsesNumericKeypad"`
}

// ModelName returns the canonical model name derived from the serial number.
// fallback is returned when the model is not in the built-in catalog.
func (info DeviceInfo) ModelName(fallback string) string {
	parts := make([]string, 0, 3)
	for _, part := range []string{info.Branding, info.FormFactor} {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return fallback
	}

	return strings.Join(parts, " ")
}

type model struct {
	Release    string
	FormFactor string
	Branding   string
	Prefix     string
	SerialFrom uint64
	SerialTo   uint64
}

var models = []model{
	{Release: "R1", FormFactor: "USB-A NFC", Branding: "Token2", Prefix: "86105"},
	{Release: "R1", FormFactor: "USB-C NFC", Branding: "Token2", Prefix: "86104"},
	{Release: "R1", FormFactor: "Dual NFC", Branding: "Token2", Prefix: "86103"},
	{Release: "R1", FormFactor: "FIDO Card", Branding: "Token2", Prefix: "86202"},

	{Release: "R2", FormFactor: "USB-A PIN+ NFC", Branding: "Token2", Prefix: "96105"},
	{Release: "R2", FormFactor: "USB-C PIN+ NFC", Branding: "Token2", Prefix: "96104"},
	{Release: "R2", FormFactor: "Dual PIN+ NFC", Branding: "Token2", Prefix: "96103"},
	{Release: "R2", FormFactor: "Dual PIN+ NFC", Branding: "Unbranded", Prefix: "23103"},

	{Release: "R3", FormFactor: "Dual PIN+ NFC", Branding: "Token2", Prefix: "76103"},
	{Release: "R3", FormFactor: "USB-C PIN+ NFC", Branding: "Token2", Prefix: "76104"},
	{Release: "R3", FormFactor: "FIDO Card", Branding: "Token2", Prefix: "76202"},
	{Release: "R3", FormFactor: "FIDO Card without ISO 7816", Branding: "Unbranded", Prefix: "86106"},
	{Release: "R3", FormFactor: "FIDO Card with ISO 7816", Branding: "Unbranded", Prefix: "76106"},

	{Release: "R3.1", FormFactor: "USB-A PIN+ NFC", Branding: "Token2", Prefix: "76105"},
	{Release: "R3.1", FormFactor: "USB-A PIN+ NFC", Branding: "Unbranded", Prefix: "26105"},
	{Release: "R3.1", FormFactor: "Mini USB-C PIN+", Prefix: "72102"},
	{Release: "R3.1", FormFactor: "Custom system access card", Branding: "Custom", SerialFrom: 70000001, SerialTo: 70002000},

	{Release: "R3.2", FormFactor: "Dual PIN+ NFC", Branding: "Token2", Prefix: "77103"},
	{Release: "R3.2", FormFactor: "Dual PIN+ NFC", Branding: "Unbranded", Prefix: "24103"},
	{Release: "R3.2", FormFactor: "Mini USB-A PIN+", Prefix: "72101"},
	{Release: "R3.2", FormFactor: "Bio3 Dual A+C PIN+", Branding: "Token2", Prefix: "72103"},
	{Release: "R3.2", FormFactor: "Bio3 Dual A+C PIN+", Branding: "Unbranded", Prefix: "22103"},

	{Release: "R3.3", FormFactor: "USB-A NFC PIN+ PIV+", Branding: "Token2", Prefix: "66105"},
	{Release: "R3.3", FormFactor: "USB-C NFC PIN+ PIV+", Branding: "Token2", Prefix: "66104"},
	{Release: "R3.3", FormFactor: "Dual NFC PIN+ PIV+", Branding: "Token2", Prefix: "66103"},
	{Release: "R3.3", FormFactor: "USB-A NFC PIN+ PIV+", Branding: "Unbranded", Prefix: "66107"},
	{Release: "R3.3", FormFactor: "USB-C NFC PIN+ PIV+", Branding: "Unbranded", Prefix: "66106"},
	{Release: "R3.3", FormFactor: "Dual NFC PIN+ PIV+", Branding: "Unbranded", Prefix: "66114"},
	{Release: "R3.3", FormFactor: "Dual NFC PIN+ PIV+", Branding: "Unbranded Octo", Prefix: "66113"},
	{Release: "R3.3", FormFactor: "FIDO Card NFC with ISO 7816 PIN+ PIV+", Branding: "Token2", Prefix: "66202"},
	{Release: "R3.3", FormFactor: "FIDO Card PIN+ PIV+", Branding: "Unbranded", Prefix: "66102"},
	{Release: "R3.3", FormFactor: "FIDO Card NFC with ISO 7816 PIN+ PIV+", Branding: "Unbranded", Prefix: "66302"},
	{Release: "R3.3", FormFactor: "Mini USB-A PIN+ PIV+", Prefix: "66101"},
	{Release: "R3.3", FormFactor: "Mini USB-C PIN+ PIV+", Prefix: "66111"},
	{Release: "R3.3", FormFactor: "Dual Bio3 PIN+ PIV+", Branding: "Token2", Prefix: "72113"},
	{Release: "R3.3", FormFactor: "Dual Bio3 PIN+ PIV+", Branding: "Unbranded", Prefix: "24133"},

	{Release: "R3.4", FormFactor: "PIN+ Dual Ace PIV+ OTP Protection", Branding: "Token2", Prefix: "65103"},
	{Release: "R3.4", FormFactor: "Dual Bio3 PIV+ OTP Protection", Branding: "Token2", Prefix: "72114"},
	{Release: "R3.4", FormFactor: "Mini USB-A PIN+ PIV+ OTP Protection", Prefix: "65101"},
	{Release: "R3.4", FormFactor: "Mini USB-C PIN+ PIV+ OTP Protection", Prefix: "65111"},
}

// Identify returns device information derived from a full Token2 serial
// number. The boolean reports whether the model is in the built-in catalog.
func Identify(serialNumber string) (DeviceInfo, bool) {
	info := DeviceInfo{SerialNumber: serialNumber}
	if len(serialNumber) < 7 {
		return info, false
	}

	for i := range len(serialNumber) {
		if serialNumber[i] < '0' || serialNumber[i] > '9' {
			return info, false
		}
	}

	serial, err := strconv.ParseUint(serialNumber, 10, 64)
	if err != nil {
		return info, false
	}

	for _, candidate := range models {
		if candidate.SerialFrom != 0 && serial >= candidate.SerialFrom && serial <= candidate.SerialTo {
			info.Release = candidate.Release
			info.FormFactor = candidate.FormFactor
			info.Branding = candidate.Branding
			return info, true
		}
	}

	prefix := serialNumber[:5]
	for _, candidate := range models {
		if candidate.Prefix == prefix {
			info.Release = candidate.Release
			info.FormFactor = candidate.FormFactor
			info.Branding = candidate.Branding
			return info, true
		}
	}

	return info, false
}
