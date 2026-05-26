# KeePass Delta-Sync — Projektspecifikation

## Formål

Bygge et synkroniseringssystem til KeePass-databaser (.kdbx) der synkroniserer på **entry-niveau** i stedet for fil-niveau. Dette løser konflikter når flere klienter redigerer samme database samtidigt, ved at genbruge KeePassXC's eksisterende merge-logik.

Filformatet (.kdbx) ændres ikke. Synkronisering sker via en let server der kun ser krypterede blobs.

## Arkitektur — overblik

```
+--------------+      delta (changed entries)      +--------------+
¦   Klient     ¦ ?--------------------------------? ¦    Server    ¦
¦              ¦                                     ¦              ¦
¦ .kdbx lokalt ¦                                     ¦  KV-store    ¦
¦ + sync-agent ¦                                     ¦  (PostgreSQL)¦
+--------------+                                     +--------------+
```

1. Sync-agent på klienten kører ved siden af KeePassXC
2. Den henter ændrede entries fra serveren (delta siden sidste sync)
3. Bygger en midlertidig .kdbx af deltaet
4. Kalder `keepassxc-cli merge` mod den lokale database
5. Identificerer lokalt ændrede entries og pusher dem tilbage
6. Serveren ser kun klient-krypterede blobs — kender ikke masterpassword

## Platforme — faser

**Fase 1:** Linux, Windows, Android
**Fase 2:** macOS, iOS

Server-komponenten er platform-uafhængig (PHP + PostgreSQL).

## Multi-user model

Serveren understøtter **flere brugere med private databaser**. Hver bruger kan have flere databaser, og hver database kan synkroniseres til flere af brugerens egne enheder. **Ingen deling mellem brugere** — det holder threat model'en simpel og passer til at KeePass-formatet ikke har et koncept om "andres entries".

### Brugeroprettelse
Lukket server: kun **admin opretter brugere**. Ingen selvbetjent registrering. Admin har et CLI-værktøj eller simpel admin-side til at:
- Oprette brugere (username + initial setup-token)
- Deaktivere/slette brugere (kaskadesletter deres databaser og enheder)
- Se status (antal databaser, sidste aktivitet)

### Enheder vs. brugere
- Én **bruger** kan have flere **enheder** (f.eks. arbejds-PC + privat laptop + telefon)
- Hver enhed har sin egen bearer-token (genereret ved enrollment)
- Enrollment: brugeren får en engangs-setup-token fra admin, kører `keepass-deltasync enroll <token>` på enheden, hvilket bytter den til en permanent enhedstoken
- Bruger kan se og tilbagekalde sine egne enheder

### Isolation
- Alle API-kald scopes til den autentificerede brugers data via `owner_id`
- En enhedstoken kan kun tilgå databaser ejet af enhedens bruger
- PostgreSQL row-level security som ekstra forsvarslag (optional men anbefales)

## Licens

Projektet udgives som **open source** med en delt licensmodel:

- **Server-komponent: AGPL-3** — sikrer at SaaS-hosting også deler ændringer tilbage
- **Klient-komponenter (desktop + Android): GPL-3** — samme licensfamilie som KeePassXC

**Begrundelse for opdelingen**:
- Klienten matcher KeePassXC's egen licens (dual-licensed GPL-2/GPL-3) — vi vælger GPL-3 som den moderne variant
- Serveren får AGPL-3 fordi den kan hostes som SaaS. Uden AGPL kunne en kommerciel aktør tage serverkoden, modificere den, og tilbyde den som lukket SaaS-produkt uden at dele ændringer
- Klient ? server kommunikerer kun over et veldefineret HTTP-API, så GPL/AGPL "smitter" ikke på tværs af komponenterne — de kan have hver sin licens

**Implementering**:
- Hvert repository indeholder `LICENSE` (fuldtekst af respektiv licens) og `COPYING`
- SPDX-header i hver kildefil:
  - Server: `// SPDX-License-Identifier: AGPL-3.0-or-later`
  - Klient: `// SPDX-License-Identifier: GPL-3.0-or-later`
