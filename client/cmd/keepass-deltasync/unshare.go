// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// runUnshare implementerer "keepass-deltasync unshare <db-name> <username>".
//
// Owner kan fjerne en hvilken som helst member. Member kan fjerne sig selv
// (= forlade en delt database) ved at angive sit eget username.
//
// Selve unshare-handlingen kryptografisk: server sletter database_members-row.
// Brugerens lokale .kdbx-fil bevares — sletning af lokale data er brugerens
// eget valg.
func runUnshare(args []string) error {
	fs := flag.NewFlagSet("unshare", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync unshare <db-name> <username>")
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
		return errors.New("unshare takes exactly 2 arguments: <db-name> <username>")
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
		return fmt.Errorf("database %q not found in local config", dbName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := api.New(cfg.Server.URL)

	lookup, err := client.LookupUser(ctx, cfg.Server.DeviceToken, username)
	if err != nil {
		return fmt.Errorf("lookup user %q: %w", username, err)
	}

	if err := client.UnshareDatabase(ctx, cfg.Server.DeviceToken, db.RemoteID, lookup.User.ID); err != nil {
		return fmt.Errorf("unshare: %w", err)
	}

	fmt.Printf("Removed %s from %q.\n", lookup.User.Username, dbName)
	return nil
}
