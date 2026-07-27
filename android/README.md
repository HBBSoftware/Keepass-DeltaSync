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
      Keystore. Kan også **scanne en QR-kode** (ZXing) fra web-admin-panelet
      — `deltasync://enroll?server=…&token=…` — der udfylder felterne, så
      brugeren slipper for at taste. Parseren ligger i `:sync`
      (`EnrollUriParser`, ren JVM + unit-testet).
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
  manuel "sync now" fra UI. Status: se [`F-DROID.md`](F-DROID.md).
- **Obtainium** (tilgængelig nu): signeret APK på GitHub Releases, som
  Obtainium selv holder opdateret. Se nedenfor.
- **Play** (senere): standard Android-signering. Kan tilføjes uden ændringer
  af kerne-arkitekturen.

### Installér med Obtainium

[Obtainium](https://github.com/ImranR98/Obtainium) henter appen direkte fra
vores GitHub Releases og giver besked når der er en ny version — ingen
app-butik involveret.

Tilføj appen med dette deep-link (åbn det på telefonen med Obtainium
installeret):

[Tilføj DeltaSync til Obtainium](obtainium://app/%7B%22id%22%3A%22dk.bjoerckbraun.deltasync%22%2C%22url%22%3A%22https%3A%2F%2Fgithub.com%2FHBBSoftware%2FKeepass-DeltaSync%22%2C%22author%22%3A%22HBBSoftware%22%2C%22name%22%3A%22DeltaSync%22%2C%22preferredApkIndex%22%3A0%2C%22additionalSettings%22%3A%22%7B%5C%22filterReleaseTitlesByRegEx%5C%22%3A%5C%22%5EDeltaSync%20Android%5C%22%2C%5C%22versionExtractionRegEx%5C%22%3A%5C%22%5B0-9.%5D%2B%24%5C%22%2C%5C%22about%5C%22%3A%5C%22End-to-end%20encrypted%20KeePass%20sync.%20Requires%20your%20own%20self-hosted%20DeltaSync%20server.%5C%22%7D%22%7D)

Eller opret den manuelt i Obtainium med disse felter:

| Felt | Værdi |
|------|-------|
| App source URL | `https://github.com/HBBSoftware/Keepass-DeltaSync` |
| Filter release titles by regex | `^DeltaSync Android` |
| Version extraction regex | `[0-9.]+$` |

De to regex'er er nødvendige fordi dette er et **monorepo**: samme repo
udgiver også `client/vX.Y.Z`-releases. Titel-filteret sorterer dem fra, og
versions-regex'en trækker `0.3.1` ud af tagget `android/v0.3.1` (Obtainium
ville ellers vise hele tag-navnet som versionsnummer). Releases uden en
`.apk` springes over af Obtainium i forvejen, så filteret er et sikkerhedsnet
snarere end et krav.

**Signatur.** Alle Obtainium-APK'er signeres med projektets egen nøgle:

```
$ apksigner verify --print-certs DeltaSync-<version>.apk
Signer #1 certificate SHA-256 digest: 476e556a66d2614c6e43509fcc081ff54f1334e39f0ca1162e606c51ebd50d68
```

⚠️ **Skifter du senere til F-Droid-versionen, kan Android ikke opdatere
hen over det** — F-Droid signerer med deres egen nøgle, og et
signaturskifte kræver afinstallation. Det betyder at appdata (enrollment,
gemt password, valgt .kdbx-fil) går tabt og enheden skal enrolles igen via
QR. Vælg én kanal og bliv på den.

### Udgiv en ny Obtainium-release

Signeringsnøglen ligger kun lokalt (`keystore.properties` + `.jks` er
gitignored), så byg og signér på egen maskine og upload derefter med
[`publish-release.sh`](publish-release.sh):

```sh
# 1. Bump versionCode + versionName i app/build.gradle.kts, commit, tag:
git tag android/v0.4.0 && git push origin android/v0.4.0

# 2. Byg den signerede APK (JAVA_HOME = Android Studios jbr):
./gradlew :app:assembleRelease

# 3. Upload til GitHub Releases (og GitLab hvis GITLAB_TOKEN er sat):
GITHUB_TOKEN=ghp_xxx ./publish-release.sh
```

Scriptet nægter at uploade hvis APK'en er usigneret, hvis dens indbyggede
`versionName`/`versionCode` ikke matcher `build.gradle.kts` (typisk en glemt
genbygning), eller hvis tagget ikke findes. `DRY_RUN=1` kører alle tjek uden
at uploade noget.

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
