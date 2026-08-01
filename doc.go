// Package token2 provides transport-independent Token2 device information,
// response parsers and model identification.
//
// Concrete device implementations live under transport. HID and PC/SC devices
// return DeviceInfo; CTAPHID devices return the partial identity encoded in ATR.
package token2
