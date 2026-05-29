# keepass-deltasync-android

Android-klient. Sync-service-only — ingen indbygget entry-editor. KeePassXC's
egne kdbx-filer redigeres af brugerens foretrukne Android-KeePass-app
([Keepass2Android](https://github.com/PhilippC/keepass2android) eller
[KeePassDX](https://github.com/Kunzisoft/KeePassDX)), og DeltaSync-app'en
holder `.kdbx`-filen synkroniseret i baggrunden via WorkManager.

- **Licens:** GPL-3.0-or-later
- **Distribution:** F-Droid først (reproducible builds, ingen Google Play
  Services), Play-version kan tilføjes senere uden refactor.
- **Status:** M4 i gang.
  - Go-siden via `mobile`-pakken er på plads (`gomobile bind`-vendt API for
    crypto + canonical wire-format).
  - Gradle-skelet + `:sync`-modulet er oppe at køre med canonical Kotlin-
    typer der parser Go-emitterede fixtures.
  - Kotpass-mapper på plads — `Mapper.toCanonical()` /
    `Mapper.toKotpass()` konverterer mellem kotpass' typed Entry og vores
    wire-format.
  - `SyncEngine` på plads — orkestrerer pull+push mod serverens REST-API
    (via OkHttp) med entry-level last-writer-wins merge. Tests via
    MockWebServer dækker pull-only, push-only, konfliktscenarier, og
    tombstones (24 grønne tests i alt).
  - Android app-modul (`:app`), WorkManager-service, og den faktiske
    `CryptoSession`-implementation oven på gomobile-bound `.aar` mangler.

## Arkitektur

```
  ┌─────────────────────────────────────────────────────┐
  │ Kotlin (Android-side)                               │
  │                                                     │
  │  WorkManager → SyncService → OkHttp → server        │
  │                  │                                  │
  │                  ├──→ kotpass: læs/skriv .kdbx     │
  │                  │                                  │
  │                  └──→ Entry ↔ canonical JSON       │
  │                       (Kotlin-side mapper)         │
  │                            │                        │
  └────────────────────────────┼────────────────────────┘
                               │ JSON ([]byte) over gomobile
                               ▼
  ┌─────────────────────────────────────────────────────┐
  │ Go (.aar, fra client/mobile/ via gomobile bind)     │
  │                                                     │
  │  Session(password, dbId) → entryKey                 │
  │  EncryptEntry(JSON)      → encrypted wire-blob     │
  │  DecryptEntry(blob)      → JSON                     │
  │  + sharing helpers (sealed-box unwrap)              │
  └─────────────────────────────────────────────────────┘
```

Go-laget er bevidst tyndt: kun det indlejret-spec-afhængige (crypto,
canonical-format-konvertering) for at sikre at desktop ↔ Android producerer
byte-identiske blobs på wiren. HTTP, persistens, threading, UI lever 100% i
Kotlin med idiomatiske Android-mønstre.

Entry-level last-writer-wins merge implementeres i Kotlin oven på kotpass.
Sammenligning af `times.modified` per UUID; ved konflikt tager den nyeste
seier — samme semantik som desktop's keepassxc-cli merge på entry-niveau.

## Build

### Kotlin-siden (`:sync`-modulet)

JVM-only modul med canonical-typer + (senere) kotpass-mapper. Tests kører
på JBR fra Android Studio, ingen emulator nødvendig:

```sh
# Sæt JAVA_HOME til Android Studio's bundlede JBR
export JAVA_HOME="/c/Program Files/Android/Android Studio/jbr"

# Test
cd android
./gradlew :sync:test
```

### Re-generér Go-fixture (når canonical-skemaet ændres)

Hvis Go-sidens `canonical.Entry` ændres, regenerér Kotlin-test-fixturen:

```sh
cd client
go test -tags=emit_fixture -run TestEmitFixture ./internal/kdbx/canonical/
```

Den producerede `android/sync/src/test/resources/canonical-entry-fixture.json`
checkes ind og bruges af Kotlin-testen til at fange schema-drift mellem de
to platforme.

### Go-siden via `gomobile bind` (kræver NDK)

`gomobile bind` producerer en `.aar` + Kotlin-stubs:

```sh
# Engangs-setup: installer gomobile + Android NDK
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init  # downloader NDK eller bruger eksisterende

# Byg .aar fra mobile/-pakken
cd client
gomobile bind -target=android -o ../android/libs/deltasync.aar \
    gitlab.com/Star95/keepass-deltasync/client/mobile
```

`.aar`'en placeres i `android/libs/` (gitignored) og inkluderes som
`implementation(files("libs/deltasync.aar"))` i `:app`-modulets build.

## Distribution

- **F-Droid** (prioriteret): reproducible builds, fuld open source-toolchain.
  Kræver at vi undgår Google Play Services — ingen FCM push, så sync
  drives af WorkManager-polling med rimelig interval (15–30 min) plus
  manuel "sync now" fra UI.
- **Play** (senere): standard Android-signering. Kan tilføjes uden ændringer
  af kerne-arkitekturen.

## Hvad er ikke i scope (forblevet på desktop)

- Egen entry-editor / UI til at læse passwords
- `keepassxc-cli` integration (kører ikke på Android)
- Daemon mode (Android har WorkManager i stedet for fsnotify)

## Hvad mangler (M4-resterende punkter)

- Gradle-skelet (Kotlin DSL, multi-module: app/, sync/)
- kotpass-dep + Kotlin-side mapper mellem kotpass.Entry og canonical JSON
- Foreground service + WorkManager-trigger
- F-Droid metadata + signed reproducible-build config
- Test mod Keepass2Android som ekstern UI
