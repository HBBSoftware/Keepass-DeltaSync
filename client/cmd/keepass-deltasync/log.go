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

// runLog henter brugerens egen audit-log fra serveren. --since accepterer en
// Go-duration (24h, 30m, 90s) målt baglæns fra nu. --limit clampes server-
// side til [1, 200]. Output er tabulært: tid, level, event, ok/fail, IP.
func runLog(args []string) error {
	fs := flag.NewFlagSet("log", flag.ContinueOnError)
	sinceFlag := fs.Duration("since", 0, "show entries from the last DURATION (e.g. 24h, 30m). 0 = no filter.")
	limitFlag := fs.Int("limit", 50, "max entries to return (server clamps to [1, 200])")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync log [--since=DURATION] [--limit=N]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return errors.New("log takes no positional arguments")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Server.URL == "" || cfg.Server.DeviceToken == "" {
		return errors.New("not enrolled — run `keepass-deltasync enroll --server <url> <token>` first")
	}

	var since *time.Time
	if *sinceFlag > 0 {
		t := time.Now().Add(-*sinceFlag)
		since = &t
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	entries, err := api.New(cfg.Server.URL).ListLog(ctx, cfg.Server.DeviceToken, since, *limitFlag)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Println("(no log entries)")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tLEVEL\tEVENT\tOK\tIP")
	for _, e := range entries {
		ok := "OK"
		if !e.Success {
			ok = "FAIL"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			e.OccurredAt, e.Level, e.EventType, ok, e.IPAddress)
	}
	return w.Flush()
}
