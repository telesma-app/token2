# go-ctap/token2

[![Go Reference](https://pkg.go.dev/badge/github.com/go-ctap/token2.svg)](https://pkg.go.dev/github.com/go-ctap/token2)
[![Go](https://github.com/go-ctap/token2/actions/workflows/go.yml/badge.svg)](https://github.com/go-ctap/token2/actions/workflows/go.yml)

Pure-Go Token2 device support over PC/SC, USB HID feature reports and CTAPHID.

> [!WARNING]
> This module is under active development. Its public API may change during `v0.x`.

## Packages

- `token2` contains the transport-independent `DeviceInfo`, `ATR` and
  `Configuration` types, response parsers and model identification.
- `token2/transport/pcsc` opens Token2 devices through
  [`github.com/go-ctap/pcsc`](https://github.com/go-ctap/pcsc).
- `token2/transport/hid` opens the Token2 HID interface through
  [`github.com/go-ctap/hid`](https://github.com/go-ctap/hid).
- `token2/transport/ctaphid` reads the Token2 ATR through CTAPHID vendor
  command `0x41`, using [`github.com/go-ctap/ctap`](https://github.com/go-ctap/ctap).
- `token2/resolver` correlates a FIDO attachment with Token2's PC/SC and
  feature-HID interfaces without guessing between multiple devices.

## Transport capabilities

| Capability                                  | PC/SC                | Feature HID | CTAPHID |
|---------------------------------------------|----------------------|-------------|---------|
| Full serial number                          | Yes                  | Yes         | No      |
| Model identification from the serial number | Yes                  | Yes         | No      |
| ATR-derived product ID and serial suffix    | Generation-dependent | No          | Yes     |
| Token2 configuration                        | Generation-dependent | No          | No      |

The HID and PC/SC transports return `token2.DeviceInfo`. The resolver enriches
the same flat type with the ATR-derived product ID, appearance, FIDO version,
interface state and all known configuration capabilities. Raw ATR bytes,
configuration masks and the ATR serial suffix remain available through the
separate low-level `ATR` and `Configuration` APIs; they are not nested in
`DeviceInfo`. CTAPHID returns `token2.ATR` because that command does not reveal
the full serial number or model.

The PC/SC serial-number query selects the Token2 OTP application and first reads
the serial number there. If the OTP application reports that the instruction is
unsupported, the query switches to the standard FIDO application and retries,
matching both R3.2 USB CCID and the official R3.1 NFC tooling. Configuration
queries are not available on every Token2 generation. In particular, R3.3
cards reject the configuration command and may expose a generic PIV ATR without
the Token2 product ID and serial suffix; `ATR` then returns `ErrInvalidATR`.
Serial-number retrieval is independent of the optional OTP configuration
command and ATR parsing. USB identity resolution can use Token2 feature reports
on the matched FIDO HID interface itself, as well as on a separate auxiliary HID
interface when the device exposes one.

### Token2 reader settings

Compatible Token2 readers expose sound-level and NFC controls through PC/SC:

```go
if err := device.SetReaderSoundLevel(ctx, token2pcsc.ReaderSoundLevel3); err != nil {
	return err
}
if err := device.SetReaderNFC(ctx, false); err != nil {
	return err
}
```

`ReaderSoundOff` disables sound; the other sound levels range from 1 through 5.
The reader must be physically power-cycled after changing either setting. A
successful `9000` response means only that the command was received: an
unsupported reader may return success and ignore it.

All concrete device types serialize complete logical operations. Malformed data
received from a card or HID device is returned as an error. Callers are expected
to pass valid reader names, HID paths and serial-number strings; the package
does not add defensive checks for programmer misuse.

## Examples

Complete runnable usage is available in the transport examples. Each example is
an independent Go module, keeping transport-specific dependencies out of the
root module.

| Example                                                        | Purpose                                              | Optional configuration                |
|----------------------------------------------------------------|------------------------------------------------------|---------------------------------------|
| [`examples/pcsc`](examples/pcsc)                               | Read identity and configuration over PC/SC           | `PCSC_READER` (reader-name substring) |
| [`examples/pcsc-reader-settings`](examples/pcsc-reader-settings) | Configure Token2 reader sound and NFC over PC/SC      | `-reader` (reader-name substring)     |
| [`examples/hid`](examples/hid)                                 | Read the full serial number over HID feature reports | `TOKEN2_HID_PATH`                     |
| [`examples/ctaphid`](examples/ctaphid)                         | Read ATR identity over the CTAPHID vendor command    | `TOKEN2_CTAPHID_PATH`                 |

The CTAPHID transport sends logical vendor command `0x41`; CTAPHID framing adds
the init-packet bit, so the on-wire command byte is `0xc1`.

Run an example from its directory:

```sh
cd examples/pcsc
go run .
```

Without an environment variable, the read-only examples select the first
matching device or reader they find. Set the corresponding variable when
multiple Token2 devices are connected or when automatic HID selection is not
available on the host.

The reader-settings example requires at least one setting and never changes a
reader merely by being run. For example:

```sh
cd examples/pcsc-reader-settings
go run . -reader 'TOKEN2' -sound 3 -nfc off
```

When `-reader` is omitted, the example prefers `Token2 Smart Reader` and then
falls back to the first PC/SC reader with a card present. An explicit `-reader`
filter never falls back to a different reader.

## Hardware tests

Hardware tests are opt-in:

```sh
TOKEN2_PCSC_TEST_READER='TOKEN2 FIDO2 Security Key(0016)' go test -run TestHardware -v ./transport/pcsc
TOKEN2_HID_TEST_PATH='platform-specific-path' go test -run TestHardware -v ./transport/hid
TOKEN2_CTAPHID_TEST_PATH='platform-specific-path' go test -run TestHardware -v ./transport/ctaphid
```
