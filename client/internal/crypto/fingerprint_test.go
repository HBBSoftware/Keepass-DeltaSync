// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"regexp"
	"testing"
)

func TestKeyFingerprintFormat(t *testing.T) {
	pub := make([]byte, BoxPublicKeySize)
	fp := KeyFingerprint(pub)
	if !regexp.MustCompile(`^[0-9A-F]{4}(-[0-9A-F]{4}){4}$`).MatchString(fp) {
		t.Fatalf("unexpected format: %q", fp)
	}
	// Fast værdi: Android regner via gomobile, men en ændring her ville stille
	// gøre alle gemte og oplæste aftryk ugyldige. SHA-256 af 32 nul-bytes.
	if want := "6668-7AAD-F862-BD77-6C8F"; fp != want {
		t.Fatalf("fingerprint of zero key = %q, want %q", fp, want)
	}
}

func TestKeyFingerprintDiffers(t *testing.T) {
	a, _, _ := GenerateBoxKeypair()
	b, _, _ := GenerateBoxKeypair()
	if KeyFingerprint(a) == KeyFingerprint(b) {
		t.Fatal("two fresh keys share a fingerprint")
	}
	if KeyID(a) == KeyID(b) || len(KeyID(a)) != 64 {
		t.Fatalf("bad key id %q", KeyID(a))
	}
}

func TestSameFingerprint(t *testing.T) {
	fp := "7F3A-91C2-0D4E-B8A1-55E0"
	for _, s := range []string{fp, "7f3a91c20d4eb8a155e0", " 7F3A 91C2 0D4E B8A1 55E0 "} {
		if !SameFingerprint(s, fp) {
			t.Errorf("%q should match", s)
		}
	}
	for _, s := range []string{"", "7F3A-91C2-0D4E-B8A1-55E1", "7F3A"} {
		if SameFingerprint(s, fp) {
			t.Errorf("%q should not match", s)
		}
	}
}
