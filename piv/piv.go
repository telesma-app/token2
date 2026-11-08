package piv

import (
	"context"
	"crypto"
	"crypto/x509"

	"github.com/telesma-app/piv/card"
	"github.com/telesma-app/piv/protocol"
	yubicopiv "github.com/telesma-app/yubico/piv"
)

// ReadVersion reads the selected TOKEN2 PIV applet's three-byte version.
func ReadVersion(ctx context.Context, transport VendorTransport) (FirmwareVersion, error) {
	return yubicopiv.ReadVersion(ctx, transport)
}

// ReadMetadata reads metadata for the selected key or authentication reference.
// Its presence does not establish certificate trust or prove key possession.
func ReadMetadata(ctx context.Context, transport VendorTransport, reference protocol.KeyReference) (Metadata, error) {
	return yubicopiv.ReadMetadata(ctx, transport, reference)
}

// AttestDER requests a slot's attestation certificate as DER. It does not verify
// the certificate or establish trust in its signer.
func AttestDER(ctx context.Context, transport VendorTransport, reference protocol.KeyReference) ([]byte, error) {
	return yubicopiv.AttestDER(ctx, transport, reference)
}

// Attest requests and parses a slot's attestation certificate. Trust validation
// and interpretation of TOKEN2-specific certificate extensions belong to the
// caller.
func Attest(ctx context.Context, transport VendorTransport, reference protocol.KeyReference) (*x509.Certificate, error) {
	return yubicopiv.Attest(ctx, transport, reference)
}

// SetManagementKey authenticates with current and replaces the management key
// (called the Admin PIN by TOKEN2 tools). Touch is encoded explicitly as Never,
// Always, or Cached; the card decides which policies and key changes it supports.
// Keys are borrowed and are neither retained nor cleared. An interrupted
// exchange may have changed the key; the function never retries.
func SetManagementKey(
	ctx context.Context,
	transport AdministrationTransport,
	replacement card.AdministrationKey,
	touch TouchPolicy,
	current card.AdministrationKey,
) error {
	return yubicopiv.SetManagementKey(ctx, transport, replacement, touch, current)
}

// SetPINRetries authenticates management, optionally verifies authentication,
// changes both retry limits and resets PIN and PUK to their factory values.
// A nil authentication relies on established PIN status. The card determines
// permitted limits. No PIN, PUK, or management key is tried automatically.
func SetPINRetries(
	ctx context.Context,
	transport AdministrationTransport,
	pinRetries, pukRetries byte,
	administrationKey card.AdministrationKey,
	authentication card.AuthenticationSource,
) error {
	return yubicopiv.SetPINRetries(ctx, transport, pinRetries, pukRetries, administrationKey, authentication)
}

// Reset erases PIV data and restores the applet's factory state. PIN and PUK
// must already be blocked; this function only sends RESET and never consumes
// retries to prepare the card. The connection remains caller-owned.
func Reset(ctx context.Context, transport VendorTransport) error {
	return yubicopiv.Reset(ctx, transport)
}

// ImportKey authenticates management and replaces the slot's private key,
// preserving its certificate. Supported inputs are two-prime RSA-1024/2048/3072/4096
// with exponent 65537, ECDSA P-256/P-384/P-521, Ed25519 and X25519 keys. The private
// key is borrowed without modification. Policies are encoded unchanged; firmware
// determines support.
// An interrupted exchange may have replaced the key. The function never retries.
func ImportKey(
	ctx context.Context,
	transport AdministrationTransport,
	reference protocol.KeyReference,
	privateKey crypto.PrivateKey,
	policy KeyPolicy,
	administrationKey card.AdministrationKey,
) error {
	return yubicopiv.ImportKey(ctx, transport, reference, privateKey, policy, administrationKey)
}

// GenerateKey authenticates management and replaces a slot's private key with
// explicit optional PIN/touch policies, returning the generated public key.
// Supported algorithms are RSA-1024/2048/3072/4096, P-256/P-384/P-521, Ed25519 and
// X25519. Certificates are unchanged. A client.KeyGenerationError means replacement was acknowledged
// but the public-key response was invalid. Interrupted exchanges may also have
// replaced the key. The function never retries.
func GenerateKey(
	ctx context.Context,
	transport AdministrationTransport,
	request GenerateKeyRequest,
	administrationKey card.AdministrationKey,
) (crypto.PublicKey, error) {
	return yubicopiv.GenerateKey(ctx, transport, request, administrationKey)
}
