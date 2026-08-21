// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// runVersions lister alle bevarede versioner (op til 3) af en entry.
// Ingen masterpassword nødvendigt — vi viser kun metadata, ikke blob-indhold.
// Nyeste version har version_num=3 og betegnes "current". STATE-kolonnen
// markerer også "deleted" hvis versionen er en tombstone.
func runVersions(args []string) error {
	fs := flag.NewFlagSet("versions", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync versions <name> <entry-uuid>")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return errors.New("versions takes exactly 2 arguments: <name> <entry-uuid>")
	}
	name := fs.Arg(0)
	entryUUID := fs.Arg(1)

	if !uuidRe.MatchString(entryUUID) {
		return fmt.Errorf("entry-uuid must be a UUID (got %q)", entryUUID)
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
	if db.LocalOnly() {
		return errLocalOnly(name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	versions, err := api.New(cfg.Server.URL).GetVersions(ctx, cfg.Server.DeviceToken, db.RemoteID, entryUUID)
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		fmt.Println("(no versions — entry not found)")
		return nil
	}

	// Server returnerer nyeste først; sortér for at være sikker.
	sort.SliceStable(versions, func(i, j int) bool {
		return versions[i].VersionNum > versions[j].VersionNum
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "VER\tSTATE\tMODIFIED\tCREATED")
	for _, v := range versions {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
			v.VersionNum,
			versionState(v.VersionNum, v.Deleted),
			v.ModifiedAt,
			v.CreatedAt,
		)
	}
	return w.Flush()
}

// versionState mapper version_num + deleted-flag til en menneskeligt
// læsbar label. version_num=3 er altid den nyeste (eller eneste, hvis
// entry'en kun har fået en enkelt PUT).
func versionState(versionNum int, deleted bool) string {
	state := "current"
	switch versionNum {
	case 2:
		state = "previous"
	case 1:
		state = "oldest"
	}
	if deleted {
		state += " (deleted)"
	}
	return state
}
