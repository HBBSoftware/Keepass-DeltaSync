// SPDX-License-Identifier: GPL-3.0-or-later

// Package mobile er det gomobile-bind-vendte API der eksponeres til
// Android-app'en. Den indkapsler crypto- og canonical-pakkerne i et minimalt
// sæt funktioner med gomobile-kompatible signaturer — kun string, []byte,
// int og pointers-til-egne-typer.
//
// Pakken er bevidst tynd: HTTP, kdbx-håndtering (via kotpass), persistens
// og UI lever 100% i Kotlin-laget på Android-siden. Vi forsyner kun det
// indlejret-spec-afhængige: nøglederivation, blob-encrypt/decrypt, og
// canonical-format-konvertering, så desktop og Android producerer
// byte-identiske blobs på wiren.
//
// gomobile bind kommando (kræver gomobile installeret + Android NDK):
//
//	gomobile bind -target=android -o deltasync.aar \
//	    gitlab.com/Star95/keepass-deltasync/client/mobile
//
// Resultat er en .aar fil + en .sources.jar med Kotlin-stubs. Android-app'en
// importerer .aar'en og kalder fx Mobile.NewSession(password, dbId).
package mobile

import (
	"encoding/json"
	"errors"
	"fmt"

	"gitlab.com/Star95/keepass-deltasync/client/internal/crypto"
	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx/canonical"
)

// Session repræsenterer en åbnet database fra klient-side. Den ejer den
// deriverede entry-encryption-key og bruges til at kryptere/dekryptere
// blobs på tværs af mange entry-operationer uden at re-derivere nøglen
// (Argon2id koster ~200ms; vi vil ikke betale det pr. entry).
//
// En Session er IKKE thread-safe. Android-koden skal serialisere kald
// fra én tråd (typisk en sync-coroutine).
type Session struct {
	entryKey []byte
}

// NewSession deriverer entry-nøglen fra et masterpassword + databasens
// remote ID (server-side UUID). Bruges af owners — for shared databases
// hvor master_key er wrapped, brug NewSessionFromMasterKey i stedet.
//
// Caller'en skal zero'e password-bytes efter kaldet returnerer.
func NewSession(password []byte, databaseID string) (*Session, error) {
	if len(password) == 0 {
		return nil, errors.New("empty password")
	}
	if databaseID == "" {
		return nil, errors.New("empty databaseID")
	}
	masterKey, err := crypto.DeriveMasterKey(password, databaseID)
	if err != nil {
		return nil, fmt.Errorf("derive master key: %w", err)
	}
	defer zero(masterKey)
	entryKey, err := crypto.DeriveEntryKey(masterKey, databaseID)
	if err != nil {
		return nil, fmt.Errorf("derive entry key: %w", err)
	}
	return &Session{entryKey: entryKey}, nil
}

// NewSessionFromMasterKey opretter en Session direkte fra et allerede-
// kendt master_key — bruges når Bob har unwrapped en wrapped_master_key
// fra en delt database (rolle = member).
//
// Parametre-orden er bevidst (string, []byte) i stedet for ([]byte, string)
// for at undgå at gomobile-bind genererer to Session-constructors med
// identisk Java-signatur (kollision med NewSession).
func NewSessionFromMasterKey(databaseID string, masterKey []byte) (*Session, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes, got %d", len(masterKey))
	}
	if databaseID == "" {
		return nil, errors.New("empty databaseID")
	}
	entryKey, err := crypto.DeriveEntryKey(masterKey, databaseID)
	if err != nil {
		return nil, fmt.Errorf("derive entry key: %w", err)
	}
	return &Session{entryKey: entryKey}, nil
}

// Close zeroer entry-nøglen. Skal kaldes når brugeren lukker app'en eller
// låser databasen for at minimere window'et hvor nøglen ligger i hukommelsen.
// Yderligere kald til EncryptEntry/DecryptEntry efter Close vil fejle.
func (s *Session) Close() {
	zero(s.entryKey)
	s.entryKey = nil
}

