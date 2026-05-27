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

// runDevices lister brugerens enrollede enheder. Den aktuelle enhed
// markeres med en asterisk, så brugeren let kan se hvilken token der
// er aktiv lokalt.
func runDevices(args []string) error {
	fs := flag.NewFlagSet("devices", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync devices")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return errors.New("devices takes no arguments")
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
