# Token2 hardware probe

Read-only collection of serial numbers, ATRs, configuration and CTAP GetInfo.
It does not create credentials, authenticate, set a PIN, reset a token, or
change token/reader settings. SELECT changes the active applet; the legacy
prelude sends GetInfo through the Token2 tunnel.

Build against the versions recorded in this repository's `go.mod`:

```sh
GOWORK=off go build -o /tmp/token2-probe ./cmd/token2-probe
/tmp/token2-probe -list
/tmp/token2-probe
```

Interactive input is a manual sample label followed by its connection:
`R3.1 usb`, `R3.1 nfc`, `R3.3-card nfc`. Enter `q` to finish. Keep one sample
connected when using a firmware-specific label. For a simultaneous inventory,
use a neutral label such as `usb-inventory`; every reading retains its endpoint.

The default PC/SC filter is `TOKEN2 FIDO2 Security Key` for USB and
`Token2 Smart Reader` for NFC. USB additionally probes HID interfaces with VID
`349e`. NFC never probes HID. Use `-reader` for a different reader name. The
inventory records all PC/SC readers, including empty and excluded readers,
but only matching readers with cards receive commands.

```sh
/tmp/token2-probe -label R3.3-card -connection nfc -reader 'Token2 Smart Reader'
/tmp/token2-probe -label usb-inventory -connection usb
```

For one USB sample among several attached tokens, use names and paths from
`-list`. Reader names and HID paths can change after reconnecting:

```sh
/tmp/token2-probe -label R3.1-sample-B -connection usb \
  -reader 'TOKEN2 FIDO2 Security Key(0024)' -reader-exact \
  -hid-path 100026f0f
```

Each capture creates a new JSONL file in the `token2-probe` subdirectory of the
OS temporary directory (`-out` overrides it). Each interface is read three times
(`-repeat`). Records include
UTC timestamps, manual label, transport, endpoint, repetition, complete raw
APDUs/HID reports, status words, transport errors and parser results. GetInfo
also retains each CBOR field's encoded bytes, preserving absent versus zero
values. Model identification is a separate serial-catalogue lookup. The library's
`Descriptor.Err` is excluded from JSON; `pcsc.library.warm` explicitly records
per-reading errors in `readErrors` alongside the aggregate `error`.

PC/SC measurement order:

1. Begin transaction and read the card status/ATR.
2. Select OTP; read serial and configuration before the prelude.
3. Send legacy GetInfo; read serial and configuration again.
4. Select FIDO; read serial and standard GetInfo.
5. Call the current library's `ReadDeviceInfo` on the same card as a warm read.
6. End transaction with `LeaveCard` and close the connection.

An unsupported command does not suppress other independent measurements.
Commands requiring a selected applet are skipped if that SELECT fails.
Feature HID reads the serial through extended APDUs; CTAPHID records INIT,
vendor ATR and GetInfo separately. Each operation has a five-second context.
Each interface/repetition runs in a child process with a 45-second hard deadline
(`-timeout`), because native HID calls may not honor context cancellation.
Partial raw evidence remains in the file if a child times out.

PC/SC is measured before HID to avoid priming a legacy applet with CTAPHID
GetInfo first. Repetitions reopen connections but do not power-cycle hardware;
they are not independent cold-start tests. To examine power-on behavior,
disconnect/reconnect the sample and start a new capture.

APDU errors, unavailable interfaces and parsing errors are observations, not
automatic firmware verdicts. A generic ATR can be valid without Token2 identity.
An OS permission denial is distinct from an unsupported device command.

On macOS the process needs access to PC/SC and HID; a sandbox can report
`SCardEstablishContext: no service` even when readers work outside it. The Bio3
keyboard collection may require Input Monitoring access; serial reads on its
FIDO HID collection can succeed independently.
