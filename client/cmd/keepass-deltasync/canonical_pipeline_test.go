// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/crypto"
	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx/canonical"
)

// testEntryKey er en deterministisk 32-byte nøgle til tests. Vi bruger den
// til at kryptere/dekryptere blobs så vi kan teste canonical-pipeline'en
// uden at gå igennem keepassxc-cli.
var testEntryKey = bytes.Repeat([]byte{0x42}, 32)

// sampleInnerXMLFragment er et minimalt fragment der dækker det push-stien
// får fra kdbx.ParseExport: UUID, Times, et par String-felter (med
// Protected på Password), og AutoType. Holdt deterministisk så tests kan
// sammenligne struct-equality.
const sampleInnerXMLFragment = `<UUID>AAECAwQFBgcICQoLDA0ODw==</UUID>` +
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
	`<String><Key>Title</Key><Value>GitLab</Value></String>` +
	`<String><Key>UserName</Key><Value>hans</Value></String>` +
	`<String><Key>Password</Key><Value Protected="True">secret</Value></String>`

func TestEncodeFragmentToBlob_ProducesCanonical(t *testing.T) {
	blob, err := encodeFragmentToBlob(testEntryKey, "00010203-0405-0607-0809-0a0b0c0d0e0f", []byte(sampleInnerXMLFragment))
	if err != nil {
		t.Fatalf("encodeFragmentToBlob: %v", err)
	}

	// Dekrypter manuelt og verificer at det er canonical-formatet (byte 0 == 0x01),
	// ikke rå XML (byte 0 == '<').
	plaintext, err := crypto.DecryptBlob(testEntryKey, blob)
	if err != nil {
		t.Fatalf("DecryptBlob: %v", err)
	}
	if got := canonical.DetectFormat(plaintext); got != canonical.FormatCanonical {
		t.Errorf("blob plaintext format = %v, want FormatCanonical", got)
	}
}

func TestEncodeFragmentToBlob_BadFragmentErrors(t *testing.T) {
	// FromInnerXML fejler på et fragment uden UUID — vi skal se en wrapped
	// fejl der nævner entry-UUID for debugability.
	_, err := encodeFragmentToBlob(testEntryKey, "bad-uuid", []byte("<NoUUIDHere/>"))
	if err == nil {
		t.Fatal("expected error for fragment without parseable UUID")
	}
	if !strings.Contains(err.Error(), "bad-uuid") {
		t.Errorf("error %q should mention entry UUID for debug context", err)
	}
}

func TestDecryptToFragment_CanonicalPath(t *testing.T) {
	// Byg en canonical-blob via samme path som push'en bruger.
	blob, err := encodeFragmentToBlob(testEntryKey, "00010203-0405-0607-0809-0a0b0c0d0e0f", []byte(sampleInnerXMLFragment))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// Server vil annoncere en ny mtime (typisk fordi vi har gemt med en
	// frisk timestamp ved push, eller en restore har bumped seq). decrypt-
	// stien skal sætte den interne LastModificationTime til denne værdi.
	serverMtime := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)

	fragment, err := decryptToFragment(testEntryKey, blob, "00010203-0405-0607-0809-0a0b0c0d0e0f", serverMtime)
	if err != nil {
		t.Fatalf("decryptToFragment: %v", err)
	}

	// Parse fragmentet tilbage og verificer at Modified blev rewritten til
	// server's mtime, ikke den oprindelige 2026-05-15.
	parsed, err := canonical.FromInnerXML(fragment)
	if err != nil {
		t.Fatalf("re-parse fragment: %v", err)
	}
	if !parsed.Times.Modified.Equal(serverMtime) {
		t.Errorf("Times.Modified = %v, want %v (server mtime override)", parsed.Times.Modified, serverMtime)
	}
	// Andre felter skal være bevaret.
	if parsed.Strings["Title"].V != "GitLab" {
		t.Errorf("Title lost in round-trip: %q", parsed.Strings["Title"].V)
	}
	if !parsed.Strings["Password"].Protected {
		t.Error("Password lost Protected flag in round-trip")
	}
}

