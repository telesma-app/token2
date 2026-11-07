package piv

import (
	"context"

	"github.com/telesma-app/iso7816"
	"github.com/telesma-app/piv/card"
	"github.com/telesma-app/piv/protocol"
	"github.com/telesma-app/yubico"
	yubicopiv "github.com/telesma-app/yubico/piv"
)

// VendorTransport exchanges a complete command on the selected PIV application.
// The caller owns the connection and its workflow serialization.
type VendorTransport interface {
	Exchange(context.Context, iso7816.Command) (iso7816.Response, error)
}

// AdministrationTransport authenticates management and exchanges the command
// under one operation gate, optionally verifying the supplied authentication
// source. A card.Device implements this contract.
type AdministrationTransport interface {
	ExchangeAdministration(context.Context, iso7816.Command, card.AdministrationKey, card.AuthenticationSource) (iso7816.Response, error)
}

// FirmwareVersion is the three-byte version reported by the PIV applet. It is
// independent of the device's TOKEN2 release number, such as R3.3 or R3.4.
type FirmwareVersion = yubico.FirmwareVersion

const (
	// AttestationKey identifies the TOKEN2 attestation signer key.
	AttestationKey protocol.KeyReference = 0xf9
	// AttestationCertificate is the attestation signer's certificate object.
	AttestationCertificate protocol.Object = 0x5fff01
)

// PINPolicy is the PIN-policy byte shared by TOKEN2 and Yubico PIV commands.
type PINPolicy = yubicopiv.PINPolicy

const (
	PINPolicyDefault = yubicopiv.PINPolicyDefault
	PINPolicyNever   = yubicopiv.PINPolicyNever
	PINPolicyOnce    = yubicopiv.PINPolicyOnce
	PINPolicyAlways  = yubicopiv.PINPolicyAlways
)

// TouchPolicy is the touch-policy byte shared by TOKEN2 and Yubico PIV commands.
type TouchPolicy = yubicopiv.TouchPolicy

const (
	TouchPolicyDefault = yubicopiv.TouchPolicyDefault
	TouchPolicyNever   = yubicopiv.TouchPolicyNever
	TouchPolicyAlways  = yubicopiv.TouchPolicyAlways
	TouchPolicyCached  = yubicopiv.TouchPolicyCached
)

// KeyPolicy contains the PIN and touch policies encoded with key import or
// generation. Zero values omit their policy tags; the card chooses its defaults.
type KeyPolicy = yubicopiv.KeyPolicy

// GenerateKeyRequest is the shared key-generation template with optional
// explicit PIN and touch policies.
type GenerateKeyRequest = yubicopiv.GenerateKeyRequest

// KeyOrigin is the provenance reported by the card, without verification.
type KeyOrigin = yubicopiv.KeyOrigin

const (
	KeyOriginGenerated = yubicopiv.KeyOriginGenerated
	KeyOriginImported  = yubicopiv.KeyOriginImported
)

// RetryCounts reports configured and remaining attempts when present in metadata.
type RetryCounts = yubicopiv.RetryCounts

// Metadata contains the shared PIV metadata TLVs. Card support and field
// presence depend on the requested reference.
type Metadata = yubicopiv.Metadata
