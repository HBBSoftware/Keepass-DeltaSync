// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// runInit registrerer en lokal .kdbx-fil som en sync-bar database.
//
// Default: opretter en ny database hos serveren via POST /databases og binder
// den lokalt under <name>.
//
// Med --bind <uuid>: opretter INGEN ny server-database, men binder lokalt
// <name> til en eksisterende server-side database. Brug `databases`-kommandoen
// til at finde UUID'en. Dette er det normale 2nd-device-flow.
func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	bind := fs.String("bind", "", "bind to an existing server-side database (UUID from `databases` listing) instead of creating a new one")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync init <name> <local.kdbx> [--bind <remote-uuid>]")
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
		return errors.New("init takes exactly 2 arguments: <name> <local.kdbx>")
	}
	name := fs.Arg(0)
	rawPath := fs.Arg(1)

	if name == "" {
		return errors.New("name must not be empty")
	}

	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", absPath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", absPath)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Server.URL == "" || cfg.Server.DeviceToken == "" {
		return errors.New("not enrolled — run `keepass-deltasync enroll --server <url> <token>` first")
	}
	if cfg.FindDatabase(name) != nil {
		return fmt.Errorf("database %q already exists in local config", name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := api.New(cfg.Server.URL)

	var remoteID, remoteName, createdAt string

	if *bind != "" {
		if !uuidRe.MatchString(*bind) {
			return fmt.Errorf("--bind must be a UUID (got %q). Use `keepass-deltasync databases` to look it up.", *bind)
		}
		remotes, err := client.ListDatabases(ctx, cfg.Server.DeviceToken)
		if err != nil {
			return fmt.Errorf("lookup server databases: %w", err)
		}
		var match *api.Database
		for i := range remotes {
			if remotes[i].ID == *bind {
				match = &remotes[i]
				break
			}
		}
		if match == nil {
			return fmt.Errorf("no server-side database with id %s (visible to this device's user)", *bind)
		}
		remoteID = match.ID
		remoteName = match.Name
		createdAt = match.CreatedAt
	} else {
		db, err := client.CreateDatabase(ctx, cfg.Server.DeviceToken, name)
		if err != nil {
			return err
		}
		remoteID = db.ID
		remoteName = db.Name
		createdAt = db.CreatedAt
	}

	cfg.AddDatabase(config.Database{
		Name:      name,
		LocalPath: absPath,
		RemoteID:  remoteID,
		LastSeq:   0,
	})
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	if *bind != "" {
		fmt.Printf("Bound to existing database.\n")
	} else {
		fmt.Printf("Database registered.\n")
	}
	fmt.Printf("  Local name:   %s\n", name)
	fmt.Printf("  Local path:   %s\n", absPath)
	fmt.Printf("  Remote ID:    %s\n", remoteID)
	fmt.Printf("  Remote name:  %s\n", remoteName)
	fmt.Printf("  Created at:   %s\n", createdAt)
	return nil
}
