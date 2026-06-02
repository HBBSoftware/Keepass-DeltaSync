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
  - `GomobileCryptoSession` på plads — production-implementation der
    wrapper den gomobile-bound `mobile.Session` fra `libs/deltasync.aar`.
    Kompilerer mod den gomobile-genererede classes.jar (compileOnly);
    JNI-laget aktiveres først på Android-runtime.
  - `KotpassLocalStateAdapter` på plads — læser/skriver mellem kotpass'
    `KeePassDatabase` og vores `LocalState`. Sletninger propagerer nu:
    den walker selv gruppe-træet (kotpass' `findEntries` filtrerer
    recycle-bin-entries fra) og materialiserer tombstones fra både
    `DeletedObjects` og recycle-bin-entries (`DeletedAt =
    LocationChanged`) — 1:1 med desktop'ens `ParseExport`.
  - `Synchronizer` på plads — high-level orkestrator der pakker hele
    pipelinen i én `sync(databaseId)`-kald: load .kdbx → adapter →
    sync-engine → adapter → atomisk-skriv .kdbx tilbage. Persistens af
    `lastSeq`/`syncedAt` mellem app-starter abstraheret via
    `SyncStatePersistence` (in-memory impl til tests; Android-impl
    bruger DataStore).
  - `EnrollmentClient` på plads — bytter éngangs enrollment-token for
    permanent device-token via POST `/api/v1/devices/enroll`. `TokenStore`-
    interface for sikker lagring af device-token + private key
    (in-memory impl til tests; Android impl bruger Keystore).
  - 40 grønne tests i alt på Kotlin-siden.
  - `:app` Android-modul **bygger som installerbar debug-APK** (~18.8 MB
    inkl. JNI .so for alle 4 ABIs) med komplet enroll → setup → sync flow:
    - `MainActivity` — status-skærm. Viser om enheden er enrolled, om en
      database er konfigureret, og en "Sync now"-knap når begge er på plads.
    - `EnrollActivity` — enrollment-formular (server-URL +
      enrollment-token + valgfrit device-navn). Genererer X25519-keypair
      via gomobile-laget, POST'er til serveren, gemmer device-token via
      Keystore.
    - `SetupActivity` — kdbx-picker via SAF (Storage Access Framework) +
      database-vælger der lister databases fra serveren.
    - `KeystoreTokenStore` — `EncryptedSharedPreferences` + Android
      Keystore master-key.
    - `DataStoreSyncStatePersistence` — Jetpack DataStore for sync-state
      (lastSeq + syncedAt) mellem app-starter.
    - `DatabaseConfigStore` — kdbx-URI + server-database-mapping.
    - `KdbxFile`-abstraktion: `PathKdbxFile` (tests) / `SafKdbxFile`
      (production via ContentResolver).
    - `SyncWorker` — periodisk WorkManager-CoroutineWorker.
  - `EncryptedPassphraseStore` + biometrisk opt-in på plads — krydser
    brugeren "husk password & synk i baggrunden" bekræftes identiteten
    med `BiometricPrompt`, og efter en vellykket sync gemmes
    masterpasswordet Keystore-krypteret (ikke længere klartekst i
    WorkManager) og den periodiske `SyncWorker` tændes. Keystore-nøglen
    kræver bevidst ikke per-brug-auth, så baggrunds-worker'en kan
    dekryptere uden bruger til stede; biometri gater opt-in'en, ikke
    hver læsning.
  - `ShareActivity` på plads — ejer-side v2 sharing fra app'en: list
    medlemmer, del med brugernavn, fjern medlem. Bruger den nye
    `mobile.WrapMasterKeyForShare`-binding (Argon2id → sealed-box til
    target-enhedens public-key) + `ApiClient.lookupUser/listShares/
    shareDatabase/unshareDatabase`. Server håndhæver owner-only (403 →
    pæn besked).
  - Mangler stadig: F-Droid-submit (metadata findes; reproducible build
    skal valideres + opdateres for minSdk 23 og nye deps).

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

Engangs-setup:

```sh
# 1. gomobile + gobind
go install golang.org/x/mobile/cmd/gomobile@latest
cd client
go get -tool golang.org/x/mobile/cmd/gobind   # registrér i go.mod

# 2. NDK r25c (gomobile er pt. konservativ overfor nyere NDK's; r25c er
#    valideret). Download fra https://dl.google.com/android/repository/
#    android-ndk-r25c-<os>.zip og udpak til $ANDROID_HOME/ndk/25.2.9519653/
```

Byg `.aar`:

```sh
export JAVA_HOME="/c/Program Files/Android/Android Studio/jbr"
export ANDROID_HOME="$HOME/AppData/Local/Android/Sdk"
export ANDROID_NDK_HOME="$ANDROID_HOME/ndk/25.2.9519653"
export PATH="$JAVA_HOME/bin:$PATH"

cd client
gomobile bind -androidapi 21 -target=android \
    -o ../android/libs/deltasync.aar \
    gitlab.com/Star95/keepass-deltasync/client/mobile

# Til :sync's compileOnly-classpath: udpak classes.jar
cd ../android/libs
unzip -o deltasync.aar classes.jar && mv classes.jar deltasync-classes.jar
```

`libs/`-filerne er gitignored. `:app`-modulet (når det skrives) vil
inkludere `.aar`'en med `implementation(files("../libs/deltasync.aar"))` —
inkl. JNI-`.so`-filerne. `:sync` bruger kun `deltasync-classes.jar` som
`compileOnly`-dep så `GomobileCryptoSession.kt` kan kompilere uden at
trække JNI-laget med ind i ren-JVM-tests.

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
