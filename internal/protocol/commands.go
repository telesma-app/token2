// Package protocol defines the Token2 commands shared by transport adapters.
package protocol

import "github.com/go-ctap/token2/apdu"

const (
	classISO          = 0x00
	classToken2       = 0x80
	classToken2Reader = 0x90

	instructionSelect        = 0xa4
	instructionConfiguration = 0xc5
	instructionDeviceInfo    = 0x33
	instructionReaderVendor  = 0xff

	configurationRead  = 0x02
	readerMagicP1      = 0x55
	readerMagicP2      = 0x77
	readerSettingNFC   = 0x03
	readerSettingSound = 0x02

	// SerialResponseTag identifies the serial-number TLV returned by the device.
	SerialResponseTag = 0xd1

	serialRequestLength = 0x10
)

// SelectOTPCommand selects the Token2 OTP application used by configuration
// commands.
func SelectOTPCommand() apdu.Command {
	return apdu.Command{
		CLA:  classISO,
		INS:  instructionSelect,
		P1:   0x04,
		Data: []byte{0xf0, 0, 0, 1, 0x4f, 0x74, 0x70, 1},
	}
}

// SelectFIDOCommand selects the standard FIDO application used by the Token2
// serial-number command over PC/SC.
func SelectFIDOCommand() apdu.Command {
	return apdu.Command{
		CLA:  classISO,
		INS:  instructionSelect,
		P1:   0x04,
		Data: []byte{0xa0, 0, 0, 6, 0x47, 0x2f, 0, 1},
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

// ReaderSoundCommand configures the Token2 reader sound level.
func ReaderSoundCommand(level byte) apdu.Command {
	return readerSettingCommand(readerSettingSound, level)
}

// ReaderNFCCommand enables or disables the Token2 reader NFC interface.
func ReaderNFCCommand(enabled bool) apdu.Command {
	value := byte(0x01)
	if enabled {
		value = 0x00
	}

	return readerSettingCommand(readerSettingNFC, value)
}

func readerSettingCommand(setting, value byte) apdu.Command {
	return apdu.Command{
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
