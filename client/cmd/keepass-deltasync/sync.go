// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// runSync er den bidirektionelle synkronisering: først pull (server → local
// merge), så push af entries der er ændret siden sidste push (local → server).
//
// Algorithme:
//
//   sync_start  = now()
//   old_push    = config.last_push (parset til time.Time)
//
//   PULL:  GET /changes, decrypt, merge ind i local
//   PUSH:  export local, PUT entries med modified_at > old_push,
//          DELETE tombstones med deletion_time > old_push
//
//   SAVE:  last_seq = pulled current_seq, last_push = sync_start
//
// last_push sættes til sync_start (ikke now()) så enhver entry der bliver
// modificeret DURING sync (fx af en konkurrerende editor) bliver pushet
// næste gang.
//
// Kendt v1-ineffektivitet: entries vi netop har pullet og merget kan have
// modified_at > old_push hvis de stammer fra en device der har pushet
// nyligt. De bliver re-pushet (server laver ny version, ingen data-tab).
// Ved næste sync vil deres modified_at < new last_push og de springes over.
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

	syncStart := time.Now().UTC()

	env, err := setupEnv(name, *pwStdin, *cliPath, 10*time.Minute,
		fmt.Sprintf("Masterpassword for %s: ", name))
	if err != nil {
		return err
	}
	defer env.cleanup()

	// Parse den eksisterende last_push til en time.Time. Tom string betyder
	// "aldrig pushet" — så vi pusher alt (since=nil) første gang.
	var since *time.Time
	if env.db.LastPush != "" {
		t, err := time.Parse(time.RFC3339, env.db.LastPush)
		if err != nil {
			return fmt.Errorf("config last_push is not valid RFC 3339: %w", err)
		}
		since = &t
	}

	// PULL
	newSeq, merged, deletions, err := env.pullChanges()
	if err != nil {
		return err
	}

	// PUSH-DELTA
	pushed, deleted, err := env.pushChanges(since)
	if err != nil {
		return err
	}

	// SAVE — last_seq + last_push opdateres atomarisk via en config-skrivning.
	env.db.LastSeq = newSeq
	env.db.LastPush = syncStart.Format("2006-01-02T15:04:05Z")
	if err := config.Save(env.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Sync complete: pulled %d entries (+ %d tombstones), pushed %d entries (+ %d tombstones). last_seq=%d, last_push=%s\n",
		merged, deletions, pushed, deleted, newSeq, env.db.LastPush)
	return nil
}
