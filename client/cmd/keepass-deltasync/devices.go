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

// runDevices lister brugerens enrollede enheder, eller — med underkommandoen
// `remove <id>` — tilbagekalder en enhed. Den aktuelle enhed markeres med en
// asterisk i listen, så brugeren let kan se hvilken token der er aktiv lokalt.
func runDevices(args []string) error {
	// Underkommando: `devices remove <id>` tilbagekalder en enhed server-side.
	if len(args) > 0 && args[0] == "remove" {
		return runDevicesRemove(args[1:])
	}

	fs := flag.NewFlagSet("devices", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync devices [remove <id>]")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return errors.New("devices takes no arguments (did you mean `devices remove <id>`?)")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Server.URL == "" || cfg.Server.DeviceToken == "" {
		return errors.New("not enrolled — run `keepass-deltasync enroll --server <url> <token>` first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	devices, err := api.New(cfg.Server.URL).ListDevices(ctx, cfg.Server.DeviceToken)
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		fmt.Println("(no devices enrolled)")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  \tNAME\tID\tENROLLED\tLAST SEEN")
	for _, d := range devices {
		marker := " "
		if d.IsCurrent {
			marker = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			marker, d.Name, d.ID, d.EnrolledAt, d.LastSeen)
	}
	return w.Flush()
}

// runDevicesRemove tilbagekalder en enhed via DELETE /devices/{id}. id'et er
// enhedens UUID som vist i ID-kolonnen i `devices`-listen. Det rører intet
// lokalt; fjerner man sin EGEN enhed (den med *), bliver den lokale token
// ugyldig ved næste server-kald.
func runDevicesRemove(args []string) error {
	fs := flag.NewFlagSet("devices remove", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync devices remove <id>")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("devices remove takes exactly one device id")
	}
	id := fs.Arg(0)

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Server.URL == "" || cfg.Server.DeviceToken == "" {
		return errors.New("not enrolled — run `keepass-deltasync enroll --server <url> <token>` first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := api.New(cfg.Server.URL).DeleteDevice(ctx, cfg.Server.DeviceToken, id); err != nil {
		return err
	}
	fmt.Printf("device %s removed (token revoked)\n", id)
	return nil
}
