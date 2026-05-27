// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// runPull fetcher alle nye entries fra serveren (siden last_seq) og merger
// ind i den lokale .kdbx via syncop.runEnv.pullChanges. Opdaterer last_seq
// efter succes — ikke last_push, da pull ikke pusher noget.
func runPull(args []string) error {
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	pwStdin := fs.Bool("password-stdin", false, "read masterpassword from stdin instead of interactive prompt")
	cliPath := fs.String("keepassxc-cli", "", "path to keepassxc-cli binary (overrides auto-detection)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync pull <name> [--password-stdin] [--keepassxc-cli PATH]")
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
		return errors.New("pull takes exactly 1 argument: <name>")
	}
	name := fs.Arg(0)

	env, err := setupEnv(name, *pwStdin, *cliPath, 5*time.Minute,
		fmt.Sprintf("Masterpassword for %s: ", name))
	if err != nil {
		return err
	}
	defer env.cleanup()

	newSeq, merged, deletions, err := env.pullChanges()
	if err != nil {
		return err
	}

	// pullChanges har allerede opdateret env.db.EntryStates internt; vi
	// gemmer config én gang her uanset om der var noget at merge.
	env.db.LastSeq = newSeq
	if err := config.Save(env.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	if merged == 0 && deletions == 0 {
		fmt.Println("Pull complete: nothing to merge.")
	} else {
		fmt.Printf("Pull complete: %d entries merged, %d tombstones applied. last_seq=%d\n", merged, deletions, newSeq)
	}
	return nil
}
