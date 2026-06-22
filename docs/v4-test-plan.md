# v4 group-sync — testplan & verifikationsstatus

Lagdelt teststrategi for v4 (se `docs/v4-group-sync.md` for designet). Bygges
nedefra: hurtige isolerede tests → ægte multi-enheds-scenarier. Status pr.
2026-06-22.

## Lag 1 — unit-tests (pr. komponent)

| Komponent | Kør | Status |
|-----------|-----|--------|
| Go desktop | `cd client && go test ./...` | ✅ grøn (canonical Group/parent, `collectTree`, `BuildStagingXMLWithGroups` round-trip, envelope 0x02) |
| Kotlin Android | `cd android && JAVA_HOME="…/Android Studio/jbr" ./gradlew :sync:test` | ✅ grøn (canonical Group, Mapper, adapter read+apply, SyncEngine, alt med fake CryptoSession) |
| Go mobile-bro | `cd client && go test ./mobile/...` | ✅ grøn (EncryptGroup/DecryptGroup round-trip + kind-separation) |
| PHP server | (suite ikke bootstrappet — kun planlagte klasser) | ⬜ hul; `php -l` grøn |

`android/sync` er et `kotlin("jvm")`-modul, så tests kører på JVM med Android
Studio's jbr — ingen emulator nødvendig.

## Lag 2 — kontrakt (Go ↔ Kotlin wire-kompat)

Entry-fixturen (`EntryTest.parses Go-emitted fixture`) sikrer at Kotlin parser
Go's canonical-JSON. **TODO:** tilsvarende Group-fixture for at fange
schema-drift på gruppe-feltet automatisk.

## Lag 3 — server-integration mod live

✅ **Verificeret** (mod `deltasync.bjoerck-braun.dk`, isoleret "DELETEME"-DB,
slettet bagefter): opret DB → PUT gruppe (object_kind=2) → PUT entry →
`GET /changes` (default = kun entry) vs `?include=groups` (begge + `kind`) →
DELETE gruppe-tombstone. Bekræfter fase 1 (forward-compat-filter) + fase 3
(gruppe-endpoints). Migration 008 + ny PHP-kode spiller sammen mod den rigtige DB.

Metode: `curl` med device-token fra `config.toml` (blobs er opaque → ingen
master-password nødvendig).

## Lag 4 — desktop end-to-end mod live

✅ **Verificeret** med rigtig `keepassxc-cli` + isoleret test-DB:

| Scenarie | Resultat |
|----------|----------|
| Push grupper + entry-placering → server | ✅ |
| Pull → genopbyg gruppetræ → keepassxc merge | ✅ |
| Root-sentinel på tværs af forskellige Root-UUID'er | ✅ (kdbxA og kdbxB uafhængigt oprettet) |
| Flyt entry mellem grupper (multi-enhed) | ✅ flytning propagerer (laterTime fanger LocationChanged-bump) |
| Samtidig flytning (konflikt, LWW) | ✅ seneste flytning vinder, konvergerer på begge enheder |

Metode/harness (Git Bash): `keepassxc-cli db-create/mkdir/add/mv/rmdir/ls`
non-interaktivt (`printf "pw\n" | …`); klient med
`--password-stdin --keepassxc-cli <path>` (**flags FØR positional navn** pga.
Go's `flag`-parser); `init --bind <remote-id>` binder en 2. lokal kdbx til samme
DB (frisk last_seq); oprydning: slet DB via curl + `config.toml` backup/restore.
Faldgrube: `keepassxc-cli mv`-mål må ikke være `/` i Git Bash (sti-mangling) —
brug navngiven gruppe; sæt IKKE `MSYS_NO_PATHCONV=1` (bryder kdbx-fil-stier).

## Lag 5 — Android end-to-end

⬜ **Kræver gomobile `.aar`-regen.** Logikken er unit-testet (Lag 1), men app'en
kan ikke gruppe-synke før `android/libs/deltasync.aar` regenereres med
`Session.EncryptGroup/DecryptGroup` (`gomobile bind`, kræver gomobile + NDK) og
`GomobileCryptoSession`-stubbene erstattes med de rigtige kald.

## Kendte begrænsninger (v4.0)

1. **Desktop gruppe-sletning:** push-side detektion virker (gruppe-tombstone
   sendes korrekt), men `keepassxc-cli merge` honorerer `DeletedObjects` for
   entries men IKKE for (tomme) grupper → en slettet gruppes tomme shell består
   på andre **desktops**. Android's `applyToDatabase` kalder derimod `removeGroup`
   og fjerner den. Entries i gruppen slettes overalt (papirkurv-synthesis), så
   det er kosmetisk, ikke datatab.
2. **Android-app:** afventer `.aar`-regen (se Lag 5).
3. **Gruppe-omdøbning** via `keepassxc-cli` har ingen direkte kommando; omdøbning
   sker normalt via GUI'en og propagerer da via gruppe-blobbens `name`-felt
   (ikke separat testet via CLI).

## Resterende testarbejde

- Group-kontrakt-fixture (Lag 2).
- Bootstrap PHPUnit-suiten (Lag 1 PHP).
- Android end-to-end efter `.aar`-regen (Lag 5).
- Evt. desktop post-merge gruppe-fjernelse hvis begrænsning #1 skal lukkes.
