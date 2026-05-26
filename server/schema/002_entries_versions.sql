-- KeePass Delta-Sync · Migration 002
-- Entries + versionshistorik (max 3 versioner pr. entry; selve rotationen sker i 005).
-- SPDX-License-Identifier: AGPL-3.0-or-later

CREATE TABLE entries (
    database_id   UUID NOT NULL REFERENCES databases(id) ON DELETE CASCADE,
    entry_uuid    UUID NOT NULL,                  -- KeePass entry UUID
    PRIMARY KEY (database_id, entry_uuid)
);

CREATE TABLE entry_versions (
    id            BIGSERIAL PRIMARY KEY,
    database_id   UUID NOT NULL,
    entry_uuid    UUID NOT NULL,
    blob          BYTEA NOT NULL,                 -- klient-krypteret payload
    modified_at   TIMESTAMPTZ NOT NULL,           -- KeePass last-modified (klientens ur)
    deleted       BOOLEAN NOT NULL DEFAULT false, -- tombstone
    server_seq    BIGINT NOT NULL,                -- monotonisk sync-kursor pr. database
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    version_num   SMALLINT NOT NULL CHECK (version_num BETWEEN 1 AND 3),
    FOREIGN KEY (database_id, entry_uuid)
        REFERENCES entries(database_id, entry_uuid) ON DELETE CASCADE
);

CREATE INDEX entry_versions_lookup       ON entry_versions(database_id, entry_uuid, version_num);
CREATE INDEX entry_versions_seq_idx      ON entry_versions(database_id, server_seq);
CREATE UNIQUE INDEX entry_versions_unique ON entry_versions(database_id, entry_uuid, version_num);

-- server_seq tildeles pr. database. Holdt i egen tabel så vi kan UPDATE ... RETURNING
-- atomisk i samme transaktion som en entry-PUT.
CREATE TABLE database_seq (
    database_id   UUID PRIMARY KEY REFERENCES databases(id) ON DELETE CASCADE,
    next_seq      BIGINT NOT NULL DEFAULT 1
);
