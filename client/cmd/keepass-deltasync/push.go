// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// runPush eksporterer den lokale .kdbx og uploader entries til serveren.
// Default: skipper entries der allerede er i db.EntryStates med matchende
// mtime — samme delta-semantik som `sync`s push-fase. Med --force pushes
// ALT uanset tracking-state (initial-sync, recovery efter serverforlis,
// eller en mistanke om at server-state er korrupt).
//
// `push` adskiller sig fra `sync` ved ikke at pulle først — hvis du har
// lokale ændringer som du vil have op nu og pull kan vente, brug `push`.
func runPush(args []string) error {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	pwStdin := fs.Bool("password-stdin", false, "read masterpassword from stdin instead of interactive prompt")
	cliPath := fs.String("keepassxc-cli", "", "path to keepassxc-cli binary (overrides auto-detection)")
	force := fs.Bool("force", false, "push every entry regardless of local tracking state")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync push <name> [--force] [--password-stdin] [--keepassxc-cli PATH]")
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
		return errors.New("push takes exactly 1 argument: <name>")
	}
	name := fs.Arg(0)

	env, err := setupEnv(name, *pwStdin, *cliPath, 5*time.Minute,
		fmt.Sprintf("Masterpassword for %s: ", name))
	if err != nil {
		return err
	}
	defer env.cleanup()

	pushed, deleted, err := env.pushChanges(*force)
	if err != nil {
		return err
	}

	if err := config.Save(env.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Push complete: %d entries uploaded, %d tombstones sent.\n", pushed, deleted)
	return nil
}