- README.md har tydelig licens-sektion med begrundelse
- Tredjepartsafhængigheder skal være licens-kompatible:
  - **GPL-3-kompatible**: MIT, BSD-2/3-Clause, Apache-2.0, ISC, MPL-2.0
  - **AGPL-3-kompatible**: samme som ovenfor (AGPL er strengere men accepterer samme permissive licenser)
  - **Inkompatible**: original BSD-4-Clause (advertising clause), proprietære licenser, GPL-2-only (uden "or later")

**Repository-struktur** (separate repos så hver komponent kan vedligeholdes uafhængigt):
- `keepass-deltasync-server` (AGPL-3) — PHP/PostgreSQL-server
- `keepass-deltasync-client` (GPL-3) — Go-baseret CLI/daemon til desktop
- `keepass-deltasync-android` (GPL-3) — Android-app
- `keepass-deltasync-docs` (CC-BY-SA-4.0) — fælles dokumentation, protokol-specifikation (CC-licens passer bedre til tekst end GPL)

**Bidragsmodel**:
- Pull requests velkomne, men kræver DCO (Developer Certificate of Origin) sign-off i commits (`git commit -s`)
- Ingen CLA (Contributor License Agreement) — bidragsydere bevarer ophavsret til deres egne ændringer, licenseret under projektets licens
- Sikkerhedshul rapporteres via GitHub Security Advisories eller separat security-mail — ikke offentlige issues
- Kode-review obligatorisk på alt der rører krypto eller auth
- Mindst to maintainere skal godkende sikkerhedsrelaterede ændringer

**Bemærkning om Android og GPL-3**:
- Hold dig til AOSP og AndroidX-biblioteker (Apache-2.0) — kompatible med GPL-3
- Undgå Google Play Services hvor muligt — proprietære, kan skabe distributions-problemer på F-Droid
- Distribuér både via Google Play (med standard Android-signering) **og** F-Droid (kræver reproducible builds, helt open source-toolchain)

---

## Server-komponent

### Stack
- **Sprog:** PHP 8.2+
- **Database:** PostgreSQL 14+
- **HTTP:** REST over HTTPS
- **Auth:** Bearer-token (genereres pr. enhed ved enrollment)

### Datamodel

```sql
CREATE TABLE users (
    id            UUID PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    display_name  TEXT,
    created_at    TIMESTAMPTZ DEFAULT now(),
    disabled      BOOLEAN DEFAULT false
);

CREATE TABLE admin_tokens (
    token_hash    TEXT PRIMARY KEY,
    created_at    TIMESTAMPTZ DEFAULT now(),
    last_used     TIMESTAMPTZ
);

CREATE TABLE enrollment_tokens (
    token_hash    TEXT PRIMARY KEY,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL,
    used          BOOLEAN DEFAULT false
);

CREATE TABLE devices (
    id            UUID PRIMARY KEY,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT,
    token_hash    TEXT NOT NULL,
    enrolled_at   TIMESTAMPTZ DEFAULT now(),
    last_seen     TIMESTAMPTZ
);

CREATE TABLE databases (
    id            UUID PRIMARY KEY,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    created_at    TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE entries (
    database_id   UUID NOT NULL REFERENCES databases(id) ON DELETE CASCADE,
    entry_uuid    UUID NOT NULL,              -- KeePass entry UUID
    PRIMARY KEY (database_id, entry_uuid)
);

CREATE TABLE entry_versions (
    id            BIGSERIAL PRIMARY KEY,
    database_id   UUID NOT NULL,
    entry_uuid    UUID NOT NULL,
    blob          BYTEA NOT NULL,             -- klient-krypteret payload
    modified_at   TIMESTAMPTZ NOT NULL,       -- KeePass last-modified
    deleted       BOOLEAN DEFAULT false,      -- tombstone
    server_seq    BIGINT NOT NULL,            -- monotonic sync-kursor
    created_at    TIMESTAMPTZ DEFAULT now(),  -- hvornår denne version blev modtaget
    version_num   SMALLINT NOT NULL,          -- 1, 2, eller 3 (3 = nyeste)
    FOREIGN KEY (database_id, entry_uuid) REFERENCES entries(database_id, entry_uuid) ON DELETE CASCADE
);

CREATE INDEX entry_versions_lookup ON entry_versions(database_id, entry_uuid, version_num);
CREATE INDEX entry_versions_seq_idx ON entry_versions(database_id, server_seq);
CREATE UNIQUE INDEX entry_versions_unique ON entry_versions(database_id, entry_uuid, version_num);
CREATE INDEX devices_user_idx ON devices(user_id);
CREATE INDEX databases_user_idx ON databases(user_id);
```

