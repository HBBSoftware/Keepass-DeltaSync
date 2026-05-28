# v2 multi-bruger sharing: live-test-plan

Test-procedure for M3+M4 når server-PHP er deployet. Kører på én Windows-
maskine ved at have to klient-config'er parallelt via
`KEEPASS_DELTASYNC_CONFIG`.

Genereret som del af M5. Hører sammen med
[`v2-concurrent-write-semantics.md`](v2-concurrent-write-semantics.md).

## Forudsætninger

- Server kører commit ≥ `c178ae3` (M3-endpoints).
- DB-migrationer 001–007 kørt.
- Klienten bygget fra commit ≥ `c6fb8ea` (M4 init-shared).
- Admin-token tilgængelig for at oprette Bob.
- Hans's eksisterende config (default-sti) bruges som Alice's klient.
- Test-vault: Alice's `adgangskoder.kdbx` — opret en test-entry "share-test"
  som vi kan tracke gennem hele flowet.

## Setup: opret Bob

```powershell
# Brug Hans's admin-token (gem den i en env-var inden)
$ADMIN = "<hans-admin-token>"
$SRV = "https://deltasync.bjoerck-braun.dk"

curl -X POST "$SRV/api/v1/admin/users" `
     -H "Authorization: Bearer $ADMIN" `
     -H "Content-Type: application/json" `
     -d '{"username":"bob","display_name":"Bob Test"}'
```

Response indeholder en enrollment-token til Bob. Gem den.

## Setup: Bob's klient-config

```powershell
$env:KEEPASS_DELTASYNC_CONFIG = "D:\Kunder\Data\Hans\Keepass-deltasync\.test\bob-config.toml"

# Bob enroller — genererer keypair lokalt, sender public til server
go run ./cmd/keepass-deltasync enroll --server $SRV --device-name "bob-laptop" <bob-enrollment-token>

# Bob's status — verificér device + public_key landet
go run ./cmd/keepass-deltasync status
```

Forvent at config.toml under `.test/bob-config.toml` har både `device_token`
og `device_private_key`.

## Test 1: Lookup virker

Hans's klient (default config):

```powershell
# Drop $env:KEEPASS_DELTASYNC_CONFIG hvis sat
Remove-Item Env:\KEEPASS_DELTASYNC_CONFIG -ErrorAction SilentlyContinue

# Direkte API-kald for at se output-format
curl -H "Authorization: Bearer <hans-device-token>" `
     "$SRV/api/v1/users/lookup?username=bob"
```

