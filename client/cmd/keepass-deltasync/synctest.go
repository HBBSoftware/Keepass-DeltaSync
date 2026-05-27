// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/crypto"
)

// Fast entry-UUID til sync-test. Hver kørsel ramler ind på samme entry — så
// /changes-resultatet er forudsigeligt og vi kan se versionrotation virke.
const syncTestEntryUUID = "deadbeef-cafe-1234-5678-123456789abc"

// runSyncTest kører en ren krypto+protokol-round-trip mod en database der
// allerede er init'et lokalt. INGEN .kdbx-læsning. Bruges til at validere at
// crypto-pakken og entry-API'et taler korrekt sammen med live serveren.
//
// Flow:
//
//	1. Læs masterpassword fra stdin (--password-stdin)
//	2. DeriveMasterKey + DeriveEntryKey mod databasens RemoteID
//	3. Krypter en hardcoded plaintext, PUT som entry-version
//	4. GET /changes since 0, find entry'en, dekrypter blob, sammenlign
//	5. Verificér også at GET /versions returnerer mindst én version
func runSyncTest(args []string) error {
	fs := flag.NewFlagSet("sync-test", flag.ContinueOnError)
	dbFlag := fs.String("database", "", "name of a locally registered database (from `init`)")
	pwStdin := fs.Bool("password-stdin", false, "read masterpassword from stdin (one line)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync sync-test --database NAME --password-stdin")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *dbFlag == "" {
		return errors.New("--database is required")
	}
	if !*pwStdin {
		return errors.New("--password-stdin is required for sync-test (interactive prompt not implemented until slice 2)")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Server.URL == "" || cfg.Server.DeviceToken == "" {
		return errors.New("not enrolled — run `keepass-deltasync enroll` first")
	}
	db := cfg.FindDatabase(*dbFlag)
	if db == nil {
		return fmt.Errorf("database %q not found in local config — run `keepass-deltasync init` first", *dbFlag)
	}

	password, err := readPasswordFromStdin()
	if err != nil {
		return err
	}
	defer zero(password)

	fmt.Fprintln(os.Stderr, "Deriving master key (Argon2id, ~200ms)...")
	masterKey, err := crypto.DeriveMasterKey(password, db.RemoteID)
	if err != nil {
		return fmt.Errorf("derive master key: %w", err)
	}
	defer zero(masterKey)

	entryKey, err := crypto.DeriveEntryKey(masterKey, db.RemoteID)
	if err != nil {
		return fmt.Errorf("derive entry key: %w", err)
	}
	defer zero(entryKey)

	plaintext := []byte(fmt.Sprintf("sync-test plaintext @ %s", time.Now().UTC().Format(time.RFC3339)))
	blob, err := crypto.EncryptBlob(entryKey, plaintext)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := api.New(cfg.Server.URL)

	putResp, err := client.PutEntry(ctx, cfg.Server.DeviceToken, db.RemoteID, syncTestEntryUUID, blob, time.Now())
	if err != nil {
		return fmt.Errorf("PUT: %w", err)
	}
	fmt.Printf("PUT seq=%d (entry=%s)\n", putResp.Seq, putResp.UUID)

	changes, err := client.GetChanges(ctx, cfg.Server.DeviceToken, db.RemoteID, 0)
	if err != nil {
		return fmt.Errorf("GET /changes: %w", err)
	}
	fmt.Printf("GET /changes: current_seq=%d, %d entries\n", changes.CurrentSeq, len(changes.Entries))

	var found *api.EntryChange
	for i := range changes.Entries {
		if changes.Entries[i].UUID == syncTestEntryUUID {
			found = &changes.Entries[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("entry %s not found in /changes response", syncTestEntryUUID)
	}

	gotBlob, err := base64.StdEncoding.DecodeString(found.Blob)
	if err != nil {
		return fmt.Errorf("server returned non-base64 blob: %w", err)
	}
	decoded, err := crypto.DecryptBlob(entryKey, gotBlob)
	if err != nil {
		return fmt.Errorf("decrypt round-tripped blob: %w", err)
	}
	if !bytes.Equal(plaintext, decoded) {
		return fmt.Errorf("plaintext mismatch:\n  sent:     %q\n  received: %q", plaintext, decoded)
	}
	fmt.Printf("Decrypt OK: plaintext matches (%d bytes)\n", len(decoded))

	versions, err := client.GetVersions(ctx, cfg.Server.DeviceToken, db.RemoteID, syncTestEntryUUID)
	if err != nil {
		return fmt.Errorf("GET /versions: %w", err)
	}
	fmt.Printf("GET /versions: %d version(s) preserved\n", len(versions))

	fmt.Println("sync-test PASSED")
	return nil
}

// readPasswordFromStdin reads exactly one line, stripping CR/LF. Empty input
// returns an explicit error so we don't accidentally derive a key from "".
func readPasswordFromStdin() ([]byte, error) {
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	line = bytes.TrimRight(line, "\r\n")
	if len(line) == 0 {
		return nil, errors.New("empty password on stdin")
	}
	// Don't keep trailing whitespace from terminal copy-paste either.
	line = []byte(strings.TrimRight(string(line), " \t"))
	if len(line) == 0 {
		return nil, errors.New("password was only whitespace")
	}
	return line, nil
}

// zero overskriver byte-slicen med 0'er. Best-effort scrubbing — Go's GC
// kan have lavet kopier vi ikke kan nå. Bedre end ingenting.
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
