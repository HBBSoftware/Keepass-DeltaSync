// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/crypto"
	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx"
	"gitlab.com/Star95/keepass-deltasync/client/internal/passwd"
)

// runPull fetcher alle nye entries fra serveren (siden last_seq), dekrypterer
// hver blob, bygger en staging-kdbx, og merger den ind i den lokale .kdbx.
//
// Sikkerhed: før merge tages en backup af local.kdbx (<path>.bak). Hvis
// merge fejler restoreres backup'en, så brugerens data ikke kan blive
// korrupt af en afbrudt merge. Backup'en slettes ved success.
//
// Hvis serveren ikke har noget nyt (eller databasen er tom), eksisterer
// pull stadigt: opdaterer last_seq men rører ikke .kdbx.
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

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Server.URL == "" || cfg.Server.DeviceToken == "" {
		return errors.New("not enrolled — run `keepass-deltasync enroll` first")
	}
	db := cfg.FindDatabase(name)
	if db == nil {
		return fmt.Errorf("database %q not found in local config — run `keepass-deltasync init` first", name)
	}
	if _, err := os.Stat(db.LocalPath); err != nil {
		return fmt.Errorf("local kdbx %s: %w", db.LocalPath, err)
	}

	cli, err := kdbx.NewCLI(*cliPath)
	if err != nil {
		return err
	}

	password, err := passwd.Read(fmt.Sprintf("Masterpassword for %s: ", name), *pwStdin)
	if err != nil {
		return err
	}
	defer passwd.Zero(password)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client := api.New(cfg.Server.URL)

	fmt.Fprintf(os.Stderr, "Fetching changes since seq=%d...\n", db.LastSeq)
	changes, err := client.GetChanges(ctx, cfg.Server.DeviceToken, db.RemoteID, db.LastSeq)
	if err != nil {
		return fmt.Errorf("GET /changes: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Server has current_seq=%d, %d new entries.\n", changes.CurrentSeq, len(changes.Entries))

	// Selv hvis der intet er at merge, vil vi opdatere last_seq —
	// så vi ikke spørger om de samme entries hver gang.
	if len(changes.Entries) == 0 {
		db.LastSeq = changes.CurrentSeq
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		fmt.Println("Pull complete: nothing to merge.")
		return nil
	}

	fmt.Fprintln(os.Stderr, "Deriving master key (Argon2id, ~200ms)...")
	masterKey, err := crypto.DeriveMasterKey(password, db.RemoteID)
	if err != nil {
		return fmt.Errorf("derive master key: %w", err)
	}
	defer passwd.Zero(masterKey)

	entryKey, err := crypto.DeriveEntryKey(masterKey, db.RemoteID)
	if err != nil {
		return fmt.Errorf("derive entry key: %w", err)
	}
	defer passwd.Zero(entryKey)

	// Dekrypter alle entries op-front. Fail loudly på første dekrypterings-
	// fejl med entry-UUID — skip-and-continue er for risikabelt mht. silent
	// data loss.
	var entries []kdbx.StagingEntry
	var deletions []kdbx.StagingDeletion
	for _, e := range changes.Entries {
		blob, err := base64.StdEncoding.DecodeString(e.Blob)
		if err != nil {
			return fmt.Errorf("entry %s: server blob not valid base64: %w", e.UUID, err)
		}
		modAt, err := time.Parse(time.RFC3339, e.ModifiedAt)
		if err != nil {
			return fmt.Errorf("entry %s: parse modified_at: %w", e.UUID, err)
		}
		if e.Deleted {
			// Tombstone — serveren sendte måske en blob (tom eller gammel),
			// men vi sender den ikke videre til merge. KeePassXC's merge
			// bruger <DeletedObjects> til at slette i target.
			deletions = append(deletions, kdbx.StagingDeletion{
				UUID:      e.UUID,
				DeletedAt: modAt,
			})
			continue
		}
		fragment, err := crypto.DecryptBlob(entryKey, blob)
		if err != nil {
			return fmt.Errorf("entry %s: decrypt failed — wrong masterpassword? %w", e.UUID, err)
		}
		entries = append(entries, kdbx.StagingEntry{
			UUID:       e.UUID,
			Fragment:   fragment,
			ModifiedAt: modAt,
		})
	}
	fmt.Fprintf(os.Stderr, "Decrypted %d entries (+ %d tombstones).\n", len(entries), len(deletions))

	// Byg staging XML
	stagingXML, err := kdbx.BuildStagingXML(entries, deletions)
	if err != nil {
		return fmt.Errorf("build staging xml: %w", err)
	}

	// Skriv til midlertidige filer
	tmpDir, err := os.MkdirTemp("", "kdsync-pull-*")
	if err != nil {
		return fmt.Errorf("mkdir temp: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpXML := filepath.Join(tmpDir, "staging.xml")
	tmpKDBX := filepath.Join(tmpDir, "staging.kdbx")
	if err := os.WriteFile(tmpXML, stagingXML, 0o600); err != nil {
		return fmt.Errorf("write staging xml: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Building staging kdbx...")
	if err := cli.Import(ctx, tmpXML, tmpKDBX, password); err != nil {
		return err
	}

	// Backup local.kdbx → .bak
	backupPath := db.LocalPath + ".bak"
	if err := copyFile(db.LocalPath, backupPath); err != nil {
		return fmt.Errorf("backup local kdbx: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Merging staging into local kdbx...")
	if err := cli.Merge(ctx, db.LocalPath, tmpKDBX, password); err != nil {
		// Restore from backup so user's data isn't left in a half-merged state.
		if restoreErr := copyFile(backupPath, db.LocalPath); restoreErr != nil {
			return fmt.Errorf("merge failed AND restore failed (backup at %s): merge=%v, restore=%w", backupPath, err, restoreErr)
		}
		return fmt.Errorf("merge failed (local restored from backup): %w", err)
	}

	// Slet backup ved success
	if err := os.Remove(backupPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not delete backup %s: %v\n", backupPath, err)
	}

	// Opdatér last_seq
	db.LastSeq = changes.CurrentSeq
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Pull complete: %d entries merged, %d tombstones applied. last_seq=%d\n", len(entries), len(deletions), changes.CurrentSeq)
	return nil
}

// copyFile is a minimal helper for backup/restore. Not atomic — but we only
// use it for backup before a merge, where atomicity isn't required.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
