# F-Droid submission notes

DeltaSync er bygget med F-Droid som primær distributionsplatform i tankerne.
Dette dokument samler de praktiske detaljer omkring submission og
reproducible builds.

## Status

- **Submittet:** [fdroiddata MR !41661](https://gitlab.com/fdroid/fdroiddata/-/merge_requests/41661) (fra fork `Star95/fdroiddata`, branch `deltasync`). **Pipelinen er grøn hele vejen** — `fdroid build` bygger gomobile-bind'et i F-Droids egen container, og alle metadata-tjek (`fdroid lint`, `fdroid rewritemeta`, `schema validation`, `check apk`/`check source code`) passerer. Mangler kun en reviewers merge.
- **Metadata-fil:** [`/metadata/dk.bjoerckbraun.deltasync.yml`](../metadata/dk.bjoerckbraun.deltasync.yml) — én `Builds`-post for 0.3.1 (`commit: android/v0.3.1`); `CurrentVersion: 0.3.1` / `CurrentVersionCode: 4`. De gamle 0.1.0–0.3.0 blev aldrig publiceret og er fjernet. Filen er på kanonisk form (`fdroid rewritemeta` er idempotent) — INGEN kommentarer, build-flags i kanonisk rækkefølge (`sudo → gradle → prebuild → scanignore → ndk`), linjer ombrudt ved 80 tegn.
- **Fastlane-beskrivelser:** [`/fastlane/metadata/android/en-US/`](../fastlane/metadata/android/en-US/) — `title.txt`, `short_description.txt`, `full_description.txt`, changelogs `1.txt`–`4.txt`. F-Droid bruger DISSE til app-sidens tekst (yml-`Description:` er kun fallback). Kun en-US findes; ingen dansk listing endnu.
- **NDK** leveres af F-Droids build-server via `ndk: 25.2.9519653`-feltet (eksponeret som `$$NDK$$`) — ikke længere en `curl`-download.
- **gomobile/gobind pinnet** til `golang.org/x/mobile`-versionen fra `client/go.mod` (ikke `@latest`) — reproducerbarhedskrav.

### Validér de to metadata-jobs lokalt

`fdroid build` kan ikke køres uden for en provisioneret build-server, men de to format-jobs der typisk driller (`fdroid rewritemeta` + `schema validation`) kan valideres 100 % lokalt i Docker med samme `fdroid`-version som CI:

```sh
docker run --rm -v "$PWD":/repo -w /repo python:3.12-slim bash -c '
  apt-get update -qq && apt-get install -y -qq git          # fdroidserver kræver git i PATH
  pip install --quiet fdroidserver check-jsonschema
  export GIT_PYTHON_REFRESH=quiet
  touch config.yml
  fdroid rewritemeta dk.bjoerckbraun.deltasync              # diff mod original SKAL være tom
  check-jsonschema --schemafile schemas/metadata.json \
      metadata/dk.bjoerckbraun.deltasync.yml               # schema fra fdroiddata-repoet
'
```

## Build-konstruktion

F-Droid's build-server kører i en Debian-baseret container. Vores app
kræver et lille pre-build-step udover det almindelige Gradle-flow.
Recipen koder det i `.yml`'ens `sudo:`- og `prebuild:`-felter; det
svarer til:

```sh
# 1. (sudo:) Hent en Go-toolchain. client/go.mod kræver go 1.26.3 — for nyt
#    til Debians golang-go — så vi henter den officielle, pinnede tarball med
#    SHA-256-verifikation.
curl -fsSL https://go.dev/dl/go1.26.3.linux-amd64.tar.gz -o /tmp/go.tar.gz
echo "<sha256>  /tmp/go.tar.gz" | sha256sum -c -
tar -C /usr/local -xzf /tmp/go.tar.gz

# 2. (prebuild:) NDK kommer fra F-Droids ndk:-felt, eksponeret som $$NDK$$.
export PATH=/usr/local/go/bin:$PATH GOPATH=/tmp/go GOTOOLCHAIN=local
export ANDROID_NDK_HOME=$$NDK$$

# 3. Installér en SDK-platform: i prebuild-fasen er platforms/ stadig tom
#    (Gradle installerer dem først senere), så gomobile bind ville ellers
#    fejle med "failed to find android SDK platform".
sdkmanager "platforms;android-35" "build-tools;35.0.0"

# 4. gomobile/gobind pinnet til client/go.mod's x/mobile-version.
go install golang.org/x/mobile/cmd/gomobile@<pinned>
go install golang.org/x/mobile/cmd/gobind@<pinned>
export PATH=$GOPATH/bin:$PATH
gomobile init

# 5. Byg .aar fra Go-pakken (client/mobile). android/libs/ er gitignored →
#    mappen findes ikke i en frisk klon, så opret den først.
cd ../../client
mkdir -p ../android/libs
gomobile bind -androidapi 21 -target=android \
    -o ../android/libs/deltasync.aar \
    gitlab.com/Star95/keepass-deltasync/client/mobile

# 6. Udpak classes.jar til :sync's compileOnly-classpath.
cd ../android/libs
unzip -o deltasync.aar classes.jar
mv classes.jar deltasync-classes.jar

# 7. Standard Gradle-build (F-Droid kører selv assembleRelease).
```

De tre gomobile-genererede artefakter (`deltasync.aar`,
`deltasync-classes.jar`, `deltasync-sources.jar`) er `scanignore`'t i
recipen, fordi F-Droids binær-scanner kører EFTER prebuild og ellers
ville flagge dem — de bygges fra kilde i prebuild-trinnet. Se
[`dk.bjoerckbraun.deltasync.yml`](../metadata/dk.bjoerckbraun.deltasync.yml)
for den eksakte form.

## Reproducible builds

For at F-Droid's verification-server kan bekræfte at vores release-APK
matcher kildekoden:

1. Build skal være deterministisk. Vores Go-bind er deterministisk givet
   samme Go-version + NDK-version (vi pinner begge).
2. Gradle skal ikke embedde tidsstempler. Vi sætter
   `archivesBaseName` og bruger `useReleaseTimestamps = false`.
3. APK'en skal være v2-signeret med en stabil nøgle. Submitter'en
   leverer signing-konfig separat.
4. Ingen ProGuard/R8 endnu — vi har ikke en mappings-fil at fryse mod.
   Tilføjes når v1 er stabil.

## Inkluderede tredje-parts biblioteker

Alle skal være på F-Droid's tilladte source-liste (typisk Maven Central
eller Google Maven):

