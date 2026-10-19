// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/crypto"
)

// runFingerprint implementerer "keepass-deltasync fingerprint": aftrykket af
// denne enheds public-key. Det er den modtager-side af delingen: den der
// deler, ser et aftryk som serveren har leveret, og det er dette den skal
// sammenlignes med — oplæst ad en kanal serveren ikke styrer.
//
// Udskriver kun aftrykket på stdout, så GUI'en kan bruge det direkte.
func runFingerprint(args []string) error {
	if len(args) != 0 {
		return errors.New("fingerprint takes no arguments")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Server.DeviceToken == "" {
		return errors.New("not enrolled — run `keepass-deltasync enroll` first")
	}
	// En enhed enrolled før v2 har endnu intet keypair; opret det nu, ellers
	// er der intet aftryk at vise og ingen nøgle at dele til.
	if len(cfg.Server.DevicePrivateKey) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := ensureDevicePublicKey(ctx, cfg, api.New(cfg.Server.URL)); err != nil {
			return fmt.Errorf("create device keypair: %w", err)
		}
	}
	pub, err := crypto.PublicKeyFromPrivate(cfg.Server.DevicePrivateKey)
	if err != nil {
		return fmt.Errorf("derive device public key: %w", err)
	}
	fmt.Println(crypto.KeyFingerprint(pub))
	return nil
}