// EncryptEntry tager JSON-repræsentationen af en canonical.Entry og
// returnerer det krypterede wire-blob klar til upload som server-side entry-
// version. Indkapsler hele push-path'en: parse JSON → canonical.Entry →
// EncodeCanonical (format-byte prefix) → EncryptBlob.
//
// entryJSON skal være output fra Kotlin's serializer der matcher
// canonical.Entry's JSON-schema (se docs/v3-canonical-entry-format.md).
func (s *Session) EncryptEntry(entryJSON []byte) ([]byte, error) {
	if s.entryKey == nil {
		return nil, errors.New("session closed")
	}
	if len(entryJSON) == 0 {
		return nil, errors.New("empty entryJSON")
	}

	var entry canonical.Entry
	if err := json.Unmarshal(entryJSON, &entry); err != nil {
		return nil, fmt.Errorf("parse entry json: %w", err)
	}

	plaintext, err := canonical.EncodeCanonical(&entry)
	if err != nil {
		return nil, fmt.Errorf("encode canonical: %w", err)
	}

	blob, err := crypto.EncryptBlob(s.entryKey, plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt blob: %w", err)
	}
	return blob, nil
}

// DecryptEntry tager et server-blob (downloadet via GET /changes) og
// returnerer entry'en som canonical JSON. Auto-detekterer mellem legacy
// XML-fragmenter (v1, byte 0 == '<') og canonical (v3, byte 0 == 0x01),
// så Android-stien virker mod både gamle og nye server-blobs.
//
// Kalder'en kan derefter parse JSON i Kotlin og applicere mod kotpass-
// Entry'en med last-writer-wins på `times.modified`.
func (s *Session) DecryptEntry(blob []byte) ([]byte, error) {
	if s.entryKey == nil {
		return nil, errors.New("session closed")
	}

	plaintext, err := crypto.DecryptBlob(s.entryKey, blob)
	if err != nil {
		return nil, fmt.Errorf("decrypt blob: %w", err)
	}

	switch canonical.DetectFormat(plaintext) {
	case canonical.FormatCanonical:
		entry, err := canonical.DecodeCanonical(plaintext)
		if err != nil {
			return nil, fmt.Errorf("decode canonical: %w", err)
		}
		return json.Marshal(entry)

	case canonical.FormatLegacyXML:
		entry, err := canonical.FromInnerXML(plaintext)
		if err != nil {
			return nil, fmt.Errorf("parse legacy fragment: %w", err)
		}
		// Tag højde for at legacy-fragmenter ikke har vores version-felt
		// sat — EncodeCanonical (på næste push fra denne enhed) vil
		// normalisere det automatisk.
		entry.V = canonical.SchemaVersion
		return json.Marshal(entry)

	default:
		if len(plaintext) == 0 {
			return nil, errors.New("empty plaintext after decrypt")
		}
		return nil, fmt.Errorf("unrecognized blob format byte 0x%02x", plaintext[0])
	}
}

// EncryptGroup tager JSON-repræsentationen af en canonical.Group og returnerer
// det krypterede wire-blob klar til upload som server-side gruppe-version
// (object_kind=2). Modstykket til EncryptEntry, men med gruppe-envelope-byten
// (0x02) via EncodeGroup. v4 group-sync.
func (s *Session) EncryptGroup(groupJSON []byte) ([]byte, error) {
	if s.entryKey == nil {
		return nil, errors.New("session closed")
	}
	if len(groupJSON) == 0 {
		return nil, errors.New("empty groupJSON")
	}

	var group canonical.Group
	if err := json.Unmarshal(groupJSON, &group); err != nil {
		return nil, fmt.Errorf("parse group json: %w", err)
	}

	plaintext, err := canonical.EncodeGroup(&group)
	if err != nil {
		return nil, fmt.Errorf("encode group: %w", err)
	}

	blob, err := crypto.EncryptBlob(s.entryKey, plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt blob: %w", err)
	}
	return blob, nil
}

// DecryptGroup tager et gruppe-blob (object_kind=2, downloadet via
// GET /changes?include=groups) og returnerer gruppen som canonical JSON.
// Kalderen kender allerede objektets kind fra server-metadata (changes.kind),
// så her kræver vi at blob'en faktisk er en gruppe (envelope 0x02). v4 group-sync.
func (s *Session) DecryptGroup(blob []byte) ([]byte, error) {
	if s.entryKey == nil {
		return nil, errors.New("session closed")
	}

	plaintext, err := crypto.DecryptBlob(s.entryKey, blob)
	if err != nil {
		return nil, fmt.Errorf("decrypt blob: %w", err)
	}

	if canonical.DetectFormat(plaintext) != canonical.FormatGroup {
		return nil, errors.New("blob is not a canonical group (expected envelope byte 0x02)")
	}
	group, err := canonical.DecodeGroup(plaintext)
	if err != nil {
		return nil, fmt.Errorf("decode group: %w", err)
	}
	return json.Marshal(group)
}

