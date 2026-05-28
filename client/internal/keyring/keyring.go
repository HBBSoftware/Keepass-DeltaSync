// SPDX-License-Identifier: GPL-3.0-or-later

// Package keyring er en tynd wrapper omkring zalando/go-keyring der gemmer
// masterpasswords pr. database i OS' native credential store (Windows
// Credential Manager / macOS Keychain / Secret Service på Linux).
//
// Service-navnet er fast — "keepass-deltasync". Hver registreret database
// gemmes under sit RemoteID som key, så samme bruger kan have flere databaser
// uden navnekollision.
package keyring

import (
	"errors"
	"fmt"

	zkr "github.com/zalando/go-keyring"
)

const service = "keepass-deltasync"

// ErrNotFound returneres når der ikke er noget gemt password for det
// RemoteID. Caller'en kan så falde tilbage til interaktiv prompt.
var ErrNotFound = errors.New("no password stored for this database")

// Get henter password fra OS keyring. Returnerer ErrNotFound hvis ingen
// entry findes for det RemoteID.
func Get(remoteID string) ([]byte, error) {
	pw, err := zkr.Get(service, remoteID)
	if err != nil {
		if errors.Is(err, zkr.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("keyring get %s: %w", remoteID, err)
	}
	return []byte(pw), nil
}

// Set gemmer (eller overskriver) password for et RemoteID. Caller'en bør
// stadig zeroe sin egen kopi af password efter dette kald — vi kan ikke
// undgå at zalando-libben holder en string-kopi internt et øjeblik, men
// vores side af snittet behøver ikke beholde plaintexten.
func Set(remoteID string, password []byte) error {
	if err := zkr.Set(service, remoteID, string(password)); err != nil {
		return fmt.Errorf("keyring set %s: %w", remoteID, err)
	}
	return nil
}

// Delete fjerner password for et RemoteID. No-op-fejl hvis der ikke var
// noget at slette — det er ikke en fejl-tilstand for caller'en.
func Delete(remoteID string) error {
	err := zkr.Delete(service, remoteID)
	if err == nil || errors.Is(err, zkr.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("keyring delete %s: %w", remoteID, err)
}
