# keepass-deltasync-android

Android-klient. Sync-service-only — ingen indbygget entry-editor. KeePassXC's
egne kdbx-filer redigeres af brugerens foretrukne Android-KeePass-app
([Keepass2Android](https://github.com/PhilippC/keepass2android) eller
[KeePassDX](https://github.com/Kunzisoft/KeePassDX)), og DeltaSync-app'en
holder `.kdbx`-filen synkroniseret i baggrunden via WorkManager.

- **Licens:** GPL-3.0-or-later
- **Distribution:** [Google Play](https://play.google.com/store/apps/details?id=dk.bjoerckbraun.deltasync),
  [F-Droid](https://f-droid.org/packages/dk.bjoerckbraun.deltasync/) og
  [GitHub Releases](https://github.com/HBBSoftware/Keepass-DeltaSync/releases)
  (via Obtainium). Alle tre kanaler er signeret med projektets egen nøgle, så
  man kan skifte mellem dem uden at afinstallere.
- **Status:** udgivet og i brug. Ændringer pr. version står i
  [`CHANGELOG.md`](../CHANGELOG.md).

## Hvad app'en kan

- **Tilmelding af enheden** (`EnrollActivity`): server-URL + éngangs
  enrollment-token, eller **scan QR-koden** fra web-admin-panelet
  (`deltasync://enroll?server=…&token=…`). Enheden får sit eget
  X25519-nøglepar; device-token gemmes i Android Keystore.
- **Vælg database** (`SetupActivity`): `.kdbx`-filen vælges via Storage
  Access Framework, og den kobles til en database på serveren.
- **Synkronisering** (`MainActivity` + `SyncWorker`): "Sync now" fra
  status-skærmen, og periodisk i baggrunden via WorkManager.
- **Baggrunds-sync med biometri**: krydser man "husk password & synk i
  baggrunden" af, bekræftes identiteten med `BiometricPrompt`, og
  masterpasswordet gemmes Keystore-krypteret, så `SyncWorker` kan køre uden
  bruger til stede. Biometrien gater opt-in'en, ikke hver læsning.
- **Deling** (`ShareActivity`): ejeren kan dele en database med en anden
  bruger, se medlemmer og fjerne dem. Master-nøglen pakkes med sealed-box til
  modtagerens enhed; serveren håndhæver at kun ejeren kan dele.

Selve posterne redigeres i brugerens foretrukne KeePass-app — app'en har
bevidst ingen editor.

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

JVM-only modul med canonical-typer, kotpass-mapper og sync-motoren. Tests kører
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

- **F-Droid** (optaget 2026-09-10, MR !41661): reproducible builds, fuld open
  source-toolchain. Kræver at vi undgår Google Play Services — ingen FCM
  push, så sync drives af WorkManager-polling med rimelig interval
  (15–30 min) plus manuel "sync now" fra UI. Nye udgivelser samles op af
  `fdroid checkupdates` fra `android/v*`-tags uden ny MR, men hver version
  skal have vores signerede APK på GitHub og skal reproducere
  bit-for-bit. Se [`F-DROID.md`](F-DROID.md).
- **Obtainium** (tilgængelig nu): signeret APK på GitHub Releases, som
  Obtainium selv holder opdateret. Samme signatur som F-Droid-kanalen.
  Se nedenfor.
- **Google Play**: AAB uploadet med **vores egen nøgle** til Play App
  Signing, så Play-versionen har samme signatur som F-Droid og Obtainium.

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

**Du kan skifte frit mellem Obtainium og F-Droid.** F-Droid udgiver netop
denne APK — den er reproducerbar, så de transplanterer vores signatur i
stedet for at signere med deres egen nøgle (`Binaries:` +
`AllowedAPKSigningKeys:` i recipen). Begge kanaler leverer dermed samme
signatur, og Android opdaterer hen over skiftet uden afinstallation.
Appdata — enrollment, gemt password, valgt `.kdbx`-fil — overlever.

### Udgiv en ny Obtainium-release

CI bygger APK'en, men signerer den ikke: signeringsnøglen er det eneste der
binder fremtidige opdateringer til projektet, og CI-variabler kan læses af
enhver Maintainer og af ethvert kompromitteret job. Derfor bliver
`keystore.properties` + `.jks` liggende lokalt (begge gitignored), og
[`publish-release.sh`](publish-release.sh) signerer på din maskine:

```sh
# 1. Bump versionCode + versionName i app/build.gradle.kts, commit, tag:
git tag android/v0.4.0 && git push origin android/v0.4.0

# 2. Vent på pipelinen. Jobbet build:android lægger
#    dist/DeltaSync-0.4.0-unsigned.apk som artefakt — hent og udpak den
#    i repo-roden.

# 3. Signér lokalt og upload til GitHub Releases
#    (og GitLab hvis GITLAB_TOKEN er sat):
GITHUB_TOKEN=ghp_xxx ./publish-release.sh
```

Vil du hellere bygge alt lokalt, så kør `./gradlew :app:assembleRelease` i
stedet for trin 2 (`JAVA_HOME` = Android Studios jbr) — scriptet tager imod
både en usigneret CI-APK og en færdigsigneret lokal build. Er APK'en usigneret,
bliver den zipalignet og signeret undervejs.

Scriptet nægter at uploade hvis APK'en er usigneret, hvis dens indbyggede
`versionName`/`versionCode` ikke matcher `build.gradle.kts` (typisk en glemt
genbygning), eller hvis tagget ikke findes. `DRY_RUN=1` kører alle tjek uden
at uploade noget.

## Hvad er ikke i scope (forblevet på desktop)

- Egen entry-editor / UI til at læse passwords
- `keepassxc-cli` integration (kører ikke på Android)
- Daemon mode (Android har WorkManager i stedet for fsnotify)
