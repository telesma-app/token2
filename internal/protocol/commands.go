// Package protocol defines the Token2 commands shared by transport adapters.
package protocol

import "github.com/go-ctap/iso7816"

const (
	classISO          = 0x00
	classToken2       = 0x80
	classToken2Reader = 0x90

	instructionSelect        = 0xa4
	instructionConfiguration = 0xc5
	instructionDeviceInfo    = 0x33
	instructionReaderVendor  = 0xff

	configurationRead  = 0x02
	configurationCTAP  = 0x03
	ctapGetInfo        = 0x04
	readerMagicP1      = 0x55
	readerMagicP2      = 0x77
	readerSettingNFC   = 0x03
	readerSettingSound = 0x02

	// SerialResponseTag identifies the serial-number TLV returned by the device.
	SerialResponseTag = 0xd1

	serialRequestLength = 0x10
)

// SelectOTPCommand selects the Token2 OTP application used by configuration
// and initial serial-number commands.
func SelectOTPCommand() iso7816.Command {
	return iso7816.Command{
		CLA:  classISO,
		INS:  instructionSelect,
		P1:   0x04,
		Data: []byte{0xf0, 0, 0, 1, 0x4f, 0x74, 0x70, 1},
	}
}

// SelectFIDOCommand selects the standard FIDO application used by the Token2
// serial-number command over PC/SC.
func SelectFIDOCommand() iso7816.Command {
	return iso7816.Command{
		CLA:  classISO,
		INS:  instructionSelect,
		P1:   0x04,
		Data: []byte{0xa0, 0, 0, 6, 0x47, 0x2f, 0, 1},
	}
}

// ConfigurationCommand reads the Token2 device configuration.
func ConfigurationCommand() iso7816.Command {
	return iso7816.Command{
		CLA:  classToken2,
		INS:  instructionConfiguration,
		P1:   configurationRead,
		Data: make([]byte, 10),
	}
}

// LegacySerialNumberPreludeCommand primes the device-information command on
// firmware releases such as R3.1 which initially reject it with 6D00.
func LegacySerialNumberPreludeCommand() iso7816.Command {
	return iso7816.Command{
		CLA:  classToken2,
		INS:  instructionConfiguration,
		P1:   configurationCTAP,
		Data: []byte{ctapGetInfo},
	}
}

// SerialNumberCommand reads the full device serial number. HID uses extended
// APDU encoding while PC/SC uses short encoding.
func SerialNumberCommand(encoding iso7816.Encoding) iso7816.Command {
	request := make([]byte, 2+serialRequestLength)
	request[0] = SerialResponseTag
	request[1] = serialRequestLength

	return iso7816.Command{
		CLA:      classToken2,
		INS:      instructionDeviceInfo,
		Data:     request,
		Encoding: encoding,
	}
}

// ReaderSoundCommand configures the Token2 reader sound level.
func ReaderSoundCommand(level byte) iso7816.Command {
	return readerSettingCommand(readerSettingSound, level)
}

// ReaderNFCCommand enables or disables the Token2 reader NFC interface.
func ReaderNFCCommand(enabled bool) iso7816.Command {
	value := byte(0x01)
	if enabled {
		value = 0x00
	}

	return readerSettingCommand(readerSettingNFC, value)
}

func readerSettingCommand(setting, value byte) iso7816.Command {
	return iso7816.Command{
		CLA: classToken2Reader,
		INS: instructionReaderVendor,
		P1:  readerMagicP1,
		P2:  readerMagicP2,
		Data: []byte{
			classToken2, instructionDeviceInfo, 0x00, 0x00, 0x03,
			setting, 0x01, value,
		},
	}
}
