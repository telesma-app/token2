// Package protocol defines the Token2 commands shared by transport adapters.
package protocol

import "github.com/go-ctap/token2/apdu"

const (
	classISO    = 0x00
	classToken2 = 0x80

	instructionSelect        = 0xa4
	instructionConfiguration = 0xc5
	instructionDeviceInfo    = 0x33

	configurationRead = 0x02
	configurationCTAP = 0x03
	ctapGetInfo       = 0x04

	// SerialResponseTag identifies the serial-number TLV returned by the device.
	SerialResponseTag = 0xd1

	serialRequestLength = 0x10
)

// SelectOTPCommand selects the Token2 OTP application used by configuration
// and device-information commands.
func SelectOTPCommand() apdu.Command {
	return apdu.Command{
		CLA:  classISO,
		INS:  instructionSelect,
		P1:   0x04,
		Data: []byte{0xf0, 0, 0, 1, 0x4f, 0x74, 0x70, 1},
	}
}

// ConfigCommand reads the Token2 device configuration.
func ConfigCommand() apdu.Command {
	return apdu.Command{
		CLA:  classToken2,
		INS:  instructionConfiguration,
		P1:   configurationRead,
		Data: make([]byte, 10),
	}
}

// LegacySerialNumberPreludeCommand primes the device-information command on
// firmware releases such as R3.1 which initially reject it with 6D00.
func LegacySerialNumberPreludeCommand() apdu.Command {
	return apdu.Command{
		CLA:  classToken2,
		INS:  instructionConfiguration,
		P1:   configurationCTAP,
		Data: []byte{ctapGetInfo},
	}
}

// SerialNumberCommand reads the full device serial number. HID uses extended
// APDU encoding while PC/SC uses short encoding.
func SerialNumberCommand(extended bool) apdu.Command {
	request := make([]byte, 2+serialRequestLength)
	request[0] = SerialResponseTag
	request[1] = serialRequestLength

	return apdu.Command{
		CLA:      classToken2,
		INS:      instructionDeviceInfo,
		Data:     request,
		Extended: extended,
	}
}
