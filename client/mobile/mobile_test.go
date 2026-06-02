// SPDX-License-Identifier: GPL-3.0-or-later

package mobile

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/crypto"
	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx/canonical"
)

const testDBID = "00000000-0000-0000-0000-000000000001"

func sampleEntryJSON(t *testing.T) []byte {
	t.Helper()
	e := canonical.Entry{
		V:    canonical.SchemaVersion,
		UUID: "00010203-0405-0607-0809-0a0b0c0d0e0f",
		Times: canonical.Times{
			Created:         time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			Modified:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			Accessed:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			LocationChanged: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
		},
		Strings: map[string]canonical.String{
			"Title":    {V: "Sample"},
			"Password": {V: "hunter2", Protected: true},
		},
	}
	raw, err := json.Marshal(&e)
	if err != nil {
		t.Fatalf("marshal sample: %v", err)
	}
	return raw
}

func TestNewSession_DerivesKey(t *testing.T) {
	s, err := NewSession([]byte("master-password"), testDBID)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer s.Close()
	if len(s.entryKey) != 32 {
		t.Errorf("entryKey size = %d, want 32", len(s.entryKey))
	}
}

func TestNewSession_RejectsEmptyInputs(t *testing.T) {
	if _, err := NewSession([]byte{}, testDBID); err == nil {
		t.Error("expected error for empty password")
	}
	if _, err := NewSession([]byte("pw"), ""); err == nil {
		t.Error("expected error for empty databaseID")
	}
}

func TestNewSessionFromMasterKey(t *testing.T) {
	masterKey := bytes.Repeat([]byte{0x42}, 32)
	s, err := NewSessionFromMasterKey(testDBID, masterKey)
	if err != nil {
		t.Fatalf("NewSessionFromMasterKey: %v", err)
	}
	defer s.Close()
	if len(s.entryKey) != 32 {
		t.Errorf("entryKey size = %d, want 32", len(s.entryKey))
	}
}

func TestNewSessionFromMasterKey_RejectsBadSize(t *testing.T) {
	if _, err := NewSessionFromMasterKey(testDBID, make([]byte, 16)); err == nil {
		t.Error("expected error for 16-byte master key")
	}
}

func TestEntryRoundTrip(t *testing.T) {
	s, err := NewSession([]byte("master-password"), testDBID)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer s.Close()

	entryJSON := sampleEntryJSON(t)

	blob, err := s.EncryptEntry(entryJSON)
	if err != nil {
		t.Fatalf("EncryptEntry: %v", err)
	}

	decoded, err := s.DecryptEntry(blob)
	if err != nil {
		t.Fatalf("DecryptEntry: %v", err)
	}

	// Sammenlign som strukturer for at undgå whitespace/key-ordering
	// følsomhed.
	var a, b canonical.Entry
	if err := json.Unmarshal(entryJSON, &a); err != nil {
		t.Fatalf("unmarshal original: %v", err)
	}
	if err := json.Unmarshal(decoded, &b); err != nil {
		t.Fatalf("unmarshal decoded: %v", err)
	}
	if a.UUID != b.UUID || a.Strings["Title"].V != b.Strings["Title"].V {
		t.Errorf("round-trip mismatch: %+v vs %+v", a, b)
	}
	if b.V != canonical.SchemaVersion {
		t.Errorf("decoded.V = %d, want %d", b.V, canonical.SchemaVersion)
	}
}

func TestSession_ClosedRejectsOperations(t *testing.T) {
	s, err := NewSession([]byte("master-password"), testDBID)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	entryJSON := sampleEntryJSON(t)
	blob, err := s.EncryptEntry(entryJSON)
	if err != nil {
		t.Fatalf("encrypt before close: %v", err)
	}

	s.Close()

	if _, err := s.EncryptEntry(entryJSON); err == nil {
		t.Error("expected EncryptEntry to fail after Close")
	}
	if _, err := s.DecryptEntry(blob); err == nil {
		t.Error("expected DecryptEntry to fail after Close")
	}
}

