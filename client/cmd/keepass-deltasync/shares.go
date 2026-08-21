// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"text/tabwriter"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"os"
)

// runShares implementerer "keepass-deltasync shares <db-name>" — lister
// alle medlemmer (owner + members) af en database. Kun owner kan kalde
// dette endpoint; server returnerer 404 for andre.
func runShares(args []string) error {
	fs := flag.NewFlagSet("shares", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync shares <db-name>")
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
		return errors.New("shares takes exactly 1 argument: <db-name>")
	}
	dbName := fs.Arg(0)

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
	if db.LocalOnly() {
		return errLocalOnly(dbName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := api.New(cfg.Server.URL)

	members, err := client.ListShares(ctx, cfg.Server.DeviceToken, db.RemoteID)
	if err != nil {
		return fmt.Errorf("list shares: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ROLE\tUSERNAME\tDISPLAY NAME\tADDED AT")
	for _, m := range members {
		displayName := ""
		if m.DisplayName != nil {
			displayName = *m.DisplayName
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Role, m.Username, displayName, m.AddedAt)
	}
	return w.Flush()
}
