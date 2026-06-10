// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// runDeleteDatabase sletter en database PERMANENT på serveren via
// DELETE /api/v1/databases/{id}. I modsætning til `forget` (der kun fjerner den
// lokale binding) fjerner dette entries, versioner, shares og historik server-
// side — CASCADE. Kun ejeren kan slette.
//
// Argumentet kan være enten et lokalt navn (slås op i config → RemoteID) ELLER
// en rå UUID. UUID-formen er nødvendig for at kunne rydde op i databaser der
// IKKE er bundet lokalt (fx en duplikat der dukker op i `databases` med markør
// "?" / "(not bound locally)") — der findes intet lokalt navn at slå op.
//
// Efter en vellykket server-sletning fjernes også eventuelle lokale bindings
// der pegede på samme RemoteID, så config og server holdes konsistente. Den
// lokale .kdbx-fil røres aldrig.
// resolveDeleteTarget oversætter et bruger-argument til en server-side RemoteID.
// Et lokalt navn slås op i config (→ RemoteID); ellers accepteres en rå UUID,
// så ikke-bundne databaser også kan slettes. Alt andet er en fejl.
func resolveDeleteTarget(cfg *config.Config, target string) (string, error) {
	if db := cfg.FindDatabase(target); db != nil {
		return db.RemoteID, nil
	}
	if uuidRe.MatchString(target) {
		return target, nil
	}
	return "", fmt.Errorf("%q is neither a local database name nor a UUID (use `databases` to list, or pass the server UUID)", target)
}

func runDeleteDatabase(args []string) error {
	fs := flag.NewFlagSet("delete-database", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "skip confirmation prompt")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync delete-database <name|uuid> [--yes]")
		fs.PrintDefaults()
	}
	args = rearrangeFlagsFirst(args, map[string]bool{"yes": true})
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("delete-database takes exactly 1 argument: <name|uuid>")
	}
	target := fs.Arg(0)

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Server.URL == "" || cfg.Server.DeviceToken == "" {
		return errors.New("not enrolled — run `keepass-deltasync enroll` first")
	}

	// Resolve target → remoteID. Prøv lokalt navn først, ellers rå UUID.
	remoteID, err := resolveDeleteTarget(cfg, target)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := api.New(cfg.Server.URL)

	// Krydstjek mod serverens liste, så vi kan vise det rigtige navn og advare
	// tidligt hvis databasen ikke er synlig eller brugeren ikke er ejer.
	serverName := "(unknown — not visible to this device)"
	if remotes, err := client.ListDatabases(ctx, cfg.Server.DeviceToken); err == nil {
		for i := range remotes {
			if remotes[i].ID == remoteID {
				serverName = remotes[i].Name
				if r := remotes[i].Role; r != "" && r != "owner" {
					return fmt.Errorf("database %q (%s) is shared with you as %q — only the owner can delete it (use `unshare` to leave it)", serverName, remoteID, r)
				}
				break
			}
		}
	}

	if !*yes {
		fmt.Fprintf(os.Stderr, "About to PERMANENTLY delete database %q (%s) on the server.\n", serverName, remoteID)
		fmt.Fprint(os.Stderr, "This removes all entries, versions, shares and history (CASCADE). The local .kdbx file is untouched.\nType 'yes' to confirm: ")
		var input string
		fmt.Scanln(&input)
		if strings.TrimSpace(input) != "yes" {
			return errors.New("aborted")
		}
	}

	if err := client.DeleteDatabase(ctx, cfg.Server.DeviceToken, remoteID); err != nil {
		return fmt.Errorf("delete database: %w", err)
	}

	// Ryd op i lokale bindings der pegede på samme RemoteID (kan være flere navne).
	kept := make([]config.Database, 0, len(cfg.Databases))
	var removed []string
	for _, d := range cfg.Databases {
		if d.RemoteID == remoteID {
			removed = append(removed, d.Name)
			continue
		}
		kept = append(kept, d)
	}
	if len(removed) > 0 {
		cfg.Databases = kept
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("server database deleted, but failed to update local config: %w", err)
		}
	}

	fmt.Printf("Deleted database %q (%s) on the server.\n", serverName, remoteID)
	if len(removed) > 0 {
		fmt.Printf("Removed local binding(s): %s. The .kdbx file(s) are untouched.\n", strings.Join(removed, ", "))
	}
	return nil
}
