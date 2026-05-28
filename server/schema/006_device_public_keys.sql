-- KeePass Delta-Sync · Migration 006
-- X25519 public keys pr. enhed til v2's multi-bruger sharing.
--
-- Hver enhed har et X25519 keypair (genereret ved enroll). Private-delen
-- forbliver kun på enheden; public-delen lagres her så andre brugere kan
-- wrappe shared database master_keys via sealed-box til denne enhed.
--
-- NULL er tilladt for legacy-enheder enrolled før v2 — de skal opdatere
-- public_key via PATCH /api/v1/me før de kan modtage shares. Når alle
-- legacy-enheder er migreret (eller fjernet) kan vi senere tilføje
-- NOT NULL constraint.
--
-- SPDX-License-Identifier: AGPL-3.0-or-later

ALTER TABLE devices
    ADD COLUMN public_key BYTEA;

COMMENT ON COLUMN devices.public_key IS
    'X25519 public key (32 bytes) til sealed-box wrapping af shared database master_keys. NULL for legacy-enheder.';
