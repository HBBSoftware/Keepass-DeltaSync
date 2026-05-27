// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// runPush eksporterer den lokale .kdbx og uploader ALLE entries til serveren
// (ingen since-filter — det er det der adskiller `push` fra `sync`). Bruges
// til initial-sync af en eksisterende kdbx eller til force-resend efter
// servertab.
func runPush(args []string) error {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	pwStdin := fs.Bool("password-stdin", false, "read masterpassword from stdin instead of interactive prompt")
	cliPath := fs.String("keepassxc-cli", "", "path to keepassxc-cli binary (overrides auto-detection)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync push <name> [--password-stdin] [--keepassxc-cli PATH]")
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

	pushed, deleted, err := env.pushChanges(nil)
	if err != nil {
		return err
	}

	env.db.LastPush = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	if err := config.Save(env.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Push complete: %d entries uploaded, %d tombstones sent.\n", pushed, deleted)
	return nil
}
