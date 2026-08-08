module github.com/telesma-app/token2/examples/pcsc

go 1.26.3

require (
	github.com/telesma-app/pcsc v0.9.0
	github.com/telesma-app/token2 v0.11.0
)

require (
	github.com/ebitengine/purego v0.10.2 // indirect
	github.com/telesma-app/iso7816 v0.2.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

replace github.com/telesma-app/token2 => ../..
