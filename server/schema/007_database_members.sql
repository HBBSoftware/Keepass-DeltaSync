-- KeePass Delta-Sync · Migration 007
-- database_members: ACL-tabel for v2's multi-bruger sharing.
--
-- Hver database har én eller flere medlemmer. Owner-rollen (role = 'owner')
-- kan dele med andre, slette databasen og overdrage ejerskab. Member-rollen
-- (role = 'member') har read-write adgang til entries men kan ikke ændre
-- medlemskab eller slette databasen.
--
-- wrapped_master_key er sealed-box (NaCl box.SealAnonymous) over databasens
-- master_key, krypteret til medlemmets enheds public-key. NULL for owners
-- der bruger Argon2id(password) direkte — vi gemmer ikke owner's master_key
-- på serveren.
--
-- Vigtig konsekvens: når Bob har flere enheder, har han én row pr. enhed —
-- IKKE pr. bruger — så hver enheds public_key kan wrappe master_key. Det
-- afspejles ved at primary key er (database_id, user_id) men wrapped_master_key
-- er device-specifik. Hmm — det giver inkonsistens. Lad os i stedet:
--
-- Beslutning: keep PK = (database_id, user_id). wrapped_master_key er
-- wrap'et med EN af user's enheders public_key (typisk den enhed der
-- delte ind, dog kan re-share rotere). User's øvrige enheder kan IKKE
-- åbne den. v2.1-feature: per-enhed wrap. v2.0: hver bruger har én
-- "active" enhed der modtager shares. Hvis Bob bytter enhed skal Alice
-- re-share.
--
-- Den begrænsning er KISS for v2.0 og passer med "keypair pr. enhed,
-- re-share ved ny enhed"-design-valget.
--
-- SPDX-License-Identifier: AGPL-3.0-or-later

CREATE TABLE database_members (
    database_id        UUID NOT NULL REFERENCES databases(id) ON DELETE CASCADE,
    user_id            UUID NOT NULL REFERENCES users(id)     ON DELETE CASCADE,
    wrapped_master_key BYTEA,
    role               TEXT NOT NULL CHECK (role IN ('owner', 'member')),
    added_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    added_by           UUID REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (database_id, user_id)
);

CREATE INDEX database_members_user_idx ON database_members(user_id);

-- Backfill: eksisterende databaser bliver ejet af deres oprindelige creator.
-- wrapped_master_key er NULL fordi owners bruger Argon2id-derivation lokalt
-- og ikke har deres master_key liggende på serveren.
INSERT INTO database_members (database_id, user_id, wrapped_master_key, role, added_by)
SELECT id, user_id, NULL, 'owner', user_id FROM databases;

-- database_members er nu sandhedskilden — drop user_id fra databases.
DROP INDEX IF EXISTS databases_user_idx;
ALTER TABLE databases DROP COLUMN user_id;
