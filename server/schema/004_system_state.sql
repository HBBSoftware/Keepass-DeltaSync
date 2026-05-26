-- KeePass Delta-Sync · Migration 004
-- Nøgle-værdi-state til throttling af baggrundsopgaver (audit-cleanup mm.).
-- SPDX-License-Identifier: AGPL-3.0-or-later

CREATE TABLE system_state (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
