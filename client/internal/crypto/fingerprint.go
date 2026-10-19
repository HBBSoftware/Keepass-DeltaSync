// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// fingerprintBytes er hvor meget af SHA-256 fingeraftrykket viser: 10 bytes =
// 80 bit = 20 hex-tegn. Det er nok til at en server ikke kan finde en anden
// nøgle med samme aftryk (2^80 forsøg), og kort nok til at læse op i telefon.
const fingerprintBytes = 10

// KeyFingerprint er det menneskelæsbare aftryk af en enheds public-key, fx
// "7F3A-91C2-0D4E-B8A1-55E0". Det er det to brugere sammenligner før deling:
// serveren leverer modtagerens public-key, og uden en sammenligning ad en
// anden kanal kan en kompromitteret server levere sin egen og få master_key.
//
// Android kalder den samme funktion via gomobile (mobile.KeyFingerprint), så
// de to platforme ikke kan komme til at regne forskelligt.
func KeyFingerprint(publicKey []byte) string {
	sum := sha256.Sum256(publicKey)
	h := strings.ToUpper(hex.EncodeToString(sum[:fingerprintBytes]))
	groups := make([]string, 0, len(h)/4)
	for i := 0; i < len(h); i += 4 {
		groups = append(groups, h[i:i+4])
	}
	return strings.Join(groups, "-")
}

// KeyID er den fulde SHA-256 af en public-key som hex. Det er det klienten
// gemmer om en kendt modtager: hele hashen, ikke det forkortede aftryk, så
// sammenligningen ved næste deling ikke hviler på 80 bit.
func KeyID(publicKey []byte) string {
	sum := sha256.Sum256(publicKey)
	return hex.EncodeToString(sum[:])
}

// SameFingerprint sammenligner et aftryk som brugeren har skrevet eller
// kopieret med et beregnet. Store/små bogstaver, bindestreger og mellemrum
// er ligegyldige.
func SameFingerprint(a, b string) bool {
	norm := func(s string) string {
		return strings.NewReplacer("-", "", " ", "", ":", "").Replace(strings.ToUpper(strings.TrimSpace(s)))
	}
	na := norm(a)
	return na != "" && na == norm(b)
}

// KeyCheck er en kontrolværdi for en databases entry-key: de første 128 bit af
// HMAC-SHA256(entryKey, "deltasync-key-check-v1"), som hex. Klienten gemmer
// den i config.toml og sammenligner ved hver kørsel, så en server ikke i
// stilhed kan få klienten til at kryptere under en anden nøgle — fx ved at
// kalde en ejer for medlem og levere sin egen wrapped_master_key.
//
// Værdien afslører ikke nøglen. Den lader dog den der har config.toml teste
// gæt på masterpassword'et offline (Argon2id pr. gæt); det kan den lokale
// .kdbx-fil allerede.
func KeyCheck(entryKey []byte) string {
	m := hmac.New(sha256.New, entryKey)
	m.Write([]byte("deltasync-key-check-v1"))
	return hex.EncodeToString(m.Sum(nil)[:16])
}

// SameKeyCheck sammenligner to kontrolværdier i konstant tid.
func SameKeyCheck(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}