| Bibliotek | Version | Licens | Kilde |
|-----------|---------|--------|-------|
| kotpass | 0.13.0 | Apache-2.0 | Maven Central |
| OkHttp | 4.12.0 | Apache-2.0 | Maven Central |
| kotlinx.serialization | 1.7.3 | Apache-2.0 | Maven Central |
| kotlinx.datetime | 0.6.1 | Apache-2.0 | Maven Central |
| kotlinx.coroutines | 1.8.1 | Apache-2.0 | Maven Central |
| AndroidX core/appcompat/material | latest stable | Apache-2.0 | Google Maven |
| AndroidX datastore | 1.1.1 | Apache-2.0 | Google Maven |
| AndroidX security-crypto | 1.1.0-alpha06 | Apache-2.0 | Google Maven |
| AndroidX work-runtime | 2.9.1 | Apache-2.0 | Google Maven |

Den gomobile-genererede `.aar` indeholder kun vores egen Go-kode
plus standard Go runtime — ingen tredje-parts ikke-FOSS deps.

## Submission-procedure

Submission er gennemført som [fdroiddata MR !41661](https://gitlab.com/fdroid/fdroiddata/-/merge_requests/41661);
nedenstående er proceduren for fremtidige opdateringer.

Forken ligger i WSL: `~/fdroiddata` (remote `fork` = `Star95/fdroiddata`,
`origin` = upstream `fdroid/fdroiddata`).

1. Tag og push en ny release i monorepoet (`android/vX.Y.Z`) — det er
   `commit:`-feltet F-Droid bygger fra.
2. I `~/fdroiddata` på branch `deltasync`: opdatér
   `metadata/dk.bjoerckbraun.deltasync.yml` (ny `Builds`-post +
   `CurrentVersion`/`CurrentVersionCode`).
3. Kør `fdroid rewritemeta` + `check-jsonschema` lokalt (se Docker-snippet
   ovenfor) så `rewritemeta`- og `schema validation`-jobbene passerer.
4. `git push fork deltasync` → re-trigger MR-pipelinen. Når den er grøn,
   afventer det en reviewers merge.

Bemærk: `UpdateCheckMode: Tags ^android/v[\d.]+$` sikrer at F-Droid kun
følger `android/v*`-tags og ikke forveksler dem med `client/v*`
(desktop-klienten) i samme monorepo. `AutoUpdateMode: Version` (uden
pattern — `android/v%v`-syntaksen afvises af det nuværende schema)
opretter selv nye build-poster fra de matchede tags.

## Inspirationsmateriale

- [F-Droid metadata-format](https://f-droid.org/en/docs/Build_Metadata_Reference/)
- [Reproducible builds-guide](https://f-droid.org/en/docs/Reproducible_Builds/)
