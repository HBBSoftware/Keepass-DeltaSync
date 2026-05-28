# Schema-migrationer

Filerne er nummereret og skal køres i rækkefølge. Pt. håndteres migrationer manuelt med `psql`; en egentlig migration-runner kan tilføjes senere når der er flere ændringer at versionere.

| Fil | Indhold |
|-----|---------|
| `001_users_devices_databases.sql` | `users`, `admin_tokens`, `enrollment_tokens`, `devices`, `databases` |
| `002_entries_versions.sql` | `entries`, `entry_versions`, `database_seq` |
| `003_audit_log.sql` | `audit_log` |
| `004_system_state.sql` | `system_state` (KV til throttling af baggrundsopgaver) |
| `005_version_rotation_trigger.sql` | `BEFORE INSERT`-trigger på `entry_versions` (max 3 versioner pr. entry) |
| `006_device_public_keys.sql` | `devices.public_key BYTEA` (X25519 til v2 multi-bruger sharing; NULL for legacy) |

## Kørsel

```sh
for f in schema/*.sql; do
  psql "$DATABASE_URL" -f "$f"
done
```

Skemaet bygger på `pgcrypto` (`gen_random_uuid()`). PostgreSQL 14+ har det indbygget men extension skal aktiveres pr. database — det gør `001` for dig.