// TestDecryptLegacyBlob simulerer at en gammel desktop-klient pushede et
// v1-blob (rå InnerXML, ingen format-byte). Mobil-stien skal kunne læse den
// og returnere canonical JSON med korrekt SchemaVersion sat.
func TestDecryptLegacyBlob(t *testing.T) {
	s, err := NewSession([]byte("master-password"), testDBID)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer s.Close()

	// En minimal InnerXML-fragment a la keepassxc-cli's export.
	legacyFragment := []byte(
		`<UUID>AAECAwQFBgcICQoLDA0ODw==</UUID>` +
			`<IconID>0</IconID>` +
			`<ForegroundColor></ForegroundColor>` +
			`<BackgroundColor></BackgroundColor>` +
			`<OverrideURL></OverrideURL>` +
			`<Tags></Tags>` +
			`<Times>` +
			`<CreationTime>2026-05-01T10:00:00Z</CreationTime>` +
			`<LastModificationTime>2026-05-15T10:00:00Z</LastModificationTime>` +
			`<LastAccessTime>2026-05-15T10:00:00Z</LastAccessTime>` +
			`<ExpiryTime>2026-05-15T10:00:00Z</ExpiryTime>` +
			`<Expires>False</Expires>` +
			`<UsageCount>0</UsageCount>` +
			`<LocationChanged>2026-05-01T10:00:00Z</LocationChanged>` +
			`</Times>` +
			`<String><Key>Title</Key><Value>LegacyEntry</Value></String>`)

	// Krypter direkte med crypto-laget (uden version-byte prefix) —
	// simulerer v1-blob fra pre-v3-desktop.
	legacyBlob, err := crypto.EncryptBlob(s.entryKey, legacyFragment)
	if err != nil {
		t.Fatalf("encrypt legacy: %v", err)
	}

	decoded, err := s.DecryptEntry(legacyBlob)
	if err != nil {
		t.Fatalf("DecryptEntry legacy: %v", err)
	}

	var entry canonical.Entry
	if err := json.Unmarshal(decoded, &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if entry.V != canonical.SchemaVersion {
		t.Errorf("V = %d, want %d (decoder should normalize legacy)", entry.V, canonical.SchemaVersion)
	}
	if entry.Strings["Title"].V != "LegacyEntry" {
		t.Errorf("Title = %q, want LegacyEntry", entry.Strings["Title"].V)
	}
}

// TestCrossCompat_DesktopBlob verificerer at en blob produceret af
// mobile.EncryptEntry kan dekrypteres af det rene canonical-pipeline
// (samme path som desktop-pull bruger). Det er den vigtigste invariant:
// Android-pushes skal være dekryptbare af desktop og omvendt.
func TestCrossCompat_DesktopBlob(t *testing.T) {
	s, err := NewSession([]byte("master-password"), testDBID)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer s.Close()

	blob, err := s.EncryptEntry(sampleEntryJSON(t))
	if err != nil {
		t.Fatalf("EncryptEntry: %v", err)
	}

	// Brug crypto-laget direkte og parser via canonical-laget — det er
	// præcis hvad desktop-klienten gør.
	plaintext, err := crypto.DecryptBlob(s.entryKey, blob)
	if err != nil {
		t.Fatalf("crypto.DecryptBlob: %v", err)
	}
	if canonical.DetectFormat(plaintext) != canonical.FormatCanonical {
		t.Fatalf("desktop side sees format %v, want FormatCanonical", canonical.DetectFormat(plaintext))
	}
	entry, err := canonical.DecodeCanonical(plaintext)
	if err != nil {
		t.Fatalf("DecodeCanonical: %v", err)
	}
	if entry.UUID != "00010203-0405-0607-0809-0a0b0c0d0e0f" {
		t.Errorf("UUID drift: %q", entry.UUID)
	}
}

func TestEncryptEntry_BadJSON(t *testing.T) {
	s, err := NewSession([]byte("master-password"), testDBID)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer s.Close()

	if _, err := s.EncryptEntry([]byte("not json {")); err == nil {
		t.Error("expected error for malformed JSON")
	}
	if _, err := s.EncryptEntry(nil); err == nil {
		t.Error("expected error for nil entryJSON")
	}
}

func TestSharingFlow_WrapUnwrap(t *testing.T) {
	// Aliceside: et master_key skal wrappes til Bob's device public-key.
	bobKp, err := GenerateDeviceKeypair()
	if err != nil {
		t.Fatalf("generate bob keypair: %v", err)
	}

	aliceMasterKey := bytes.Repeat([]byte{0xab}, 32)

	// Wrap (server-side store dette opaque blob).
	wrapped, err := crypto.WrapKey(aliceMasterKey, bobKp.PublicKey)
	if err != nil {
		t.Fatalf("WrapKey: %v", err)
	}

	// Bob-side: download wrapped, derive pub fra priv, unwrap.
	derivedPub, err := PublicKeyFromPrivate(bobKp.PrivateKey)
	if err != nil {
		t.Fatalf("PublicKeyFromPrivate: %v", err)
	}
	if !bytes.Equal(derivedPub, bobKp.PublicKey) {
		t.Fatal("derivedPub != bobKp.PublicKey")
	}

	unwrapped, err := UnwrapSharedMasterKey(wrapped, derivedPub, bobKp.PrivateKey)
	if err != nil {
		t.Fatalf("UnwrapSharedMasterKey: %v", err)
	}
	if !bytes.Equal(unwrapped, aliceMasterKey) {
		t.Error("unwrapped master_key doesn't match original")
	}

	// Bob bootstrapper en Session med den unwrapped master_key.
	s, err := NewSessionFromMasterKey(testDBID, unwrapped)
	if err != nil {
		t.Fatalf("NewSessionFromMasterKey: %v", err)
	}
	defer s.Close()
	if len(s.entryKey) != 32 {
		t.Errorf("entryKey size = %d, want 32", len(s.entryKey))
	}
}

// TestWrapMasterKeyForShare_EndToEnd verificerer ejer→medlem-stien gennem
// mobile-API'et: Alice wrapper sit (Argon2id-deriverede) master_key til Bob,
// Bob unwrapper, og de to ender med IDENTISKE entry-keys — dvs. Bob kan
// dekryptere det Alice krypterer.
func TestWrapMasterKeyForShare_EndToEnd(t *testing.T) {
	const password = "alice-master-password"

	bobKp, err := GenerateDeviceKeypair()
	if err != nil {
		t.Fatalf("generate bob keypair: %v", err)
	}

	// Alice (owner): wrap master_key til Bob's device public-key.
	wrapped, err := WrapMasterKeyForShare([]byte(password), testDBID, bobKp.PublicKey)
	if err != nil {
		t.Fatalf("WrapMasterKeyForShare: %v", err)
	}

	// Bob (member): unwrap → Session.
	unwrapped, err := UnwrapSharedMasterKey(wrapped, bobKp.PublicKey, bobKp.PrivateKey)
	if err != nil {
		t.Fatalf("UnwrapSharedMasterKey: %v", err)
	}
	bobSession, err := NewSessionFromMasterKey(testDBID, unwrapped)
	if err != nil {
		t.Fatalf("NewSessionFromMasterKey: %v", err)
	}
	defer bobSession.Close()

	// Alice's egen Session deriveret fra password skal have samme entry-key.
	aliceSession, err := NewSession([]byte(password), testDBID)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer aliceSession.Close()

	if !bytes.Equal(aliceSession.entryKey, bobSession.entryKey) {
		t.Error("entry keys differ — Bob cannot decrypt Alice's entries")
	}
}

func TestWrapMasterKeyForShare_RejectsBadInputs(t *testing.T) {
	good, _ := GenerateDeviceKeypair()
	if _, err := WrapMasterKeyForShare(nil, testDBID, good.PublicKey); err == nil {
		t.Error("expected error for empty password")
	}
	if _, err := WrapMasterKeyForShare([]byte("pw"), "", good.PublicKey); err == nil {
		t.Error("expected error for empty databaseID")
	}
	if _, err := WrapMasterKeyForShare([]byte("pw"), testDBID, make([]byte, 16)); err == nil {
		t.Error("expected error for wrong-size public key")
	}
}

func TestSchemaVersion_Exposed(t *testing.T) {
	if SchemaVersion != canonical.SchemaVersion {
		t.Errorf("mobile.SchemaVersion = %d, canonical.SchemaVersion = %d — must match",
			SchemaVersion, canonical.SchemaVersion)
	}
}

func TestSession_CloseZerosKey(t *testing.T) {
	s, err := NewSession([]byte("master-password"), testDBID)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	// Tag en kopi før Close så vi kan inspekt.
	keyCopy := make([]byte, len(s.entryKey))
	copy(keyCopy, s.entryKey)

	s.Close()

	if s.entryKey != nil {
		// Hvis vi ikke nuller pointeren, så kontroller indholdet.
		allZero := true
		for _, b := range s.entryKey {
			if b != 0 {
				allZero = false
				break
			}
		}
		if !allZero {
			t.Error("entryKey not zeroed after Close")
		}
	}

	// Sanity: keyCopy er ikke nul (vi tjekker faktisk nogen lever før Close
	// gjorde sit arbejde).
	allZeroInCopy := true
	for _, b := range keyCopy {
		if b != 0 {
			allZeroInCopy = false
			break
		}
	}
	if allZeroInCopy {
		t.Error("keyCopy was already zero before Close — NewSession didn't derive a key")
	}
}

func TestDecryptEntry_RejectsCorruptBlob(t *testing.T) {
	s, err := NewSession([]byte("master-password"), testDBID)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer s.Close()

	// Random bytes, gennem AEAD vil de fejle MAC-check.
	if _, err := s.DecryptEntry(bytes.Repeat([]byte{0xff}, 64)); err == nil {
		t.Error("expected decrypt failure for random bytes")
	}
}

func TestDecryptEntry_UnknownFormatErrors(t *testing.T) {
	s, err := NewSession([]byte("master-password"), testDBID)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer s.Close()

	// Krypter en byte-sekvens der hverken starter med '<' eller 0x01.
	blob, err := crypto.EncryptBlob(s.entryKey, []byte{0xff, 0xee, 0xdd})
	if err != nil {
		t.Fatalf("encrypt unknown: %v", err)
	}
	_, err = s.DecryptEntry(blob)
	if err == nil {
		t.Fatal("expected error for unknown format byte")
	}
	if !strings.Contains(err.Error(), "unrecognized") {
		t.Errorf("error %q should mention unrecognized format", err)
	}
}
