module github.com/telesma-app/token2/examples/hid

go 1.26.3

require (
	github.com/telesma-app/hid v0.12.0
	github.com/telesma-app/token2 v0.11.0
)

require (
	github.com/ebitengine/purego v0.10.2 // indirect
	github.com/telesma-app/iso7816 v0.2.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
)

replace github.com/telesma-app/token2 => ../..
