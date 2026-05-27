// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// runSync er den bidirektionelle synkronisering: pull (server → local merge),
// så push af de entries der har ændret sig siden vi sidst pushede/pullede
// dem.
//
// Push-delta-filteret er pr.-entry via db.EntryStates: hver entry pushes kun
// hvis dens mtime er nyere end den senest sete værdi for samme UUID. Dette
// erstatter det tidligere globale last_push-filter, som havde to subtle bugs:
// (1) entries pullet i samme sync kunne re-pushes; (2) edits i samme sekund
// som forrige sync kunne mistes pga. sekund-præcisions-sammenligning.
func runSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	pwStdin := fs.Bool("password-stdin", false, "read masterpassword from stdin instead of interactive prompt")
	cliPath := fs.String("keepassxc-cli", "", "path to keepassxc-cli binary (overrides auto-detection)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync sync <name> [--password-stdin] [--keepassxc-cli PATH]")
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
		return errors.New("sync takes exactly 1 argument: <name>")
	}
	name := fs.Arg(0)

	env, err := setupEnv(name, *pwStdin, *cliPath, 10*time.Minute,
		fmt.Sprintf("Masterpassword for %s: ", name))
	if err != nil {
		return err
	}
	defer env.cleanup()

	newSeq, merged, deletions, err := env.pullChanges()
	if err != nil {
		return err
	}

	pushed, deleted, maxPushSeq, err := env.pushChanges(false)
	if err != nil {
		return err
	}

	// Avancér last_seq forbi det vi netop har pushet, så næste sync's pull
	// ikke fetcher vores egne entries tilbage.
	if maxPushSeq > newSeq {
		newSeq = maxPushSeq
	}
	env.db.LastSeq = newSeq
	if err := config.Save(env.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Sync complete: pulled %d entries (+ %d tombstones), pushed %d entries (+ %d tombstones). last_seq=%d\n",
		merged, deletions, pushed, deleted, newSeq)
	return nil
}