Forvent:
- 200 OK
- `user.username = "bob"`
- `target_device.public_key` er en 44-tegns base64-streng (Bob's keypair)

## Test 2: Share fra Hans til Bob

```powershell
# Hans's klient (default config)
go run ./cmd/keepass-deltasync share adgangskoder bob
```

Prompt: Hans's masterpassword for adgangskoder. Argon2id ~200ms. Output:
"Shared 'adgangskoder' with bob (device: bob-laptop)."

Verificér via API:
```powershell
curl -H "Authorization: Bearer <hans-device-token>" `
     "$SRV/api/v1/databases/<adgangskoder-uuid>/shares"
```

Forvent 2 medlemmer: hans (owner) + bob (member).

## Test 3: Bob ser delt database

```powershell
$env:KEEPASS_DELTASYNC_CONFIG = ".\.test\bob-config.toml"
go run ./cmd/keepass-deltasync databases
```

Forvent at `adgangskoder` (Hans's navn) dukker op i Bob's liste — men kun
remote, ikke lokalt. Direkte API:

```powershell
curl -H "Authorization: Bearer <bob-device-token>" "$SRV/api/v1/databases"
```

Forvent: `databases[0].role = "member"`, `databases[0].wrapped_master_key`
er en ~108-tegns base64-streng (sealed-box overhead = 48 + 32 master_key).

## Test 4: init-shared (Bob's bootstrap)

```powershell
$env:KEEPASS_DELTASYNC_CONFIG = ".\.test\bob-config.toml"
go run ./cmd/keepass-deltasync init-shared adgangskoder ".\.test\bob-vault.kdbx"
```

Prompt: Bob vælger nyt lokalt password (kan være forskelligt fra Alice's).
Forvent at `.test\bob-vault.kdbx` oprettes med Alice's entries inde i en
"deltasync"-undergruppe.

Verificér:
- Åbn `bob-vault.kdbx` i KeePassXC med Bob's lokale password
- Se "share-test"-entry under deltasync-gruppen
- Værdierne er identiske med Alice's lokale kopi

## Test 5: Round-trip member-edit

Bob laver en ændring:

```powershell
# I KeePassXC: edit share-test entry, ændre password, save
# Eller via cli:
"<bob-local-pw>`n<new-share-test-pw>`n<new-share-test-pw>`n" | & "C:\Program Files\KeePassXC\keepassxc-cli.exe" edit -p .test\bob-vault.kdbx share-test
```

Bob sync:

```powershell
$env:KEEPASS_DELTASYNC_CONFIG = ".\.test\bob-config.toml"
go run ./cmd/keepass-deltasync sync adgangskoder
```

Prompt: Bob's lokale password. Forvent `pushed 1 (+ 0 tombstones)`.

Hans pull:

```powershell
Remove-Item Env:\KEEPASS_DELTASYNC_CONFIG
go run ./cmd/keepass-deltasync sync adgangskoder
```

Prompt: Hans's masterpassword. Forvent `pulled 1 ...`. Åbn Hans's
`adgangskoder.kdbx` i KeePassXC, verificér at share-test har Bob's nye
password.

## Test 6: Concurrent edit (race-validering)

Mest interessante M5-test. Koordinér timing manuelt.

1. Hans og Bob har begge `adgangskoder` lokalt med share-test entry
2. Stop alle daemons
3. Hans edit share-test (password=alice1), GEM IKKE endnu i KeePassXC
4. Bob edit share-test (password=bob1), GEM IKKE endnu
5. Hans gem (Ctrl+S). Hans sync. Server får version A (Hans's).
6. Bob gem (Ctrl+S). Bob sync. Bob pull først → ser Hans's version A med
   Hans's mtime T_a. Lokal mtime hos Bob T_b > T_a (Bob gemte senere).
   Merge: Bob's version vinder lokalt. Push: server får version B (Bob's).
7. Hans sync. Pull: får version B. Merge: T_b > T_a, Bob's vinder. Hans's
   lokal opdateres til Bob's version.

**Slutresultat:** begge har Bob's password (bob1). Hans's edit gik tabt.
Det er last-writer-wins, semantisk korrekt for vores model.

Verificér via `versions`-kommandoen:

```powershell
go run ./cmd/keepass-deltasync versions adgangskoder <share-test-uuid>
```

Forvent 3 versioner: original, Hans's (alice1), Bob's (bob1). Restore til
version 2 (Hans's) bringer alice1 tilbage som ny nyeste.

## Test 7: Unshare (Alice trækker Bob's adgang tilbage)

```powershell
Remove-Item Env:\KEEPASS_DELTASYNC_CONFIG
go run ./cmd/keepass-deltasync unshare adgangskoder bob
```

Verificér via Bob's klient:

```powershell
$env:KEEPASS_DELTASYNC_CONFIG = ".\.test\bob-config.toml"
go run ./cmd/keepass-deltasync sync adgangskoder
```

Forvent: pull fejler med 404 (Bob er ikke længere medlem). Bob's lokal
`bob-vault.kdbx` bevares — vi sletter ikke lokal data automatisk.

## Test 8: Re-share (rotation)

Hvis Bob enroller en NY enhed (nyt keypair), skal Alice re-share for at den
nye enhed kan unwrap:

```powershell
# Hans
go run ./cmd/keepass-deltasync share adgangskoder bob
```

Vores POST /shares har `ON CONFLICT DO UPDATE` — wrapped_master_key
roterer til Bob's nyeste enheds public_key. master_key er uændret.

## Pass/fail-kriterier

Alle tests skal passere før M3+M4 anses live-valideret. Specifikt:

- ✅ Test 1–5: basic share-flow virker end-to-end
- ✅ Test 6: concurrent-edit har forventet last-writer-wins-adfærd
- ✅ Test 7: unshare blokerer fremtidig adgang
- ✅ Test 8: re-share roterer wrapped_master_key

Hvis nogle tests fejler:
- Test 1–4: server-deploy issue, tjek PHP-logs
- Test 5: krypto-issue, mest sandsynligt `device_private_key` mismatch
- Test 6: race-bug (skulle ikke ske ifølge analyse)
- Test 7: ACL-bug i ShareController::destroy
- Test 8: SQL ON CONFLICT-issue

## Oprydning

```powershell
# Slet Bob via admin
curl -X DELETE "$SRV/api/v1/admin/users/<bob-user-id>" `
     -H "Authorization: Bearer $ADMIN"

# Slet Bob's lokale config + vault
Remove-Item -Recurse .\.test\

# Hans's keyring entries for test-databaser kan ryddes via Credential Manager
```
