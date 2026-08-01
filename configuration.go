package token2

import (
	"errors"
	"fmt"
)

const (
	TransferTypeFIDODisabledMask          byte = 0x01
	TransferTypeHOTPKeystrokeDisabledMask byte = 0x02
	TransferTypeCCIDDisabledMask          byte = 0x04

	DeviceConfigurationHOTPSuppressEnterMask    byte = 0x01
	DeviceConfigurationFIDOPINSetMask           byte = 0x02
	DeviceConfigurationHOTPMask                 byte = 0x04
	DeviceConfigurationFingerprintSensorMask    byte = 0x08
	DeviceConfigurationNFCMask                  byte = 0x10
	DeviceConfigurationHOTPLongPressMask        byte = 0x20
	DeviceConfigurationFIDOPINLockedMask        byte = 0x40
	DeviceConfigurationButtonHOTPConfiguredMask byte = 0x80

	DeviceExtensionTOTPMask                        byte = 0x01
	DeviceExtensionFIDO21Mask                      byte = 0x02
	DeviceExtensionFingerprintRegistrationMask     byte = 0x04
	DeviceExtensionHOTPNumericKeypadMask           byte = 0x08
	DeviceExtensionCCIDMask                        byte = 0x10
	DeviceExtensionButtonHOTPUnsupportedMask       byte = 0x20
	DeviceExtensionOTPRequiresFingerprintMask      byte = 0x40
	DeviceExtensionMandatoryFingerprintSupportMask byte = 0x80
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

// Configuration describes the Token2 configuration response. Raw retains the
// complete response, including fields unknown to this version of the package.
// TransferType is an interface-state mask; DeviceConfiguration and
// DeviceExtension are capability and configuration masks.
type Configuration struct {
	Raw []byte `json:"raw"`

	TransferType        byte        `json:"transferType"`
	DeviceConfiguration byte        `json:"deviceConfiguration"`
	Appearance          Appearance  `json:"appearance"`
	FIDOVersion         FIDOVersion `json:"fidoVersion"`
	DeviceExtension     byte        `json:"deviceExtension"`
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
		TransferType: response[0],
	}
	if len(response) == 1 {
		return config, nil
	}

	config.DeviceConfiguration = response[1]
	config.Appearance = Appearance(response[2:6])
	config.FIDOVersion = FIDOVersion{response[6], response[7], response[8]}
	config.DeviceExtension = response[9]

	return config, nil
}
