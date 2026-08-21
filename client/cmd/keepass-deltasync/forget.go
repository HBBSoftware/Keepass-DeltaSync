// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/keyring"
)

// runForget fjerner en lokal database-binding fra config. Det sletter INTET på
// serveren og rører ikke .kdbx-filen — det fjerner kun koblingen lokalt. Bruges
// fx efter en konto-switch hvor en tidligere binding er blevet forældet, eller
// hvis man vil holde op med at synke en database fra denne enhed.
func runForget(args []string) error {
	fs := flag.NewFlagSet("forget", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync forget <name>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("forget takes exactly 1 argument: <name>")
	}
	name := fs.Arg(0)

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	kept := make([]config.Database, 0, len(cfg.Databases))
	found := false
	var localSecret string
	for _, d := range cfg.Databases {
		if d.Name == name {
			found = true
			// En lokal-kun binding har et id vi selv genererede, og et nyt
			// `add-local` genererer et nyt. Keyring-entry'en kan altså aldrig
			// genbruges, og at lade masterpasswordet blive ville være at
			// efterlade en hemmelighed uden noget der peger på den.
			if d.LocalOnly() {
				localSecret = d.LocalID
			}
			continue
		}
		kept = append(kept, d)
	}
	if !found {
		return fmt.Errorf("no local database named %q (use `databases` to list)", name)
	}
	if localSecret != "" {
		if err := keyring.Delete(localSecret); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not remove the masterpassword from the keyring: %v\n", err)
		}
	}

	cfg.Databases = kept
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Forgot local binding %q. The server database and the .kdbx file are untouched.\n", name)
	return nil
}
