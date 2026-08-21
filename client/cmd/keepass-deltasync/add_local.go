// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx"
	"gitlab.com/Star95/keepass-deltasync/client/internal/keyring"
	"gitlab.com/Star95/keepass-deltasync/client/internal/passwd"
)

// runAddLocal registrerer en .kdbx UDEN at der er en server involveret.
//
// Det er søge-kun-vejen: browser-hosten (og dermed Firefox-udvidelsen) læser
// sine databaser fra config, men den eneste vej ind i config'en var indtil nu
// `init`, som kræver enrollment. En bruger der bare vil kunne søge i sin egen
// lokale database havde altså ingen understøttet vej — kun håndredigering af
// config.toml.
//
// Bindingen får INTET remote_id. Alt der taler med serveren tjekker
// Database.LocalOnly() og afviser sådan en binding frem for at sende en tom
// UUID afsted. Vil man senere synkronisere den, er vejen `forget` + `init`.
func runAddLocal(args []string) error {
	fs := flag.NewFlagSet("add-local", flag.ContinueOnError)
	savePassword := fs.Bool("save-password", false, "store the masterpassword in the OS keyring, so searching does not prompt")
	pwStdin := fs.Bool("password-stdin", false, "read the masterpassword from stdin instead of the terminal (implies --save-password)")
	cliPath := fs.String("keepassxc-cli", "", "path to keepassxc-cli (default: $KEEPASSXC_CLI, then PATH)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync add-local <name> <local.kdbx> [--save-password]")
		fmt.Fprintln(fs.Output(), "\nRegisters a .kdbx for local search only — no server, no enrollment, no sync.")
		fmt.Fprintln(fs.Output(), "This is what the Firefox extension needs in order to see the database.")
		fs.PrintDefaults()
	}
	// `add-local mydb ~/db.kdbx --save-password` er den rækkefølge folk
	// skriver, og flag stopper ved første positional. Samme behandling som
	// delete-database giver sine flag.
	args = rearrangeFlagsFirst(args, map[string]bool{"save-password": true, "password-stdin": true})
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return errors.New("add-local takes exactly 2 arguments: <name> <local.kdbx>")
	}
	name := fs.Arg(0)
	if name == "" {
		return errors.New("name must not be empty")
	}

	absPath, err := filepath.Abs(fs.Arg(1))
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", absPath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", absPath)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.FindDatabase(name) != nil {
		return fmt.Errorf("database %q already exists in local config", name)
	}

	// keepassxc-cli tjekkes FØR vi skriver noget. Uden den kan hverken hosten
	// eller vi selv åbne databasen, og en binding der ikke kan bruges er værre
	// end en fejl her.
	cli, err := kdbx.NewCLI(*cliPath)
	if err != nil {
		return err
	}

	localID, err := newLocalID()
	if err != nil {
		return err
	}

	if *pwStdin {
		*savePassword = true
	}
	if *savePassword {
		password, err := passwd.Read(fmt.Sprintf("Masterpassword for %s: ", name), *pwStdin)
		if err != nil {
			return err
		}
		defer passwd.Zero(password)

		// Verificér før vi gemmer. Et forkert password i keyringen giver en
		// fejl først næste gang udvidelsen søger, langt fra det sted hvor det
		// blev tastet.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if _, err := cli.Export(ctx, absPath, password); err != nil {
			return fmt.Errorf("open %s: %w", absPath, err)
		}
		if err := keyring.Set(localID, password); err != nil {
			return fmt.Errorf("store masterpassword in keyring: %w", err)
		}
	}

	cfg.AddDatabase(config.Database{
		Name:      name,
		LocalPath: absPath,
		LocalID:   localID,
	})
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Registered for local search only.\n")
	fmt.Printf("  Local name:     %s\n", name)
	fmt.Printf("  Local path:     %s\n", absPath)
	if *savePassword {
		fmt.Printf("  Masterpassword: stored in the OS keyring\n")
	} else {
		fmt.Printf("  Masterpassword: not stored — the extension will ask for it\n")
	}
	fmt.Printf("\nThis database is NOT synced. Run `keepass-deltasync install-browser-host`\n")
	fmt.Printf("and restart Firefox if you have not already done so.\n")
	return nil
}

// errLocalOnly er svaret når en kommando der har brug for serveren rammer en
// binding fra `add-local`. Den skal navngive vejen videre: uden det ligner det
// bare en database der ikke virker.
func errLocalOnly(name string) error {
	return fmt.Errorf("database %q is registered for local search only (`add-local`) — there is no server-side database to sync with. To sync it: `keepass-deltasync forget %s`, then `init`", name, name)
}

// newLocalID genererer et UUIDv4 til keyring-nøglen. Formatet er valgt så en
// lokal binding ser ud som enhver anden i keyringen og i config'en — der er
// ingen grund til at kunne kende dem fra hinanden dér.
func newLocalID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate local id: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