// DeviceKeypair returnerer fra GenerateDeviceKeypair. Pakket som struct så
// gomobile bind kan generere en stabil Kotlin-class med to byte[]-gettere
// — multi-return-non-error funcs har dårlig support i gomobile.
type DeviceKeypair struct {
	PublicKey  []byte
	PrivateKey []byte
}

// GenerateDeviceKeypair producerer en X25519-keypair brugt til v2 sharing.
// Public-delen postes til serveren ved enrollment; private-delen gemmes
// lokalt (Android EncryptedSharedPreferences / Keystore). Begge slices er
// 32 bytes.
func GenerateDeviceKeypair() (*DeviceKeypair, error) {
	pub, priv, err := crypto.GenerateBoxKeypair()
	if err != nil {
		return nil, err
	}
	return &DeviceKeypair{PublicKey: pub, PrivateKey: priv}, nil
}

// UnwrapSharedMasterKey er medlems-side af v2 sharing: når Bob henter
// listen af databaser fra server, indeholder hver entry han er medlem af
// et `wrapped_master_key` (sealed-box til hans device public-key). Denne
// funktion unwrapper det til den faktiske master_key, som Bob derefter
// føder til NewSessionFromMasterKey.
//
// devicePublicKey er pubkey'en der matcher devicePrivateKey — den kan
// regenereres via PublicKeyFromPrivate hvis Android-koden kun har gemt
// privaten.
func UnwrapSharedMasterKey(wrappedBlob, devicePublicKey, devicePrivateKey []byte) ([]byte, error) {
	return crypto.UnwrapKey(wrappedBlob, devicePublicKey, devicePrivateKey)
}

// PublicKeyFromPrivate udleder X25519-public-key fra en private-key.
// Bruges når Android-koden kun har gemt privaten (sparer plads) og har
// brug for at give pubkey'en til UnwrapSharedMasterKey.
func PublicKeyFromPrivate(privateKey []byte) ([]byte, error) {
	return crypto.PublicKeyFromPrivate(privateKey)
}

// WrapMasterKeyForShare er ejer-side af v2 sharing — modstykket til
// UnwrapSharedMasterKey. Alice (owner) deriverer database master_key fra sit
// masterpassword (Argon2id, ~200ms) og wrap'er det som sealed-box til
// target-enhedens public-key. Resultatet POST'es til
// /databases/{id}/shares som det opaque `wrapped_master_key`; kun
// target-enhedens private-key kan unwrappe det igen.
//
// Spejler desktop-klientens `runShare` (DeriveMasterKey → crypto.WrapKey).
// Caller'en skal zero'e password-bytes efter kaldet returnerer.
func WrapMasterKeyForShare(password []byte, databaseID string, targetPublicKey []byte) ([]byte, error) {
	if len(password) == 0 {
		return nil, errors.New("empty password")
	}
	if databaseID == "" {
		return nil, errors.New("empty databaseID")
	}
	if len(targetPublicKey) != crypto.BoxPublicKeySize {
		return nil, fmt.Errorf("target public key must be %d bytes, got %d", crypto.BoxPublicKeySize, len(targetPublicKey))
	}
	masterKey, err := crypto.DeriveMasterKey(password, databaseID)
	if err != nil {
		return nil, fmt.Errorf("derive master key: %w", err)
	}
	defer zero(masterKey)
	return crypto.WrapKey(masterKey, targetPublicKey)
}

// SchemaVersion eksponerer canonical-skemaets aktuelle version som en
// konstant Kotlin kan tjekke. Brug den i tests for at fange schema-drift
// mellem Go og Kotlin sider.
const SchemaVersion = canonical.SchemaVersion

// zero overskriver et byte-slice med nuller. Bruges på Session.Close og
// efter master-key-derivation. Compiler er ikke garanteret at lade
// optimeringen stå — for crypto-grade zeroing skal vi bruge runtime/debug
// eller subtle.ConstantTimeCompare-tricks. For nu er en simpel loop OK;
// vi opgradere hvis sikkerhedsreview kræver det.
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
