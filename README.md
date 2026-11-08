# telesma-app/token2

[![Go Reference](https://pkg.go.dev/badge/github.com/telesma-app/token2.svg)](https://pkg.go.dev/github.com/telesma-app/token2)
[![Go](https://github.com/telesma-app/token2/actions/workflows/go.yml/badge.svg)](https://github.com/telesma-app/token2/actions/workflows/go.yml)

Go library for Token2 device information, reader settings and PIV extensions
over PC/SC, USB HID feature reports and CTAPHID.

> [!WARNING]
> This module is under active development. Its public API may change during `v0.x`.

## Features

- Device serial numbers, ATR identity and configuration through the available
  PC/SC and HID interfaces.
- Model identification from reported serial numbers, with parsers for device
  configuration, enabled interfaces and capabilities.
- Reader sound-level and NFC settings on compatible Token2 readers.
- PIV version, slot metadata and attestation certificates.
- PIV key import, generation, management-key changes and administrative commands.

Available information and commands depend on the device, connection and firmware.
USB and NFC interfaces may expose different information for the same token.
PIV extensions use an existing connection from [`telesma-app/piv`](https://github.com/telesma-app/piv).

## Installation

```sh
go get github.com/telesma-app/token2@latest
```

See [`go.mod`](go.mod) for the required Go version.

## Quick start

Open a device by its exact PC/SC reader name and print its serial number and
interface configuration. Replace the reader name with one from your system:

```go
package main

import (
	"context"
	"fmt"
	"log"

	token2pcsc "github.com/telesma-app/token2/transport/pcsc"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	device, err := token2pcsc.Open("exact PC/SC reader name")
	if err != nil {
		return err
	}
	defer device.Close()

	ctx := context.Background()
	serial, err := device.SerialNumber(ctx)
	if err != nil {
		return err
	}

	config, err := device.Configuration(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Serial: %s\nInterfaces: %+v\n", serial, config.InterfaceState())
	return nil
}
```

This example only reads information. PC/SC access requires a running native
smart-card service and a connected token with its CCID interface enabled.
Close devices when finished; the PC/SC connection leaves the card powered by default.

## Packages

| Package | Purpose |
| --- | --- |
| [`token2`](https://pkg.go.dev/github.com/telesma-app/token2) | Device data, response parsers and model identification |
| [`transport/pcsc`](transport/pcsc) | Device information and reader settings over PC/SC |
| [`transport/hid`](transport/hid) | Serial numbers through USB HID feature reports |
| [`transport/ctaphid`](transport/ctaphid) | ATR identity through a FIDO HID connection |
| [`piv`](piv) | Token2 PIV extensions |

## Examples

Runnable examples cover [PC/SC information](examples/pcsc),
[HID serial numbers](examples/hid), [CTAPHID identity](examples/ctaphid) and
[reader settings](examples/pcsc-reader-settings). Each example is a separate Go module.

```sh
cd examples/pcsc
go run .
```

It uses the first matching reader; set `PCSC_READER` to a reader-name substring
to select one. The reader-settings example requires explicit settings and changes
the reader configuration; the information examples only read data.

## Development

```sh
go test ./...
make lint
```

Normal tests do not require hardware. The lint target checks the root module
and each example with golangci-lint v2.

## License

[Apache License 2.0](LICENSE).

## References

- [Token2 Companion App: device information and configuration](https://www.token2.com/site/page/token2-companion-app-v2-user-manual)
- [Token2 Dual Reader and sound configuration tool](https://www.token2.com/products/nfc-contact-dual-desktop-reader)
- [Token2 PIV vendor-command implementation](https://github.com/token2/fido2-manage/blob/main/token2-piv-tool/lib/t2piv.c)
