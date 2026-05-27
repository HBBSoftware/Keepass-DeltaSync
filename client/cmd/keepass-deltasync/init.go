// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// runInit registrerer en lokal .kdbx-fil som en sync-bar database hos serveren.
// Resultatet: et nyt [[database]]-block i config.toml der binder lokalt path
// til server-genereret UUID. Filen åbnes/parses ikke — kun stat'es. Crypto +
// .kdbx-håndtering kommer i sync-skiven.
func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync init <name> <local.kdbx>")
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

	// Resolvér til absolut sti så config er entydigt uafhængigt af CWD.
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	// Stat-check: filen skal eksistere og være en almindelig fil.
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

	db, err := api.New(cfg.Server.URL).CreateDatabase(ctx, cfg.Server.DeviceToken, name)
	if err != nil {
		return err
	}

	cfg.AddDatabase(config.Database{
		Name:      name,
		LocalPath: absPath,
		RemoteID:  db.ID,
		LastSeq:   0,
	})
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Database registered.\n")
	fmt.Printf("  Name:        %s\n", name)
	fmt.Printf("  Local path:  %s\n", absPath)
	fmt.Printf("  Remote ID:   %s\n", db.ID)
	fmt.Printf("  Created at:  %s\n", db.CreatedAt)
	return nil
}
