// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

// NonceSize er størrelsen på XChaCha20-Poly1305-nonce'en i bytes.
// 24 byte (192 bit) tillader trygge tilfældige nonces uden et tæller.
const NonceSize = chacha20poly1305.NonceSizeX

// EncryptBlob krypterer plaintext med XChaCha20-Poly1305 under en frisk
// random nonce og returnerer wire-formatet:
//
//	blob = nonce ‖ ciphertext (inkl. Poly1305-tag)
//
// Key skal være præcis 32 bytes (typisk output fra DeriveEntryKey).
func EncryptBlob(key, plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("new xchacha20poly1305: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("read random nonce: %w", err)
	}

	// Append ciphertext (med tag) til nonce-prefixet. Det giver os
	// nonce ‖ ct ‖ tag uden en separat allokering.
	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptBlob inverterer EncryptBlob: trækker de første 24 bytes ud som nonce,
// dekrypterer resten med XChaCha20-Poly1305, og returnerer plaintext.
// Returnerer fejl hvis blob'en er for kort eller Poly1305-tag'en ikke matcher
// (forkert nøgle ELLER tampered ciphertext).
func DecryptBlob(key, blob []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("new xchacha20poly1305: %w", err)
	}
	if len(blob) < aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("blob too short to contain nonce + tag")
	}

	nonce, ct := blob[:aead.NonceSize()], blob[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}
