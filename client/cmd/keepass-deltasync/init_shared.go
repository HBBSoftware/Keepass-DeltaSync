// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/crypto"
	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx"
	"gitlab.com/Star95/keepass-deltasync/client/internal/passwd"
)

// runInitShared bootstrapper Bob's lokale .kdbx fra en shared database.
//
// Forudsætning: Alice har kørt `share <db> <bob's-username>` så
// database_members har en row med wrapped_master_key til Bob's enheds
// public_key. Bob har kørt en hvilken som helst klient-kommando der har
// auto-upgraded hans device-keypair.
//
// Flow:
//
//  1. Find shared db på server (matchet på remote-name + role=member)
//  2. Unwrap master_key med Bob's device-private-key
//  3. Hent alle entries via GetChanges(since=0)
//  4. Dekryptér med entry_key
//  5. Prompt Bob for et nyt lokalt password til hans .kdbx
//  6. Byg staging-XML + kør keepassxc-cli import → ny lokal .kdbx
//  7. Skriv config-binding så efterfølgende sync/pull/push fungerer
func runInitShared(args []string) error {
	fs := flag.NewFlagSet("init-shared", flag.ContinueOnError)
	asName := fs.String("as", "", "lokalt alias for databasen (default: samme som remote name)")
	cliPath := fs.String("keepassxc-cli", "", "path til keepassxc-cli")
	pwStdin := fs.Bool("password-stdin", false, "læs nyt lokalt password fra stdin")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync init-shared <remote-name> <local-path> [--as <local-name>] [--password-stdin] [--keepassxc-cli PATH]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return errors.New("init-shared takes exactly 2 arguments: <remote-name> <local-path>")
	}
	remoteName := fs.Arg(0)
	rawPath := fs.Arg(1)

	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	if _, err := os.Stat(absPath); err == nil {
		return fmt.Errorf("%s already exists; choose a different path or remove it first", absPath)
	}

	localName := *asName
	if localName == "" {
		localName = remoteName
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Server.URL == "" || cfg.Server.DeviceToken == "" {
		return errors.New("not enrolled — run `keepass-deltasync enroll` first")
	}
	if cfg.FindDatabase(localName) != nil {
		return fmt.Errorf("database %q already exists in local config — use --as to choose a different alias", localName)
	}

	cli, err := kdbx.NewCLI(*cliPath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client := api.New(cfg.Server.URL)
	if upErr := ensureDevicePublicKey(ctx, cfg, client); upErr != nil {
		return fmt.Errorf("auto-upgrade keypair: %w", upErr)
	}

	shared, err := findSharedDatabaseByName(ctx, client, cfg.Server.DeviceToken, remoteName)
	if err != nil {
		return err
	}

	// Unwrap master_key med Bob's device-keypair. Hvis det fejler kan
	// ingen entries dekrypteres — vi vil hellere fejle her end midtvejs.
	if shared.WrappedMasterKey == nil {
		return errors.New("shared database has no wrapped_master_key — owner must (re-)share")
	}
	if len(cfg.Server.DevicePrivateKey) == 0 {
		return errors.New("device has no private key — re-enroll")
	}
	wrapped, err := base64.StdEncoding.DecodeString(*shared.WrappedMasterKey)
	if err != nil {
		return fmt.Errorf("decode wrapped key: %w", err)
	}
	pub, err := crypto.PublicKeyFromPrivate(cfg.Server.DevicePrivateKey)
	if err != nil {
		return fmt.Errorf("derive device public key: %w", err)
	}
	masterKey, err := crypto.UnwrapKey(wrapped, pub, cfg.Server.DevicePrivateKey)
	if err != nil {
		return fmt.Errorf("unwrap master key: %w", err)
	}
	defer passwd.Zero(masterKey)

	entryKey, err := crypto.DeriveEntryKey(masterKey, shared.ID)
	if err != nil {
		return fmt.Errorf("derive entry key: %w", err)
	}
	defer passwd.Zero(entryKey)

	fmt.Fprintln(os.Stderr, "Fetching entries from server...")
	changes, err := client.GetChanges(ctx, cfg.Server.DeviceToken, shared.ID, 0, false)
	if err != nil {
		return fmt.Errorf("get changes: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Server has %d entries (current_seq=%d).\n", len(changes.Entries), changes.CurrentSeq)

	password, err := passwd.Read(fmt.Sprintf("Choose a new local password for %s: ", localName), *pwStdin)
	if err != nil {
		return err
	}
	defer passwd.Zero(password)

	entries, deletions, entryStates, err := decryptChangesForImport(changes, entryKey)
	if err != nil {
		return err
	}

	stagingXML, err := kdbx.BuildStagingXML(entries, deletions, "")
	if err != nil {
		return fmt.Errorf("build staging xml: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "kdsync-initshared-*")
	if err != nil {
		return fmt.Errorf("mkdir temp: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpXML := filepath.Join(tmpDir, "staging.xml")
	if err := os.WriteFile(tmpXML, stagingXML, 0o600); err != nil {
		return fmt.Errorf("write staging xml: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o700); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Creating local kdbx...")
	if err := cli.Import(ctx, tmpXML, absPath, password); err != nil {
		return fmt.Errorf("import to %s: %w", absPath, err)
	}

	// Skriv config-binding. last_seq sættes til current_seq fordi vi netop
	// har pulled alt. entry-states sættes så next sync ikke re-pusher det
	// vi netop modtog.
	newDB := config.Database{
		Name:        localName,
		LocalPath:   absPath,
		RemoteID:    shared.ID,
		LastSeq:     changes.CurrentSeq,
		EntryStates: entryStates,
	}
	cfg.AddDatabase(newDB)
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Shared database initialized.\n")
	fmt.Printf("  Local name:   %s\n", localName)
	fmt.Printf("  Local path:   %s\n", absPath)
	fmt.Printf("  Remote ID:    %s\n", shared.ID)
	fmt.Printf("  Remote name:  %s\n", shared.Name)
	fmt.Printf("  Entries:      %d\n", len(entries))
	fmt.Printf("  Tombstones:   %d\n", len(deletions))
	fmt.Printf("  last_seq:     %d\n", changes.CurrentSeq)
	fmt.Println("Open it in KeePassXC and the entries appear under the 'deltasync' group; move them where you want.")
	return nil
}

// findSharedDatabaseByName finder den shared database (role=member) hos
// serveren der matcher det givne navn. Hvis flere matcher: tag den første
// (sjælden edge case). Hvis ingen matcher: klar fejl.
func findSharedDatabaseByName(ctx context.Context, client *api.Client, deviceToken, name string) (*api.Database, error) {
	dbs, err := client.ListDatabases(ctx, deviceToken)
	if err != nil {
		return nil, fmt.Errorf("list databases: %w", err)
	}
	for i := range dbs {
		if dbs[i].Name == name && dbs[i].Role == "member" {
			return &dbs[i], nil
		}
	}
	return nil, fmt.Errorf("no shared database with name %q found (must have role=member); use `databases` to verify what's available", name)
}

// decryptChangesForImport dekrypterer alle entries i en ChangesResponse til
// kdbx.StagingEntry/StagingDeletion, og bygger samtidig en entry-states-map
// som kan skrives i config så next sync ikke re-pusher dem.
func decryptChangesForImport(changes *api.ChangesResponse, entryKey []byte) ([]kdbx.StagingEntry, []kdbx.StagingDeletion, map[string]string, error) {
	var entries []kdbx.StagingEntry
	var deletions []kdbx.StagingDeletion
	entryStates := make(map[string]string, len(changes.Entries))

	for _, c := range changes.Entries {
		blob, err := base64.StdEncoding.DecodeString(c.Blob)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("entry %s: decode blob: %w", c.UUID, err)
		}
		modAt, err := time.Parse(time.RFC3339, c.ModifiedAt)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("entry %s: parse modified_at: %w", c.UUID, err)
		}
		entryStates[c.UUID] = modAt.UTC().Format("2006-01-02T15:04:05Z")
		if c.Deleted {
			deletions = append(deletions, kdbx.StagingDeletion{UUID: c.UUID, DeletedAt: modAt})
			continue
		}
		fragment, err := decryptToFragment(entryKey, blob, c.UUID, modAt)
		if err != nil {
			return nil, nil, nil, err
		}
		entries = append(entries, kdbx.StagingEntry{
			UUID:       c.UUID,
			Fragment:   fragment,
			ModifiedAt: modAt,
		})
	}
	return entries, deletions, entryStates, nil
}
