// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// Stable test fixtures. fixedDBUUID er en kanonisk testvalue; ændringer her
// invaliderer known-vector-testen og vil fange utilsigtede ændringer i KDF-
// pipelinen.
const (
	fixedDBUUID   = "01234567-89ab-cdef-0123-456789abcdef"
	fixedPassword = "correct horse battery staple"
)

func TestDeriveMasterKey_Deterministic(t *testing.T) {
	k1, err := DeriveMasterKey([]byte(fixedPassword), fixedDBUUID)
	if err != nil {
		t.Fatalf("derive 1: %v", err)
	}
	k2, err := DeriveMasterKey([]byte(fixedPassword), fixedDBUUID)
	if err != nil {
		t.Fatalf("derive 2: %v", err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatalf("same inputs should yield same key: %x vs %x", k1, k2)
	}
	if len(k1) != 32 {
		t.Fatalf("master key must be 32 bytes, got %d", len(k1))
	}
}

// TestDeriveMasterKey_KnownVector låser KDF-output ned med en kendt værdi.
// Hvis denne test fejler er KDF-parametre eller -input ændret, og alle tidligere
// krypterede databaser vil ikke kunne dekrypteres med samme password.
// Værdien er optaget fra første kørsel og er ikke beregnet ekstern — den
// fanger UTILSIGTEDE ændringer, ikke korrekthed.
func TestDeriveMasterKey_KnownVector(t *testing.T) {
	const expectedHex = "bf507c394dc4288031c6cc889a9db64033fac3f448082a06a42b8b0e56e8c830"
	k, err := DeriveMasterKey([]byte(fixedPassword), fixedDBUUID)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	got := hex.EncodeToString(k)
	if expectedHex == "" {
		// Første-kørsel mode: print så vi kan fylde konstanten ind.
		t.Logf("RECORD: expectedHex = %q", got)
		return
	}
	if got != expectedHex {
		t.Fatalf("KDF output changed!\n  expected: %s\n  got:      %s", expectedHex, got)
	}
}

func TestDeriveEntryKey_DependsOnUUID(t *testing.T) {
	mk, err := DeriveMasterKey([]byte(fixedPassword), fixedDBUUID)
	if err != nil {
		t.Fatalf("master: %v", err)
	}
	ek1, err := DeriveEntryKey(mk, fixedDBUUID)
	if err != nil {
		t.Fatalf("entry 1: %v", err)
	}
	ek2, err := DeriveEntryKey(mk, "fedcba98-7654-3210-fedc-ba9876543210")
	if err != nil {
		t.Fatalf("entry 2: %v", err)
	}
	if bytes.Equal(ek1, ek2) {
		t.Fatal("entry keys for different UUIDs must differ")
	}
}

func TestParseUUID_AcceptsBothForms(t *testing.T) {
	withDashes, err := parseUUIDBytes("01234567-89ab-cdef-0123-456789abcdef")
	if err != nil {
		t.Fatalf("with dashes: %v", err)
	}
	withoutDashes, err := parseUUIDBytes("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("without dashes: %v", err)
	}
	if withDashes != withoutDashes {
		t.Fatal("dashed and undashed UUIDs should parse to same bytes")
	}
}

func TestParseUUID_RejectsBadInput(t *testing.T) {
	for _, bad := range []string{
		"",
		"not-a-uuid",
		"0123456789abcdef0123456789abcde",  // 31 chars
		"0123456789abcdef0123456789abcdez", // non-hex
	} {
		if _, err := parseUUIDBytes(bad); err == nil {
			t.Errorf("expected error for %q, got nil", bad)
		}
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	mk, _ := DeriveMasterKey([]byte(fixedPassword), fixedDBUUID)
	key, _ := DeriveEntryKey(mk, fixedDBUUID)

	plaintext := []byte("hello, sync world — æøå")
	blob, err := EncryptBlob(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if len(blob) < NonceSize {
		t.Fatalf("blob shorter than nonce")
	}

	decoded, err := DecryptBlob(key, blob)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(plaintext, decoded) {
		t.Fatalf("round-trip mismatch:\n  in:  %q\n  out: %q", plaintext, decoded)
	}
}

func TestEncrypt_RandomNonce(t *testing.T) {
	mk, _ := DeriveMasterKey([]byte(fixedPassword), fixedDBUUID)
	key, _ := DeriveEntryKey(mk, fixedDBUUID)

	// Samme plaintext to gange skal give to forskellige blobs pga. random nonce.
	plaintext := []byte("same input")
	b1, _ := EncryptBlob(key, plaintext)
	b2, _ := EncryptBlob(key, plaintext)
	if bytes.Equal(b1, b2) {
		t.Fatal("two encryptions of same plaintext produced identical blobs — nonce reuse")
	}
}

func TestDecrypt_WrongKeyFails(t *testing.T) {
	mk, _ := DeriveMasterKey([]byte(fixedPassword), fixedDBUUID)
	key, _ := DeriveEntryKey(mk, fixedDBUUID)

	blob, _ := EncryptBlob(key, []byte("secret"))

	wrongMK, _ := DeriveMasterKey([]byte("wrong password"), fixedDBUUID)
	wrongKey, _ := DeriveEntryKey(wrongMK, fixedDBUUID)
	if _, err := DecryptBlob(wrongKey, blob); err == nil {
		t.Fatal("decrypt with wrong key should fail")
	}
}

func TestDecrypt_TamperedFails(t *testing.T) {
	mk, _ := DeriveMasterKey([]byte(fixedPassword), fixedDBUUID)
	key, _ := DeriveEntryKey(mk, fixedDBUUID)

	blob, _ := EncryptBlob(key, []byte("payload"))
	tampered := append([]byte{}, blob...)
	tampered[len(tampered)-1] ^= 1 // flip last bit (in the Poly1305 tag area)
	if _, err := DecryptBlob(key, tampered); err == nil || !strings.Contains(err.Error(), "decrypt") {
		t.Fatalf("expected decrypt-failure on tampered blob, got: %v", err)
	}
}

func TestDecrypt_TooShortFails(t *testing.T) {
	mk, _ := DeriveMasterKey([]byte(fixedPassword), fixedDBUUID)
	key, _ := DeriveEntryKey(mk, fixedDBUUID)
	if _, err := DecryptBlob(key, []byte{1, 2, 3}); err == nil {
		t.Fatal("expected error on blob shorter than nonce+tag")
	}
}