`server_seq` er en monotonisk tæller pr. database. Klienter husker sidste set sekvens og henter alt efter den.

### Versionering — model

Hver entry har op til **3 versioner** i `entry_versions`:
- `version_num = 3`: nyeste (current)
- `version_num = 2`: forrige
- `version_num = 1`: ældste bevarede

**Ved upload af ny entry-version** (PUT på en entry):
1. Slet eventuel række med `version_num = 1` (ældste)
2. Renummerer: `version_num 2 ? 1`, `version_num 3 ? 2`
3. Indsæt ny række som `version_num = 3`

Dette gøres atomisk i en transaktion. PostgreSQL kan gøre det med en CTE eller med en BEFORE INSERT trigger. Anbefales: trigger, så API-koden forbliver simpel.

**Eksempel-trigger** (skitse):
```sql
CREATE OR REPLACE FUNCTION rotate_entry_versions()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM entry_versions
     WHERE database_id = NEW.database_id
       AND entry_uuid = NEW.entry_uuid
       AND version_num = 1;
    UPDATE entry_versions SET version_num = version_num - 1
     WHERE database_id = NEW.database_id
       AND entry_uuid = NEW.entry_uuid
       AND version_num IN (2, 3);
    NEW.version_num := 3;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

**Førstegangsupload**: Hvis ingen tidligere versioner findes, indsættes som `version_num = 3` direkte.

**Sletning (tombstone)**: Tæller som en ny version — `deleted = true`, `version_num = 3`. Tidligere versioner bevares så brugeren kan "fortryde sletning".

### API-endpoints

Alt under `/api/v1/`. Auth via `Authorization: Bearer <token>`. Tokens har tre typer: admin, enrollment (engangs), enhed.

**Brugerendpoints** (kræver enhedstoken):

| Metode | Path | Formål |
|--------|------|--------|
| `POST` | `/databases` | Opret ny synkroniseret database |
| `GET`  | `/databases` | Liste over brugerens egne databaser |
| `DELETE` | `/databases/{id}` | Slet database (med tilhørende entries) |
| `GET`  | `/databases/{id}/changes?since={seq}` | Hent entries ændret efter `seq` (nyeste version pr. entry) |
| `PUT`  | `/databases/{id}/entries/{uuid}` | Upload entry (krypteret blob) — laver ny version |
| `DELETE` | `/databases/{id}/entries/{uuid}` | Marker som tombstone (også en version) |
| `GET`  | `/databases/{id}/entries/{uuid}/versions` | Liste alle versioner af en entry (op til 3) |
| `GET`  | `/databases/{id}/entries/{uuid}/versions/{num}` | Hent specifik version (1, 2, eller 3) |
| `POST` | `/databases/{id}/entries/{uuid}/restore/{num}` | Rul tilbage: kopiér version `num` som ny nyeste version |
| `GET`  | `/devices` | Liste over brugerens egne enheder |
| `DELETE` | `/devices/{id}` | Tilbagekald enhed |
| `GET`  | `/me` | Aktuel bruger-info |

**Enrollment** (kræver enrollment-token, engangsbrug):

| Metode | Path | Formål |
|--------|------|--------|
| `POST` | `/devices/enroll` | Bytter enrollment-token til permanent enhedstoken. Body: `{"device_name": "..."}` |

**Admin-endpoints** (kræver admin-token):

| Metode | Path | Formål |
|--------|------|--------|
| `POST` | `/admin/users` | Opret bruger. Returnerer enrollment-token til første enhed |
| `GET`  | `/admin/users` | Liste over alle brugere + status |
| `PATCH` | `/admin/users/{id}` | Deaktiver/genaktiver bruger |
| `DELETE` | `/admin/users/{id}` | Slet bruger (kaskade) |
| `POST` | `/admin/users/{id}/enrollment` | Generér ny enrollment-token (f.eks. til ekstra enhed) |
| `POST` | `/admin/databases/{id}/entries/{uuid}/restore/{num}` | Recovery: rul en entry tilbage på vegne af bruger (uden at læse blob-indhold) |

`GET /changes` returnerer JSON (kun nyeste version pr. entry):
```json
{
  "current_seq": 12345,
  "entries": [
    {
      "uuid": "...",
      "blob": "base64...",
      "modified_at": "2026-05-25T10:00:00Z",
      "deleted": false,
      "seq": 12340,
      "available_versions": 3
    }
  ]
}
```

`GET /entries/{uuid}/versions` returnerer:
```json
{
  "entry_uuid": "...",
  "versions": [
    { "version_num": 3, "modified_at": "...", "created_at": "...", "deleted": false, "blob": "base64..." },
    { "version_num": 2, "modified_at": "...", "created_at": "...", "deleted": false, "blob": "base64..." },
    { "version_num": 1, "modified_at": "...", "created_at": "...", "deleted": false, "blob": "base64..." }
  ]
}
```

**Rollback-semantik**: `POST /restore/{num}` tager indholdet af version `num`, og indsætter det som en ny nyeste version (`version_num = 3`). Den eksisterende version_num=3 bliver derved version_num=2 osv. — version_num=1 (ældste) droppes. Det betyder rollback **ikke** sletter historik, men bevæger en gammel version op som "current".

### Authorization-logik
- Alle requests under `/databases/`, `/devices/`, `/me` validerer at den autentificerede enhedstoken tilhører en bruger, og at den anmodede ressource ejes af samme bruger
- 404 (ikke 403) returneres ved adgang til andres ressourcer — undgår information leak om hvad der eksisterer
- Admin-tokens kan **ikke** tilgå brugeres database-indhold — de kan kun administrere brugere og enheder. Dette holder server-administrator ude af entry-data.

### Sikkerhed
- TLS påkrævet
- Blob-indhold krypteres klient-side (server ser kun opaque bytes)
- Admin-tokens har **ingen** adgang til entry-blobs — kun bruger/enheds-administration. Databaseejer-server-admin kan stadig læse blobs direkte fra PostgreSQL, men kan ikke dekryptere dem uden masterpassword
- Rate limiting pr. token
- Tokens genereres som 32 random bytes, hash'es med Argon2id ved lagring
- Enrollment-tokens udløber efter 24 timer og er engangsbrug
- Mislykkede auth-forsøg logges og kan udløse midlertidig IP-block

## Audit-log

Serveren fører en aktivitetslog med automatisk oprydning efter **30 dage**.

### Log-niveauer

Log-niveau styres via konfigurationsvariabel `LOG_LEVEL`:

| Niveau | I drift | I udvikling | Hvad logges |
|--------|---------|-------------|-------------|
| `INFO` | ? default | ? | Auth-hændelser: login, enrollment, fejlede forsøg, token-revokering |
| `DEBUG` | ? | ? default | Alle API-kald inkl. GET/changes, entry-PUTs, version-restore osv. |

Skift sker via miljøvariabel uden kodeændring. Dette holder produktions-log slank og privacy-venlig, mens udviklere kan se alt under fejlfinding.

### Datamodel

```sql
CREATE TABLE audit_log (
    id            BIGSERIAL PRIMARY KEY,
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    level         TEXT NOT NULL,            -- 'INFO' eller 'DEBUG'
    event_type    TEXT NOT NULL,            -- f.eks. 'auth.success', 'entry.put'
    user_id       UUID REFERENCES users(id) ON DELETE CASCADE,  -- nullable: fejlede auth har ingen user
    device_id     UUID REFERENCES devices(id) ON DELETE SET NULL,
    database_id   UUID,                     -- ikke FK, da databasen kan slettes
    entry_uuid    UUID,                     -- nullable
    ip_address    INET,
    user_agent    TEXT,
    details       JSONB,                    -- ekstra kontekst, f.eks. {"version_num": 2}
    success       BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX audit_log_occurred ON audit_log(occurred_at);
CREATE INDEX audit_log_user ON audit_log(user_id, occurred_at DESC);
CREATE INDEX audit_log_event ON audit_log(event_type);
```

### Event-typer

**Auth-hændelser (INFO-niveau)**:
- `auth.success` — gyldigt token brugt
- `auth.failure` — ugyldigt/udløbet token
- `auth.rate_limited` — for mange forsøg
- `enrollment.success` — ny enhed enrolled
- `enrollment.failure` — fejlet enrollment
- `device.revoked` — enhed tilbagekaldt (af bruger eller admin)
- `user.created` / `user.disabled` / `user.deleted` — admin-handlinger
- `admin.action` — generel admin-handling

**Aktivitets-hændelser (DEBUG-niveau)**:
- `database.created` / `database.deleted`
- `entries.changes_fetched` — GET /changes
- `entry.put` — ny version uploaded
- `entry.deleted` — tombstone sat
- `entry.versions_listed` — version-historik tilgået
- `entry.restored` — gammel version rullet frem
- `entry.version_fetched` — specifik version hentet

### API-endpoints

**Brugerendpoints** (kræver enhedstoken):

| Metode | Path | Formål |
|--------|------|--------|
| `GET` | `/log?since={timestamp}&limit={n}` | Brugerens egen aktivitet (paginated, nyeste først) |

Bruger ser **kun** rækker hvor `user_id = den_autentificerede_bruger`. `ip_address` og `user_agent` returneres så brugeren kan opdage uautoriserede enheder.

**Admin-endpoints** (kræver admin-token):

| Metode | Path | Formål |
|--------|------|--------|
| `GET` | `/admin/log?since={timestamp}&user_id={uuid}&event_type={type}&limit={n}` | Komplet log med filtre |
| `POST` | `/admin/log/cleanup` | Manuel trigger af oprydning |

### Oprydning

Oprydning sker som **trigger ved server-opstart** (når PHP-API'en initialiseres / FPM-worker starter):

```php
// Køres én gang ved første request efter opstart, beskyttet med advisory lock
function cleanupAuditLog(PDO $pdo): void {
    // pg_try_advisory_lock så kun én worker rydder op
    $locked = $pdo->query("SELECT pg_try_advisory_lock(42)")->fetchColumn();
    if (!$locked) return;

    try {
        $pdo->exec("DELETE FROM audit_log WHERE occurred_at < now() - INTERVAL '30 days'");
        $pdo->exec("DELETE FROM enrollment_tokens WHERE expires_at < now() OR used = true");
    } finally {
        $pdo->query("SELECT pg_advisory_unlock(42)");
    }
}
```

**Hvorfor opstart-trigger og ikke cron**:
- Ingen ekstern afhængighed (cron, systemd timer, pg_cron-extension)
- Server-administrator kan ikke "glemme" at sætte cron op
- Fungerer ens på alle deployments (Docker, bare metal, hosting)

**Hvorfor advisory lock**:
- Forhindrer race condition hvis flere PHP-workers starter samtidigt
- `pg_try_advisory_lock` returnerer false hvis en anden worker har lock — så springer den oprydningen over

**Throttling**: For at undgå at oprydning kører ved hver request, gemmes `last_cleanup_at` i en lille `system_state`-tabel. Oprydning køres kun hvis sidste run var for mere end 1 time siden.

```sql
CREATE TABLE system_state (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TIMESTAMPTZ DEFAULT now()
);
```

**Admin kan også trigge manuelt** via `POST /admin/log/cleanup` — nyttigt hvis man vil tvinge oprydning før en GDPR-anmodning eller efter at have ændret retention-policy.

---

## Klient-komponent — sync-agent

### Designprincip
Sync-agenten er et **separat værktøj** der ikke kræver ændringer i KeePassXC. Den læser og skriver .kdbx-filer via `keepassxc-cli`, som allerede understøtter alt vi behøver:

- `keepassxc-cli db-info` — basic info
- `keepassxc-cli export` — eksportér til XML
- `keepassxc-cli import` — importér fra XML
- `keepassxc-cli merge` — merge to databaser

### Sprog og platforme

**Sprog: Go** (begrundelse)
- Krydskompilerer trivielt til Linux, Windows, macOS, Android (via gomobile)
- Statisk binær — ingen runtime-afhængigheder
- God krypto-standardbibliotek
- Lille footprint passer til mobile platforme

### Hovedflow

```
+-------------------------------------------------------------+
¦                       Sync cycle                             ¦
+-------------------------------------------------------------¦
¦ 1. Lås lokal database mod redigering (file lock)            ¦
¦ 2. GET /changes?since=<last_seq>                            ¦
¦ 3. Dekryptér modtagne blobs ? entry-XML-fragmenter          ¦
¦ 4. Byg temp.kdbx med samme masterpassword                   ¦
¦ 5. keepassxc-cli merge local.kdbx temp.kdbx                 ¦
¦ 6. Find lokalt ændrede entries siden sidste push:           ¦
¦    - Eksportér local.kdbx til XML                           ¦
¦    - Sammenlign last-modified med last_push_time            ¦
¦ 7. Kryptér ændrede entries ? upload                         ¦
¦ 8. Gem ny last_seq og last_push_time                        ¦
¦ 9. Frigør lock                                              ¦
+-------------------------------------------------------------+
```

### Klient-side kryptering

Hver entry-blob krypteres separat med:
- **Algoritme:** XChaCha20-Poly1305
- **Nøgle:** HKDF-SHA256(masterpassword_derived_key, "deltasync-entry-v1", database_uuid)
- **Nonce:** 24 random bytes pr. entry, prefixed til ciphertext

Serveren kender aldrig nøglen. To forskellige databaser med samme masterpassword har forskellige entry-nøgler pga. database_uuid i HKDF-info.

### Konfigurationsfil

`~/.config/keepass-deltasync/config.toml` (Linux/macOS), `%APPDATA%\keepass-deltasync\config.toml` (Windows):

```toml
[server]
url = "https://sync.example.com"
device_token = "..."

[[database]]
name = "personal"
local_path = "~/Documents/personal.kdbx"
remote_id = "uuid-from-server"
last_seq = 0
last_push = "2026-05-25T10:00:00Z"
```

Masterpassword gemmes **ikke** i config — promptes ved hver sync, eller integreres med OS keyring (libsecret/Windows Credential Manager).

### CLI-interface

```
keepass-deltasync enroll <enrollment-token>     # Registrer enheden hos serveren
keepass-deltasync init <name> <local.kdbx>      # Registrer en database til sync
keepass-deltasync sync [name]                   # Kør sync-cyklus
keepass-deltasync status                        # Vis sync-status
keepass-deltasync daemon                        # Watch-mode (poll hver N min)
keepass-deltasync devices                       # Liste enheder + tilbagekald
keepass-deltasync versions <name> <entry-uuid>  # List tilgængelige versioner af en entry
keepass-deltasync restore <name> <entry-uuid> <version-num>  # Rul en entry tilbage
keepass-deltasync log [--since=24h] [--limit=50]  # Vis egen aktivitet fra serveren
```

Til `versions`/`restore` kan brugeren finde entry-UUID enten via KeePassXC's GUI (vises i entry-properties) eller via `keepassxc-cli` med en søgning på titel. CLI'en kan tilbyde `--by-title "Gmail"` som genvej der laver opslaget automatisk.

### Daemon-mode

I daemon-mode poller agenten serveren periodisk (default 5 min) eller reagerer på fil-ændringer i den lokale .kdbx via fsnotify. Når KeePassXC gemmer databasen, trigges en sync.

---

## Android-klient

Android kræver særlig behandling fordi `keepassxc-cli` ikke kører nativt.

### Approach
- Sync-agent skrives som Go-bibliotek (`gomobile bind` ? .aar)
- Wrapped i en lille Android-app der:
  - Lader brugeren konfigurere server + database
  - Schedulerer sync via WorkManager
  - Bruger app-storage til at gemme den synkroniserede .kdbx
- Merge-delen: brug **kotpass** (Kotlin KeePass-bibliotek) til at parse/merge .kdbx direkte, da `keepassxc-cli` ikke er tilgængelig
- Brugeren åbner databasen i Keepass2Android, som peger på samme fil-sti

### Alternativ
Hvis `kotpass` ikke understøtter merge ordentligt: implementér en simpel entry-level merge i Go (UUID-match + newest-wins-by-timestamp + history-preservation).

---

## Implementeringsrækkefølge

### Milestone 1 — Server MVP
- [ ] PostgreSQL-schema (users, devices, databases, entries, entry_versions, tokens, audit_log, system_state)
- [ ] Trigger til version-rotation (max 3 versioner pr. entry)
- [ ] PHP-API (databases, entries, versions, devices, me, log)
- [ ] Audit-log med INFO/DEBUG-niveauer styret via miljøvariabel
- [ ] Oprydnings-trigger ved opstart (advisory lock + throttling)
- [ ] Admin-CLI til oprettelse/sletning af brugere
- [ ] Token-auth (admin, enrollment, device)
- [ ] Enrollment-flow
- [ ] TLS + nginx reverse proxy
- [ ] Integrationstests med curl — inkl. cross-user isolation, versionsrotation og log-oprydning

### Milestone 2 — Linux-klient (Go)
- [ ] `enroll`-kommando (bytter enrollment-token til enhedstoken)
- [ ] `init`-kommando (database-registrering)
- [ ] Krypterings-lag (XChaCha20-Poly1305 + HKDF)
- [ ] `keepassxc-cli` wrapper
- [ ] Sync-cyklus end-to-end
- [ ] `versions`/`restore` kommandoer
- [ ] Konfigurationsfil + state
- [ ] Test: to maskiner (samme bruger) redigerer forskellige entries ? ingen tab
- [ ] Test: bruger A kan ikke tilgå bruger B's database
- [ ] Test: 4 sekventielle edits ? kun 3 versioner gemmes, ældste droppes
- [ ] Test: restore en version ? ændring propagerer korrekt til anden enhed

### Milestone 3 — Windows-klient
- [ ] Krydskompilér Go-binær til Windows
- [ ] Test mod `keepassxc-cli.exe`
- [ ] Windows Credential Manager-integration
- [ ] Optional: lille tray-app

### Milestone 4 — Android
- [ ] `gomobile bind` af sync-bibliotek
- [ ] Android-app skelet
- [ ] kotpass-integration eller egen merge-impl
- [ ] WorkManager-scheduling
- [ ] Test med Keepass2Android som UI

### Milestone 5 — Polish
- [ ] Daemon-mode med fsnotify
- [ ] OS keyring-integration (Linux: libsecret, Windows: WCM)
- [ ] Fejlhåndtering: netværksfejl, korrupte blobs, version-mismatch
- [ ] Dokumentation

### Senere (fase 2)
- [ ] macOS-klient (trivielt fra Go-build)
- [ ] iOS-klient (gomobile + Swift wrapper, eller MAUI)

---

## Vigtige tekniske beslutninger / faldgruber

1. **`keepassxc-cli merge` skal testes grundigt.** Specielt: håndterer den entry-history korrekt? Hvad sker der ved gruppestruktur-ændringer?

2. **Tombstones er kritiske.** Hvis en entry slettes på maskine A og synces til server som tombstone, skal maskine B respektere det og ikke "genoprette" den ved næste push. KeePass har `DeletedObjects` i .kdbx-formatet — dette skal mappes.

3. **Atomicity i push.** Hvis klienten pusher 10 entries og afbrydes efter 5, skal det være OK. Hver entry-PUT er sin egen transaktion på serveren. Klienten holder en liste over "pending pushes" lokalt og retrier.

4. **Clock skew.** `modified_at` baseres på klientens ur. Server skal ikke afvise entries med fremtidig timestamp — KeePass merge'r baseret på timestamps, ikke serverens ankomsttid.

5. **Masterpassword-skift.** Hvis brugeren skifter masterpassword på databasen, ændres entry-krypteringsnøglen. Strategi: marker databasen som "key-rotated", upload alle entries på ny med nye nøgler, marker gammel generation som forældet. Eller (simplere v1): kræv at masterpassword aldrig ændres efter init — dokumentér begrænsningen.

6. **Initial sync af eksisterende database.** Når en bruger kører `init` på en eksisterende .kdbx med 500 entries, skal alle entries uploades. Sørg for batch-upload.

7. **Server lækker metadata.** Antal entries, hyppighed af ændringer, og timing er synligt for serveren. Samme threat model som Bitwarden — dokumentér det.

8. **`keepassxc-cli` versionering.** Forskellige versioner kan have forskellig opførsel. Detektér version ved start og fejl hvis under minimum.

9. **Versionering og storage.** Med 3 versioner pr. entry stiger storage til ~3x. For 1000 entries à 500 bytes = ~1.5 MB pr. database — stadig minimalt. Ved upload af ny version skal renumeringen være atomisk (transaktion + trigger anbefales over applikation-side logik).

10. **Restore vs. sync-loop.** Når en bruger laver `restore` fra en gammel version, sendes det tilbage til klienterne via den normale `changes`-mekanisme — alle klienter vil se den restored entry som en almindelig ændring og merge den ind. Det betyder også at restore på enhed A propagerer til enhed B automatisk ved næste sync. Vigtigt: restore bumper `server_seq`, så alle klienter ser ændringen.

11. **Version-nummerering ved nye entries.** En nyoprettet entry starter med kun version_num=3. Efter første edit har den 2 og 3. Efter anden edit har den 1, 2 og 3. CLI'ens `versions`-kommando skal håndtere at færre end 3 versioner kan eksistere.

12. **Log-volumen i DEBUG-mode.** Med DEBUG-niveau logges alle GET /changes-kald, som klienter laver ved hver sync-poll. Med 5-minutters poll og 10 enheder = 2880 log-rækker pr. dag. Over 30 dage ~86k rækker — stadig håndterbart, men hold øje med tabel-størrelse i drift hvis I'd ved et uheld kører med DEBUG i produktion.

13. **Privacy i log.** Selv INFO-niveau logger IP-adresser og user-agents. Dokumentér dette i privacy-policy. Brugere kan se egen aktivitet via `/log`-endpoint, hvilket både er et privacy-værktøj (transparens) og et sikkerhedsværktøj (opdage ukendte enheder).

14. **GDPR og log-sletning.** 30-dages retention overholder "data minimisation"-princippet. Når en bruger slettes, kaskadesletter `audit_log`-rækker for den bruger pga. FK-constraint — undtagen `auth.failure`-rækker hvor user_id er null. De forbliver, men indeholder kun IP/timestamp.

---

## Test-strategi

- **Unit:** Krypto-lag, blob-serialisering, config-parsing
- **Integration:** Klient ? server happy path
- **Conflict scenarios:** To klienter ændrer samme entry ? newest-wins, begge har korrekt history bagefter
- **Tombstone:** Sletning propageres korrekt
- **Recovery:** Afbrudt sync midtvejs ? retry virker
- **Cross-platform:** Linux laver entry, Windows henter og redigerer, Android ser ændringen
