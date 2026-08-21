// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// runDatabases lister databaser server-side og krydsrefererer med lokal config.
// Markører:
//
//	*  registreret både hos serveren OG i lokal config — klar til sync
//	?  findes hos serveren men ikke i lokal config — bruger skal binde via init
//	   (eller: hører til en anden enhed på samme konto)
//	L  kun lokal (`add-local`) — søgbar fra Firefox-udvidelsen, synkes ikke
//
// Uden enrollment fejler kommandoen IKKE: en bruger der kun har lokal-kun
// databaser skal stadig kunne se hvad der er registreret.
func runDatabases(args []string) error {
	fs := flag.NewFlagSet("databases", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync databases")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return errors.New("databases takes no arguments")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Server.URL == "" || cfg.Server.DeviceToken == "" {
		if !hasLocalOnly(cfg) {
			return errors.New("not enrolled — run `keepass-deltasync enroll --server <url> <token>` first")
		}
		fmt.Println("Not enrolled — showing local-only databases.")
		return printLocalOnly(cfg)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	remote, err := api.New(cfg.Server.URL).ListDatabases(ctx, cfg.Server.DeviceToken)
	if err != nil {
		return err
	}

	// Indekser lokal binding på RemoteID for hurtigt opslag.
	local := make(map[string]*config.Database, len(cfg.Databases))
	for i := range cfg.Databases {
		if cfg.Databases[i].LocalOnly() {
			continue
		}
		local[cfg.Databases[i].RemoteID] = &cfg.Databases[i]
	}

	if len(remote) == 0 {
		fmt.Println("(no databases registered for this user)")
		if hasLocalOnly(cfg) {
			return printLocalOnly(cfg)
		}
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  \tNAME\tID\tCREATED\tLOCAL PATH")
	for _, r := range remote {
		marker := "?"
		path := "(not bound locally)"
		if l := local[r.ID]; l != nil {
			marker = "*"
			path = l.LocalPath
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			marker, r.Name, r.ID, r.CreatedAt, path)
	}
	for i := range cfg.Databases {
		if cfg.Databases[i].LocalOnly() {
			fmt.Fprintf(w, "L\t%s\t(local only)\t\t%s\n", cfg.Databases[i].Name, cfg.Databases[i].LocalPath)
		}
	}
	return w.Flush()
}

func hasLocalOnly(cfg *config.Config) bool {
	for i := range cfg.Databases {
		if cfg.Databases[i].LocalOnly() {
			return true
		}
	}
	return false
}

// printLocalOnly er listen for en bruger uden server. ID- og CREATED-kolonnerne
// udelades: de har ingen betydning uden en server-side database bag sig.
func printLocalOnly(cfg *config.Config) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  \tNAME\tLOCAL PATH")
	for i := range cfg.Databases {
		if !cfg.Databases[i].LocalOnly() {
			continue
		}
		fmt.Fprintf(w, "L\t%s\t%s\n", cfg.Databases[i].Name, cfg.Databases[i].LocalPath)
	}
	return w.Flush()
}
