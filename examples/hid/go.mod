module github.com/go-ctap/token2/examples/hid

go 1.26.3

require (
	github.com/go-ctap/hid v0.10.1
	github.com/go-ctap/token2 v0.5.0
)

require (
	github.com/ebitengine/purego v0.10.2 // indirect
	github.com/go-ctap/iso7816 v0.1.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
)

replace github.com/go-ctap/token2 => ../..
