-- KeePass Delta-Sync · Migration 005
-- Trigger der rotererer entry-versioner ved insert, så max 3 versioner bevares pr. entry.
--
-- Anvendelse fra applikationskode:
--   INSERT INTO entry_versions
--     (database_id, entry_uuid, blob, modified_at, deleted, server_seq, version_num)
--   VALUES (..., 3);
--
-- Triggeren overskriver alligevel NEW.version_num := 3, så værdien er en placeholder
-- der opfylder CHECK-constraint'en. Eksisterende versioner forskydes 3→2, 2→1, og
-- en evt. tidligere version 1 slettes.
--
-- SPDX-License-Identifier: AGPL-3.0-or-later

CREATE OR REPLACE FUNCTION rotate_entry_versions()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM entry_versions
     WHERE database_id = NEW.database_id
       AND entry_uuid  = NEW.entry_uuid
       AND version_num = 1;

    UPDATE entry_versions
       SET version_num = version_num - 1
     WHERE database_id = NEW.database_id
       AND entry_uuid  = NEW.entry_uuid
       AND version_num IN (2, 3);

    NEW.version_num := 3;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER entry_versions_rotate
    BEFORE INSERT ON entry_versions
    FOR EACH ROW EXECUTE FUNCTION rotate_entry_versions();
