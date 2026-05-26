-- KeePass Delta-Sync · Migration 003
-- Audit-log med INFO/DEBUG-niveauer. Retention håndteres af cleanup-trigger ved opstart.
-- SPDX-License-Identifier: AGPL-3.0-or-later

CREATE TABLE audit_log (
    id            BIGSERIAL PRIMARY KEY,
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    level         TEXT NOT NULL,                 -- 'INFO' | 'DEBUG'
    event_type    TEXT NOT NULL,                 -- f.eks. 'auth.success', 'entry.put'
    user_id       UUID REFERENCES users(id) ON DELETE CASCADE,    -- nullable: fejlede auth-forsøg har ingen user
    device_id     UUID REFERENCES devices(id) ON DELETE SET NULL,
    database_id   UUID,                          -- ikke FK; bevares efter sletning
    entry_uuid    UUID,
    ip_address    INET,
    user_agent    TEXT,
    details       JSONB,
    success       BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX audit_log_occurred ON audit_log(occurred_at);
CREATE INDEX audit_log_user     ON audit_log(user_id, occurred_at DESC);
CREATE INDEX audit_log_event    ON audit_log(event_type);
