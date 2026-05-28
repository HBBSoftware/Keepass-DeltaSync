// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"bytes"
	"testing"
)

func TestGenerateBoxKeypair_SizesAndUniqueness(t *testing.T) {
	pub1, priv1, err := GenerateBoxKeypair()
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	if len(pub1) != BoxPublicKeySize {
		t.Fatalf("public key length = %d, want %d", len(pub1), BoxPublicKeySize)
	}
	if len(priv1) != BoxPrivateKeySize {
		t.Fatalf("private key length = %d, want %d", len(priv1), BoxPrivateKeySize)
	}

	pub2, priv2, err := GenerateBoxKeypair()
	if err != nil {
		t.Fatalf("second generate: %v", err)
	}
	if bytes.Equal(pub1, pub2) {
		t.Fatal("two generated public keys are identical — rand.Reader broken?")
	}
	if bytes.Equal(priv1, priv2) {
		t.Fatal("two generated private keys are identical — rand.Reader broken?")
	}
}

func TestWrapUnwrap_RoundTrip(t *testing.T) {
	bobPub, bobPriv, err := GenerateBoxKeypair()
	if err != nil {
		t.Fatalf("generate bob: %v", err)
	}
	masterKey := []byte("this-is-a-32-byte-master-key.xx!")
	if len(masterKey) != 32 {
		t.Fatalf("test fixture: masterKey must be 32 bytes, got %d", len(masterKey))
	}

	sealed, err := WrapKey(masterKey, bobPub)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	if len(sealed) != len(masterKey)+48 {
		t.Fatalf("sealed length = %d, want %d (plaintext+48 sealed-box overhead)", len(sealed), len(masterKey)+48)
	}

	out, err := UnwrapKey(sealed, bobPub, bobPriv)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if !bytes.Equal(out, masterKey) {
		t.Fatalf("round-trip mismatch: got %x, want %x", out, masterKey)
	}
}

func TestWrapKey_NondeterministicOutput(t *testing.T) {
	// Sealed-box bruger ephemeral keypair pr. wrap → samme inputs giver
	// forskellige outputs. Hvis vi nogensinde får determinisme her, har vi
	// brudt en kritisk sikkerheds-garanti.
	pub, _, err := GenerateBoxKeypair()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	plaintext := []byte("constant")

	a, err := WrapKey(plaintext, pub)
	if err != nil {
		t.Fatalf("wrap a: %v", err)
	}
	b, err := WrapKey(plaintext, pub)
	if err != nil {
		t.Fatalf("wrap b: %v", err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("two wraps of same plaintext+pub produced identical sealed-boxes — ephemeral key fixed?")
	}
}

func TestUnwrapKey_TamperedCiphertextFails(t *testing.T) {
	pub, priv, err := GenerateBoxKeypair()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	sealed, err := WrapKey([]byte("secret"), pub)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	// Flip ét bit i ciphertext-delen (efter 32-byte ephemeral pub-prefix).
	tampered := make([]byte, len(sealed))
	copy(tampered, sealed)
	tampered[40] ^= 0x01
	if _, err := UnwrapKey(tampered, pub, priv); err == nil {
		t.Fatal("unwrap of tampered sealed-box should fail, but succeeded")
	}
}

func TestUnwrapKey_WrongRecipientFails(t *testing.T) {
	bobPub, _, err := GenerateBoxKeypair()
	if err != nil {
		t.Fatalf("generate bob: %v", err)
	}
	alicePub, alicePriv, err := GenerateBoxKeypair()
	if err != nil {
		t.Fatalf("generate alice: %v", err)
	}
	sealed, err := WrapKey([]byte("for-bob"), bobPub)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	// Alice forsøger at åbne en box wrappet til Bob.
	if _, err := UnwrapKey(sealed, alicePub, alicePriv); err == nil {
		t.Fatal("unwrap with wrong keypair should fail, but succeeded")
	}
}

func TestWrapKey_RejectsWrongSizedPublicKey(t *testing.T) {
	if _, err := WrapKey([]byte("x"), make([]byte, 31)); err == nil {
		t.Fatal("WrapKey accepted 31-byte public key, should reject")
	}
	if _, err := WrapKey([]byte("x"), make([]byte, 33)); err == nil {
		t.Fatal("WrapKey accepted 33-byte public key, should reject")
	}
}

func TestUnwrapKey_RejectsWrongSizedKeys(t *testing.T) {
	pub, priv, _ := GenerateBoxKeypair()
	sealed, _ := WrapKey([]byte("x"), pub)
	if _, err := UnwrapKey(sealed, pub[:31], priv); err == nil {
		t.Fatal("UnwrapKey accepted short public key, should reject")
	}
	if _, err := UnwrapKey(sealed, pub, priv[:31]); err == nil {
		t.Fatal("UnwrapKey accepted short private key, should reject")
	}
}
