// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/crypto"
	"gitlab.com/Star95/keepass-deltasync/client/internal/passwd"
)

// runShare implementerer "keepass-deltasync share <db-name> <username>".
//
// Alice deriverer master_key fra masterpassword (Argon2id), sluser Bob's
// device-public-key op, wrap'er master_key som sealed-box, og POST'er til
// /shares. Server lagrer den opaque sealed-box; den kan kun unwrappes af
// Bob's enheds private-key.
func runShare(args []string) error {
	fs := flag.NewFlagSet("share", flag.ContinueOnError)
	pwStdin := fs.Bool("password-stdin", false, "read masterpassword from stdin instead of interactive prompt")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync share <db-name> <username> [--password-stdin]")
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
		return errors.New("share takes exactly 2 arguments: <db-name> <username>")
	}
	dbName := fs.Arg(0)
	username := fs.Arg(1)

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Server.URL == "" || cfg.Server.DeviceToken == "" {
		return errors.New("not enrolled — run `keepass-deltasync enroll` first")
	}
	db := cfg.FindDatabase(dbName)
	if db == nil {
		return fmt.Errorf("database %q not found in local config — run `keepass-deltasync init` first", dbName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := api.New(cfg.Server.URL)

	// Auto-upgrade keypair hvis legacy. Vi har ikke selv brug for vores
	// public_key her, men vi vil have det op på serveren før vi forsøger
	// nogen sharing-handlinger.
	if upErr := ensureDevicePublicKey(ctx, cfg, client); upErr != nil {
		fmt.Fprintf(os.Stderr, "warning: auto-upgrade of device keypair failed (%v) — continuing anyway\n", upErr)
	}

	// Verificér at vi er owner. Server håndhæver det også, men vi får en
	// pænere fejl her end "404 not found".
	role, err := fetchRole(ctx, client, cfg.Server.DeviceToken, db.RemoteID)
	if err != nil {
		return err
	}
	if role != "owner" {
		return fmt.Errorf("cannot share %q: you are %s, not owner", dbName, role)
	}

	// Slå Bob op + få hans target-device public-key.
	lookup, err := client.LookupUser(ctx, cfg.Server.DeviceToken, username)
	if err != nil {
		return fmt.Errorf("lookup user %q: %w", username, err)
	}
	targetPub, err := base64.StdEncoding.DecodeString(lookup.TargetDevice.PublicKey)
	if err != nil {
		return fmt.Errorf("decode target public key: %w", err)
	}
	if len(targetPub) != crypto.BoxPublicKeySize {
		return fmt.Errorf("target public key has unexpected size: %d", len(targetPub))
	}

	// Deriver master_key fra masterpassword. Argon2id ~200ms.
	password, err := passwd.Read(fmt.Sprintf("Masterpassword for %s: ", dbName), *pwStdin)
	if err != nil {
		return err
	}
	defer passwd.Zero(password)

	fmt.Fprintln(os.Stderr, "Deriving master key (Argon2id, ~200ms)...")
	masterKey, err := crypto.DeriveMasterKey(password, db.RemoteID)
	if err != nil {
		return fmt.Errorf("derive master key: %w", err)
	}
	defer passwd.Zero(masterKey)

	wrapped, err := crypto.WrapKey(masterKey, targetPub)
	if err != nil {
		return fmt.Errorf("wrap master key: %w", err)
	}

	if err := client.ShareDatabase(ctx, cfg.Server.DeviceToken, db.RemoteID, lookup.User.ID, wrapped); err != nil {
		return fmt.Errorf("share database: %w", err)
	}

	fmt.Printf("Shared %q with %s (device: %s).\n", dbName, lookup.User.Username, lookup.TargetDevice.Name)
	fmt.Println("They can now run `databases` to see the shared database, then `init-shared` (coming in M4) to bootstrap a local copy.")
	return nil
}

// fetchRole returnerer caller'ens rolle ('owner' / 'member') for en given
// database ved at slå op i GET /databases. Hvis databasen ikke er listet,
// returneres en klar fejl. Bruges af share/unshare/shares-kommandoerne til
// klient-side rolle-tjek før det dyre Argon2id-arbejde sættes i gang.
func fetchRole(ctx context.Context, client *api.Client, deviceToken, databaseID string) (string, error) {
	dbs, err := client.ListDatabases(ctx, deviceToken)
	if err != nil {
		return "", fmt.Errorf("list databases: %w", err)
	}
	for _, d := range dbs {
		if d.ID == databaseID {
			if d.Role == "" {
				// Pre-v2 server returnerer ingen role. Antag owner.
				return "owner", nil
			}
			return d.Role, nil
		}
	}
	return "", fmt.Errorf("database %s not found on server (deleted, or you lost access)", databaseID)
}
