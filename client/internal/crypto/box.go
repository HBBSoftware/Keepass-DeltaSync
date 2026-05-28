// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

// BoxPublicKeySize / BoxPrivateKeySize er størrelsen af X25519 (Curve25519) keys
// som NaCl's box bruger. Begge er 32 bytes. Eksponeret som konstanter så caller
// (config, network) kan validere uden at importere nacl direkte.
const (
	BoxPublicKeySize  = 32
	BoxPrivateKeySize = 32
)

// GenerateBoxKeypair laver et frisk X25519 keypair til sealed-box wrapping af
// database master_keys i v2's multi-bruger sharing. Hver enhed har ét keypair,
// genereret lokalt ved enroll, hvor public-delen lagres på server og private-
// delen lever lokalt (i config.toml).
//
// Returnerede slices ejes af caller. Private-key'en er sensitive — caller
// bør zeroe den når den ikke længere skal bruges (passwd.Zero).
func GenerateBoxKeypair() (publicKey, privateKey []byte, err error) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate box keypair: %w", err)
	}
	return pub[:], priv[:], nil
}

// WrapKey krypterer plaintext (typisk en 32-byte master_key) til en sealed-
// box for recipient. Sealed-box mode bruger et ephemeral keypair pr. wrap,
// så hvert kald giver unikt ciphertext selv for identiske inputs — afsender
// behøver ikke eget keypair og er anonym for serveren.
//
// Format på output: ephemeral_pub (32) ‖ ciphertext (plaintext+16 Poly1305 mac).
// Overhead er præcis 48 bytes.
func WrapKey(plaintext, recipientPublicKey []byte) ([]byte, error) {
	if len(recipientPublicKey) != BoxPublicKeySize {
		return nil, fmt.Errorf("recipient public key must be %d bytes, got %d", BoxPublicKeySize, len(recipientPublicKey))
	}
	var pub [BoxPublicKeySize]byte
	copy(pub[:], recipientPublicKey)
	sealed, err := box.SealAnonymous(nil, plaintext, &pub, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("seal anonymous: %w", err)
	}
	return sealed, nil
}

// UnwrapKey åbner en sealed-box krypteret med vores public-key. Hvis ciphertext
// er tamper'et med, eller hvis ourPublicKey/ourPrivateKey ikke matcher det
// keypair der oprindeligt blev wrap'et til, returneres en fejl uden at afsløre
// hvorfor (Poly1305 MAC-fejl = generisk "unwrap failed").
func UnwrapKey(sealed, ourPublicKey, ourPrivateKey []byte) ([]byte, error) {
	if len(ourPublicKey) != BoxPublicKeySize {
		return nil, fmt.Errorf("public key must be %d bytes, got %d", BoxPublicKeySize, len(ourPublicKey))
	}
	if len(ourPrivateKey) != BoxPrivateKeySize {
		return nil, fmt.Errorf("private key must be %d bytes, got %d", BoxPrivateKeySize, len(ourPrivateKey))
	}
	var pub, priv [BoxPublicKeySize]byte
	copy(pub[:], ourPublicKey)
	copy(priv[:], ourPrivateKey)
	plaintext, ok := box.OpenAnonymous(nil, sealed, &pub, &priv)
	if !ok {
		return nil, errors.New("unwrap failed: wrong recipient key or tampered ciphertext")
	}
	return plaintext, nil
}
