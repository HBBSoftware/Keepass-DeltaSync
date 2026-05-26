# Tests

PHPUnit-baserede integrationstests. Mod en rigtig PostgreSQL (ikke en mock) — netop hele værdien af spec'ens version-rotation-trigger og row-level constraints går tabt hvis vi mocker databasen.

## Planlagte testklasser (Milestone 1)

- `CrossUserIsolationTest` — bruger A kan ikke tilgå bruger B's databaser, devices eller log
- `VersionRotationTest` — 4 sekventielle PUTs på samme entry → kun 3 versioner gemt, version_num 1/2/3 korrekt
- `RestoreTest` — restore af version 1 placerer den som version 3, bumper server_seq, propagerer via /changes
- `TombstoneTest` — DELETE markerer som tombstone, ikke fysisk slet; tidligere versioner bevares
- `AuditLogCleanupTest` — rækker ældre end retention slettes ved opstart; advisory lock forhindrer race
- `AdminBlobAccessTest` — admin-token kan ikke læse entry-blobs gennem nogen API-vej
- `EnrollmentFlowTest` — engangs-token byttes til device-token; gentaget brug fejler

## Kørsel

```sh
vendor/bin/phpunit
```

Forventer en separat test-database — sæt `DATABASE_URL_TEST` i miljøet inden kørsel.
