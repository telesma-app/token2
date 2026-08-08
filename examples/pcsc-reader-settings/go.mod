module github.com/telesma-app/token2/examples/pcsc-reader-settings

go 1.26.3

require (
	github.com/stretchr/testify v1.11.1
	github.com/telesma-app/pcsc v0.9.0
	github.com/telesma-app/token2 v0.11.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/ebitengine/purego v0.10.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/telesma-app/iso7816 v0.2.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/telesma-app/token2 => ../..
