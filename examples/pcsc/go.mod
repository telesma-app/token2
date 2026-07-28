module github.com/go-ctap/token2/examples/pcsc

go 1.26.3

require (
	github.com/go-ctap/pcsc v0.6.0
	github.com/go-ctap/token2 v0.5.0
)

require (
	github.com/ebitengine/purego v0.10.2 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

replace github.com/go-ctap/token2 => ../..