func TestDecryptToFragment_LegacyPath(t *testing.T) {
	// Simulér en legacy-blob fra før v3: krypter rå InnerXML uden version-prefix.
	legacyBlob, err := crypto.EncryptBlob(testEntryKey, []byte(sampleInnerXMLFragment))
	if err != nil {
		t.Fatalf("encrypt legacy: %v", err)
	}

	serverMtime := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)

	fragment, err := decryptToFragment(testEntryKey, legacyBlob, "00010203-0405-0607-0809-0a0b0c0d0e0f", serverMtime)
	if err != nil {
		t.Fatalf("decryptToFragment: %v", err)
	}

	// Legacy-stien bruger rewriteLastModificationTime, så den nye mtime
	// skal være i fragmentet OG entry'ens egen <LastModificationTime> skal
	// være væk. Andre 2026-05-15-tidsstempler (CreationTime/LastAccessTime/
	// ExpiryTime) er ikke mtime og skal med vilje bevares.
	wantNew := []byte("<LastModificationTime>2026-05-30T12:00:00Z</LastModificationTime>")
	if !bytes.Contains(fragment, wantNew) {
		t.Errorf("legacy path didn't rewrite mtime; fragment:\n%s", fragment)
	}
	if bytes.Contains(fragment, []byte("<LastModificationTime>2026-05-15T10:00:00Z</LastModificationTime>")) {
		t.Errorf("legacy path didn't strip old mtime tag; fragment:\n%s", fragment)
	}
}

func TestDecryptToFragment_UnknownFormatErrors(t *testing.T) {
	// Krypter en byte-string der hverken starter med '<' eller 0x01.
	weirdPlaintext := []byte{0xff, 0xff, 0xff}
	blob, err := crypto.EncryptBlob(testEntryKey, weirdPlaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = decryptToFragment(testEntryKey, blob, "test-uuid", time.Now())
	if err == nil {
		t.Fatal("expected error for unknown format byte")
	}
	if !strings.Contains(err.Error(), "unrecognized blob format") {
		t.Errorf("error %q should mention unrecognized format", err)
	}
}

func TestDecryptToFragment_WrongKeyErrors(t *testing.T) {
	// Krypter med én nøgle, dekrypter med en anden — skal fejle på AEAD's MAC.
	blob, err := encodeFragmentToBlob(testEntryKey, "00010203-0405-0607-0809-0a0b0c0d0e0f", []byte(sampleInnerXMLFragment))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	wrongKey := bytes.Repeat([]byte{0x99}, 32)
	_, err = decryptToFragment(wrongKey, blob, "test-uuid", time.Now())
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
	if !strings.Contains(err.Error(), "decrypt failed") {
		t.Errorf("error %q should mention decrypt failure", err)
	}
}

// TestPipelineRoundTrip simulerer hele push→pull cyklen: push converterer
// fragment til canonical-blob; pull dekrypterer og giver fragment tilbage;
// efter to round-trips skal indholdet være ækvivalent (Modified ændres
// bevidst, men resten bevares).
func TestPipelineRoundTrip(t *testing.T) {
	pushBlob, err := encodeFragmentToBlob(testEntryKey, "00010203-0405-0607-0809-0a0b0c0d0e0f", []byte(sampleInnerXMLFragment))
	if err != nil {
		t.Fatalf("push: %v", err)
	}

	serverMtime := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	pullFragment, err := decryptToFragment(testEntryKey, pushBlob, "00010203-0405-0607-0809-0a0b0c0d0e0f", serverMtime)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}

	// Re-push den pullede fragment.
	rePushBlob, err := encodeFragmentToBlob(testEntryKey, "00010203-0405-0607-0809-0a0b0c0d0e0f", pullFragment)
	if err != nil {
		t.Fatalf("re-push: %v", err)
	}

	// Pull igen med samme mtime — verificer struct-equality af de to
	// canonical entries.
	rePullFragment, err := decryptToFragment(testEntryKey, rePushBlob, "00010203-0405-0607-0809-0a0b0c0d0e0f", serverMtime)
	if err != nil {
		t.Fatalf("re-pull: %v", err)
	}

	a, _ := canonical.FromInnerXML(pullFragment)
	b, _ := canonical.FromInnerXML(rePullFragment)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("pipeline round-trip drift:\nfirst:  %+v\nsecond: %+v", a, b)
	}
}
