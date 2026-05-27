// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/crypto"
	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx"
	"gitlab.com/Star95/keepass-deltasync/client/internal/passwd"
)

// runPush eksporterer hele den lokale .kdbx via keepassxc-cli, krypterer hver
// entry separat, og uploader dem som nye versioner. DeletedObjects i
// .kdbx-filen sendes som DELETE-calls til serveren.
//
// V1 push-strategi: alle entries hver gang. Hver kørsel skaber nye versioner
// på serveren — enklere end inkrementel push og umuligt at få desync.
func runPush(args []string) error {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	pwStdin := fs.Bool("password-stdin", false, "read masterpassword from stdin instead of interactive prompt")
	cliPath := fs.String("keepassxc-cli", "", "path to keepassxc-cli binary (overrides auto-detection and $KEEPASSXC_CLI)")
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

	fmt.Fprintln(os.Stderr, "Exporting kdbx via keepassxc-cli...")
	xmlBytes, err := cli.Export(ctx, db.LocalPath, password)
	if err != nil {
		return err
	}

	entries, deletions, err := kdbx.ParseExport(xmlBytes)
	if err != nil {
		return fmt.Errorf("parse export: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Found %d entries, %d deletions in kdbx.\n", len(entries), len(deletions))

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

	client := api.New(cfg.Server.URL)

	// PUT alle entries
	pushed := 0
	for _, e := range entries {
		blob, err := crypto.EncryptBlob(entryKey, e.Fragment)
		if err != nil {
			return fmt.Errorf("encrypt entry %s: %w", e.UUID, err)
		}
		if _, err := client.PutEntry(ctx, cfg.Server.DeviceToken, db.RemoteID, e.UUID, blob, e.ModifiedAt); err != nil {
			return fmt.Errorf("PUT entry %s: %w", e.UUID, err)
		}
		pushed++
	}

	// DELETE alle tombstones
	deleted := 0
	for _, d := range deletions {
		if _, err := client.DeleteEntry(ctx, cfg.Server.DeviceToken, db.RemoteID, d.UUID, nil, d.DeletedAt); err != nil {
			return fmt.Errorf("DELETE entry %s: %w", d.UUID, err)
		}
		deleted++
	}

	// Opdatér last_push i config
	db.LastPush = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Push complete: %d entries uploaded, %d tombstones sent.\n", pushed, deleted)
	return nil
}
