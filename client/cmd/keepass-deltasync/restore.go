// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// runRestore beder serveren om at promovere en gammel version af en entry
// som ny nyeste version. Kun server-side handling — den lokale .kdbx rør
// vi ikke. Brugeren skal selv køre `sync` bagefter for at hente den
// restored version ned og merge ind i lokal kdbx.
//
// Spec: POST /databases/{id}/entries/{uuid}/restore/{num} kopierer
// version {num}'s blob ind som ny version_num=3 med server's now() som
// modified_at (så den vinder under KeePassXC's merge på klienter).
func runRestore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync restore <name> <entry-uuid> <version-num>")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 3 {
		fs.Usage()
		return errors.New("restore takes exactly 3 arguments: <name> <entry-uuid> <version-num>")
	}
	name := fs.Arg(0)
	entryUUID := fs.Arg(1)
	versionArg := fs.Arg(2)

	if !uuidRe.MatchString(entryUUID) {
		return fmt.Errorf("entry-uuid must be a UUID (got %q)", entryUUID)
	}
	versionNum, err := strconv.Atoi(versionArg)
	if err != nil || versionNum < 1 || versionNum > 3 {
		return fmt.Errorf("version-num must be 1, 2 or 3 (got %q)", versionArg)
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := api.New(cfg.Server.URL).RestoreVersion(ctx, cfg.Server.DeviceToken, db.RemoteID, entryUUID, versionNum)
	if err != nil {
		return err
	}

	fmt.Printf("Restored version %d as new current version.\n", resp.RestoredFrom)
	fmt.Printf("  Entry UUID:    %s\n", resp.UUID)
	fmt.Printf("  New seq:       %d\n", resp.Seq)
	fmt.Printf("  Modified at:   %s\n", resp.ModifiedAt)
	if resp.Deleted {
		fmt.Println("  Note: restored version is a tombstone — entry will be deleted on next sync")
	}
	fmt.Printf("\nRun `keepass-deltasync sync %s` to fetch the restored version into your local .kdbx.\n", name)
	return nil
}
