// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/crypto"
)

// ensureDevicePublicKey auto-upgrader legacy-enheder enrolled før v2: hvis
// config mangler DevicePrivateKey men har en gyldig DeviceToken, generér et
// frisk X25519 keypair, upload public-delen via PATCH /api/v1/me, og gem
// private-delen i config.
//
// Idempotent: returnerer straks hvis private-key allerede er sat eller hvis
// enheden ikke er enrolled. Sikker at kalde fra enhver kommando der har
// fået sin api.Client + config klar.
//
// Fejl her er ikke-fatale for de fleste kommandoer — sync virker fint uden
// keypair, kun sharing-features kræver det. Caller'en bør logge og fortsætte,
// ikke aborte hele kommandoen.
func ensureDevicePublicKey(ctx context.Context, cfg *config.Config, client *api.Client) error {
	if len(cfg.Server.DevicePrivateKey) > 0 {
		return nil
	}
	if cfg.Server.DeviceToken == "" {
		return nil
	}

	pub, priv, err := crypto.GenerateBoxKeypair()
	if err != nil {
		return fmt.Errorf("generate device keypair: %w", err)
	}

	if err := client.UpdateDevicePublicKey(ctx, cfg.Server.DeviceToken, pub); err != nil {
		return fmt.Errorf("upload device public key: %w", err)
	}

	cfg.Server.DevicePrivateKey = config.Base64Bytes(priv)
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config after key generation: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Auto-generated device keypair for v2 sharing (public uploaded).")
	return nil
}
