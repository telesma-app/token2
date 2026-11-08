// Package piv implements TOKEN2 PIV extensions over a borrowed, selected PIV
// connection. Functions neither own the connection nor select another applet.
// A card.Device supplies operation serialization and administration workflows.
//
// TOKEN2's official token2-piv-tool uses the Yubico-compatible wire formats for
// version, metadata, attestation, management keys, import, key generation with
// policies, retry counts and reset. Those operations share the implementation
// in github.com/telesma-app/yubico/piv. Card support and access requirements still
// depend on its applet and policy. No default management key, automatic retry,
// firmware gate, or vendor fallback is applied.
//
// Generation, import and metadata support RSA-1024/2048/3072/4096,
// P-256/P-384/P-521, Ed25519 and X25519 using their shared wire encodings.
// The card decides which algorithms its firmware supports.
//
// TOKEN2 serial suffixes use packed BCD rather than Yubico's binary uint32.
// Attestation certificates may use TOKEN2's 1.3.6.1.4.1.66563.3 arc or the legacy
// 1.3.6.1.4.1.41482.3 arc; returning or parsing a certificate does not establish
// attestation trust. Callers supply their trust policy and roots.
//
// TOKEN2's separate EE/FE private-key deletion and minidriver cleanup workflow
// is not implemented. Yubico's F6 move/delete command is not exposed because
// TOKEN2 compatibility has not been established. Biometric binding extensions
// and cross-applet device configuration remain outside this package.
//
// Wire references:
// https://github.com/token2/fido2-manage/tree/main/token2-piv-tool
// https://github.com/token2/fido2-manage/blob/main/token2-piv-gui/src-tauri/src/piv.rs
package piv
