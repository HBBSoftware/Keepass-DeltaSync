// SPDX-License-Identifier: GPL-3.0-or-later

// Package crypto implementerer klient-side entry-kryptering for
// keepass-deltasync. Serveren ser kun opaque blobs af formen
// nonce ‖ ciphertext, hvor ciphertext er XChaCha20-Poly1305 over en
// entrys plaintext-XML.
//
// Nøgle-afledning:
//
//	master_key  = Argon2id(password, salt=db_uuid_bytes, time=3, mem=64 MiB, par=1)  [32 bytes]
//	entry_key   = HKDF-SHA256(master_key, salt=db_uuid_bytes, info="deltasync-entry-v1")  [32 bytes]
//
// Begrundelse for nøgle-skemaet er dokumenteret i
// `keepass-deltasync-spec.md` § "Klient-side kryptering".
package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/hkdf"
)

const (
	// Argon2id-parametre. Matcher OWASP-anbefalingen for password-hashing
	// (64 MiB memory, 3 iterationer). Køres kun én gang pr. sync-cyklus —
	// ikke pr. entry — så ~200 ms latency er acceptabel UX.
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024 // KiB
	argonThreads uint8  = 1
	keyLen       uint32 = 32

	// hkdfInfo isolerer denne nøglebrug fra evt. fremtidige nøglebrug af samme
	// master_key. Hvis vi senere bruger master_key til, fx, fil-level encryption,
	// gives den en anden info-streng for at få en uafhængig sub-nøgle.
	hkdfInfo = "deltasync-entry-v1"
)

// DeriveMasterKey kører Argon2id over masterpassword med database-UUID'en som
// salt. Dette gør master-nøglen unik pr. database, så samme masterpassword på
// to forskellige databaser ikke giver samme nøglemateriale.
//
// dbUUID accepterer både formen med og uden bindestreger; ugyldige UUID'er
// returnerer fejl.
func DeriveMasterKey(password []byte, dbUUID string) ([]byte, error) {
	salt, err := parseUUIDBytes(dbUUID)
	if err != nil {
		return nil, fmt.Errorf("parse database uuid: %w", err)
	}
	if len(password) == 0 {
		return nil, errors.New("password must not be empty")
	}
	return argon2.IDKey(password, salt[:], argonTime, argonMemory, argonThreads, keyLen), nil
}

// DeriveEntryKey kører HKDF-SHA256 på master-nøglen med UUID som salt og en
// fast info-streng. Resultatet er den 32-byte nøgle der bruges direkte som
// XChaCha20-Poly1305-nøgle for alle entries i den givne database.
//
// Samme nøgle bruges for alle entries i databasen — sikkert pga. random
// 24-byte nonces (2^192 nonce-rum, ingen kollision-risiko ved tilfældige
// valg). Per-entry HKDF ville give samme sikkerhed med mere CPU-arbejde.
func DeriveEntryKey(masterKey []byte, dbUUID string) ([]byte, error) {
	if len(masterKey) != int(keyLen) {
		return nil, fmt.Errorf("master key must be %d bytes, got %d", keyLen, len(masterKey))
	}
	salt, err := parseUUIDBytes(dbUUID)
	if err != nil {
		return nil, fmt.Errorf("parse database uuid: %w", err)
	}
	r := hkdf.New(sha256.New, masterKey, salt[:], []byte(hkdfInfo))
	out := make([]byte, keyLen)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, fmt.Errorf("hkdf read: %w", err)
	}
	return out, nil
}

// parseUUIDBytes konverterer en UUID-streng (med eller uden bindestreger) til
// de 16 rå bytes. Ingen versionsvalidering — vi behandler UUID som opaque
// 128-bit identifier.
func parseUUIDBytes(s string) ([16]byte, error) {
	stripped := strings.ReplaceAll(s, "-", "")
	if len(stripped) != 32 {
		return [16]byte{}, fmt.Errorf("uuid must be 32 hex chars (with or without dashes), got %d", len(stripped))
	}
	var out [16]byte
	if _, err := hex.Decode(out[:], []byte(stripped)); err != nil {
		return [16]byte{}, fmt.Errorf("uuid hex decode: %w", err)
	}
	return out, nil
}
