// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/crypto"
)

// runEnroll håndterer "keepass-deltasync enroll [--server URL] [--device-name N] <token>".
// Tokenen byttes med serveren til en permanent device-token, som gemmes i config.toml.
func runEnroll(args []string) error {
	fs := flag.NewFlagSet("enroll", flag.ContinueOnError)
	serverFlag := fs.String("server", "", "server URL (e.g. https://deltasync.example.dk). Required on first enroll; persisted afterwards.")
	deviceFlag := fs.String("device-name", "", "device name shown in /me and admin listings (default: hostname)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync enroll [--server URL] [--device-name NAME] <enrollment-token>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("expected exactly one positional argument: the enrollment token")
	}
	enrollmentToken := fs.Arg(0)

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	serverURL := *serverFlag
	if serverURL == "" {
		serverURL = cfg.Server.URL
	}
	if serverURL == "" {
		return errors.New("server URL not provided — pass --server <url> (no existing config found)")
	}

	deviceName := *deviceFlag
	if deviceName == "" {
		if h, err := os.Hostname(); err == nil {
			deviceName = h
		}
	}

	// Generér device-keypair lokalt: private bliver i config, public sendes
	// til serveren så andre brugere kan wrappe shared database-keys til os.
	pubKey, privKey, err := crypto.GenerateBoxKeypair()
	if err != nil {
		return fmt.Errorf("generate device keypair: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := api.New(serverURL)
	resp, err := client.Enroll(ctx, enrollmentToken, deviceName, pubKey)
	if err != nil {
		return err
	}

	cfg.Server.URL = serverURL
	cfg.Server.DeviceToken = resp.Token
	cfg.Server.DevicePrivateKey = config.Base64Bytes(privKey)
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	path, _ := config.Path()
	fmt.Printf("Enrolled successfully.\n")
	fmt.Printf("  Device ID:   %s\n", resp.Device.ID)
	fmt.Printf("  Device name: %s\n", resp.Device.Name)
	fmt.Printf("  Enrolled at: %s\n", resp.Device.EnrolledAt)
	fmt.Printf("  Token saved: %s\n", path)
	return nil
}
