-- KeePass Delta-Sync · Migration 008
-- object_kind: skelner entries (1) fra grupper (2) i den delte, klient-krypterede
-- blob-store. Forbereder v4 gruppe-sync (se docs/v4-group-sync.md).
--
-- Additiv og bagudkompatibel: alle eksisterende rækker bliver kind=1, og
-- GET /changes filtrerer entries-only som default (kun nye klienter der sender
-- ?include=groups ser kind=2). Adfærden er derfor uændret indtil gruppe-objekter
-- faktisk skrives — denne migration kan udrulles for sig selv, før resten af v4.
-- SPDX-License-Identifier: AGPL-3.0-or-later

ALTER TABLE entry_versions
    ADD COLUMN object_kind SMALLINT NOT NULL DEFAULT 1;

-- 1 = entry, 2 = group. Navngivet constraint så den er nem at finde/udvide.
ALTER TABLE entry_versions
    ADD CONSTRAINT entry_versions_object_kind_chk CHECK (object_kind IN (1, 2));

-- Understøtter den entries-only-filtrerede /changes-forespørgsel.
CREATE INDEX entry_versions_kind_seq_idx
    ON entry_versions (database_id, object_kind, server_seq);
