# telesma-app/token2

[![Go Reference](https://pkg.go.dev/badge/github.com/telesma-app/token2.svg)](https://pkg.go.dev/github.com/telesma-app/token2)
[![Go](https://github.com/telesma-app/token2/actions/workflows/go.yml/badge.svg)](https://github.com/telesma-app/token2/actions/workflows/go.yml)

Pure-Go Token2 device support over PC/SC, USB HID feature reports and CTAPHID.

> [!WARNING]
> This module is under active development. Its public API may change during `v0.x`.

## Packages

- `token2` contains response parsers, device-information types and model identification.
- `token2/transport/pcsc` opens Token2 devices through
  [`github.com/telesma-app/pcsc`](https://github.com/telesma-app/pcsc).
- `token2/transport/hid` opens the Token2 HID interface through
  [`github.com/telesma-app/hid`](https://github.com/telesma-app/hid).
- `token2/transport/ctaphid` reads the Token2 ATR through CTAPHID vendor
  command `0x41`, using [`github.com/telesma-app/ctap`](https://github.com/telesma-app/ctap).
- Interface correlation and physical-device selection belong to the consuming runtime.

## Data available by release and connection

For the verified devices below, **✓** means the library reads the data through
at least one method on that connection. **—** means the current readers do not
provide that field.

| Release / sample | Connection | Full serial | Raw ATR | PID + suffix from ATR | Token2 configuration |
|---|---|:---:|:---:|:---:|:---:|
| R3.1 USB-A NFC | USB | ✓ | ✓ | ✓ | ✓ |
| R3.1 USB-A NFC | NFC | ✓ | ✓ | — | — |
| R3.2 Mini USB-A / Bio3 | USB | ✓ | ✓ | ✓ | ✓ |
| R3.3 Dual | USB | ✓ | ✓ | ✓¹ | ✓ |
| R3.3 Dual | NFC | ✓ | ✓ | — | — |
| R3.3 card | NFC | ✓ | ✓ | — | — |

USB results combine Feature HID, CTAPHID and PC/SC reads.
¹ R3.3 Dual supplies PID and serial suffix through CTAPHID; its PC/SC ATR
contains no Token2 identity.

On the listed NFC devices, the ATR is valid without Token2 identity, and the
configuration command returns `6A86`. Serial and ATR remain available.
Untested models and connections, including R3.4, are omitted.

Model names can be derived from the full serial using `Identify`.

### Device information API

`DeviceInfo` contains independent `Descriptor[T]` observations from one interface,
using the same `Value *T` and `Err error` pattern as the HID package. A non-nil
value preserves a completed read, including zero values and empty strings.
The original error remains available through `errors.Is`/`errors.As`; a value
and an error may coexist. JSON contains `value` (null when absent) and excludes
`Err`. Applications serialize errors as needed.

Use `hid.ReadDeviceInfo(ctx, path, vendor)` for a specific HID interface and its
optional borrowed CTAPHID channel. Use `pcsc.Device.DeviceInfo(ctx)` for one
managed transaction, or `pcsc.ReadDeviceInfo(ctx, card)` inside a caller-owned
transaction. Readers preserve independent serial, ATR and configuration results.

`ParseATR` walks the ATR interface-byte groups and validates its length/checksum.
A valid ATR without Token2 historical bytes returns `ATR{Raw: ..., Identity: nil}`.
Malformed ATR data returns `ErrInvalidATR`.

`Identify(serial)` looks up a model without changing the reported data.
`Configuration.InterfaceState()` and `Capabilities()` decode configuration flags.

For an existing PC/SC connection, use `ReadSerialNumber` and `ReadConfiguration`.
The caller owns the transaction and must reselect the applet needed by its next
operation.

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

Read-only examples select the first matching device unless the corresponding
environment variable is set.

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

To read full serial numbers from several PC/SC tokens one after another, run the event monitor and insert or present each token:

```sh
go run ./cmd/pcsc-serial-monitor
```

For labelled USB/NFC captures with complete raw replies, repeated reads and
independent error recording, use [`cmd/token2-probe`](cmd/token2-probe):

```sh
GOWORK=off go build -o /tmp/token2-probe ./cmd/token2-probe
/tmp/token2-probe
```
