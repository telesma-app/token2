module github.com/telesma-app/token2/examples/pcsc-reader-settings

go 1.27.3

require (
	github.com/telesma-app/pcsc v0.10.0
	github.com/telesma-app/token2 v0.15.0
)

require (
	github.com/ebitengine/purego v0.11.3 // indirect
	github.com/telesma-app/iso7816 v0.7.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace github.com/telesma-app/token2 => ../..
