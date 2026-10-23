package token2

import (
	"errors"
	"fmt"
)

// TransferType is the interface-state bitmask reported by the device.
type TransferType byte

const (
	TransferTypeFIDODisabledMask          TransferType = 0x01
	TransferTypeHOTPKeystrokeDisabledMask TransferType = 0x02
	TransferTypeCCIDDisabledMask          TransferType = 0x04
)

// DeviceConfiguration contains the device's configuration and capability bits.
type DeviceConfiguration byte

const (
	DeviceConfigurationHOTPSuppressEnterMask    DeviceConfiguration = 0x01
	DeviceConfigurationFIDOPINSetMask           DeviceConfiguration = 0x02
	DeviceConfigurationHOTPMask                 DeviceConfiguration = 0x04
	DeviceConfigurationFingerprintSensorMask    DeviceConfiguration = 0x08
	DeviceConfigurationNFCMask                  DeviceConfiguration = 0x10
	DeviceConfigurationHOTPLongPressMask        DeviceConfiguration = 0x20
	DeviceConfigurationFIDOPINLockedMask        DeviceConfiguration = 0x40
	DeviceConfigurationButtonHOTPConfiguredMask DeviceConfiguration = 0x80
)

// DeviceExtension contains the extended configuration and capability bits.
type DeviceExtension byte

const (
	DeviceExtensionTOTPMask                        DeviceExtension = 0x01
	DeviceExtensionFIDO21Mask                      DeviceExtension = 0x02
	DeviceExtensionFingerprintRegistrationMask     DeviceExtension = 0x04
	DeviceExtensionHOTPNumericKeypadMask           DeviceExtension = 0x08
	DeviceExtensionCCIDMask                        DeviceExtension = 0x10
	DeviceExtensionButtonHOTPUnsupportedMask       DeviceExtension = 0x20
	DeviceExtensionOTPRequiresFingerprintMask      DeviceExtension = 0x40
	DeviceExtensionMandatoryFingerprintSupportMask DeviceExtension = 0x80
)

// ErrInvalidConfiguration reports malformed configuration data received from
// a Token2 device.
var ErrInvalidConfiguration = errors.New("token2: invalid configuration")

// FIDOVersion is the three-component FIDO version reported by a Token2 device.
type FIDOVersion struct {
	Major byte `json:"major"`
	Minor byte `json:"minor"`
	Patch byte `json:"patch"`
}

// Appearance identifies the Token2 device appearance reported by its firmware.
type Appearance [4]byte

// Configuration holds the decoded response; Raw retains all bytes, including
// unknown fields.
type Configuration struct {
	Raw []byte `json:"raw"`

	TransferType        TransferType        `json:"transferType"`
	DeviceConfiguration DeviceConfiguration `json:"deviceConfiguration"`
	Appearance          Appearance          `json:"appearance"`
	FIDOVersion         FIDOVersion         `json:"fidoVersion"`
	DeviceExtension     DeviceExtension     `json:"deviceExtension"`
}

// ParseConfiguration parses either a one-byte legacy response or a modern
// response of at least ten bytes. Additional bytes are retained in Raw.
func ParseConfiguration(response []byte) (Configuration, error) {
	if len(response) == 0 || len(response) > 1 && len(response) < 10 {
		return Configuration{}, fmt.Errorf(
			"%w: got %d bytes, want 1 or at least 10",
			ErrInvalidConfiguration,
			len(response),
		)
	}

	config := Configuration{
		Raw:          response,
		TransferType: TransferType(response[0]),
	}
	if len(response) == 1 {
		return config, nil
	}

	config.DeviceConfiguration = DeviceConfiguration(response[1])
	config.Appearance = Appearance(response[2:6])
	config.FIDOVersion = FIDOVersion{Major: response[6], Minor: response[7], Patch: response[8]}
	config.DeviceExtension = DeviceExtension(response[9])

	return config, nil
}

// InterfaceState describes the interfaces enabled by the configuration mask.
type InterfaceState struct {
	FIDOEnabled          bool `json:"fidoEnabled"`
	HOTPKeystrokeEnabled bool `json:"hotpKeystrokeEnabled"`
	CCIDEnabled          bool `json:"ccidEnabled"`
}

// InterfaceState decodes the transfer-type mask, including legacy responses.
func (c Configuration) InterfaceState() InterfaceState {
	return InterfaceState{
		FIDOEnabled:          c.TransferType&TransferTypeFIDODisabledMask == 0,
		HOTPKeystrokeEnabled: c.TransferType&TransferTypeHOTPKeystrokeDisabledMask == 0,
		CCIDEnabled:          c.TransferType&TransferTypeCCIDDisabledMask == 0,
	}
}

// Capabilities contains flags reported in a modern configuration.
type Capabilities struct {
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

// Capabilities decodes the modern configuration masks. The boolean is false
// for a legacy response, which supplies only interface state.
func (c Configuration) Capabilities() (Capabilities, bool) {
	if len(c.Raw) < 10 {
		return Capabilities{}, false
	}
	return Capabilities{
		FIDOPINSet:                      c.DeviceConfiguration&DeviceConfigurationFIDOPINSetMask != 0,
		FIDOPINLocked:                   c.DeviceConfiguration&DeviceConfigurationFIDOPINLockedMask != 0,
		SupportsHOTP:                    c.DeviceConfiguration&DeviceConfigurationHOTPMask != 0,
		SupportsTOTP:                    c.DeviceExtension&DeviceExtensionTOTPMask != 0,
		SupportsNFC:                     c.DeviceConfiguration&DeviceConfigurationNFCMask != 0,
		SupportsCCID:                    c.DeviceExtension&DeviceExtensionCCIDMask != 0,
		SupportsFIDO21:                  c.DeviceExtension&DeviceExtensionFIDO21Mask != 0,
		HasFingerprintSensor:            c.DeviceConfiguration&DeviceConfigurationFingerprintSensorMask != 0,
		SupportsFingerprintRegistration: c.DeviceExtension&DeviceExtensionFingerprintRegistrationMask != 0,
		SupportsMandatoryFingerprint:    c.DeviceExtension&DeviceExtensionMandatoryFingerprintSupportMask != 0,
		OTPRequiresFingerprint:          c.DeviceExtension&DeviceExtensionOTPRequiresFingerprintMask != 0,
		SupportsButtonHOTP:              c.DeviceExtension&DeviceExtensionButtonHOTPUnsupportedMask == 0,
		ButtonHOTPConfigured:            c.DeviceConfiguration&DeviceConfigurationButtonHOTPConfiguredMask != 0,
		ButtonHOTPSendsEnter:            c.DeviceConfiguration&DeviceConfigurationHOTPSuppressEnterMask == 0,
		ButtonHOTPRequiresLongPress:     c.DeviceConfiguration&DeviceConfigurationHOTPLongPressMask != 0,
		ButtonHOTPUsesNumericKeypad:     c.DeviceExtension&DeviceExtensionHOTPNumericKeypadMask != 0,
	}, true
}
