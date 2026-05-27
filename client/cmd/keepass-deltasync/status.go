// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// runStatus printer den enrollet enheds aktuelle status: hvem serveren mener
// vi er, hvornår vi sidst blev set, og hvor config-filen ligger. Bruger device-
// tokenet fra config — fejler tydeligt hvis brugeren ikke har kørt enroll endnu.
func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync status")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return errors.New("status takes no arguments")
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

	resp, err := api.New(cfg.Server.URL).Me(ctx, cfg.Server.DeviceToken)
	if err != nil {
		return err
	}

	path, _ := config.Path()
	fmt.Printf("Server:        %s\n", cfg.Server.URL)
	fmt.Printf("User:          %s (%s)\n", resp.User.Username, resp.User.ID)
	if resp.User.DisplayName != "" {
		fmt.Printf("Display name:  %s\n", resp.User.DisplayName)
	}
	fmt.Printf("Device:        %s (%s)\n", resp.Device.Name, resp.Device.ID)
	fmt.Printf("Enrolled at:   %s\n", resp.Device.EnrolledAt)
	fmt.Printf("Last seen:     %s\n", resp.Device.LastSeen)
	fmt.Printf("Config file:   %s\n", path)
	return nil
}
