# F-Droid submission notes

DeltaSync er bygget med F-Droid som primær distributionsplatform i tankerne.
Dette dokument samler de praktiske detaljer omkring submission og
reproducible builds.

## Status

- **Submittet:** [fdroiddata MR !41661](https://gitlab.com/fdroid/fdroiddata/-/merge_requests/41661) (fra fork `Star95/fdroiddata`, branch `deltasync`). **Pipelinen er grøn hele vejen** — `fdroid build` bygger gomobile-bind'et i F-Droids egen container, og alle metadata-tjek (`fdroid lint`, `fdroid rewritemeta`, `schema validation`, `check apk`/`check source code`) passerer.
- **⚠ Blokeret på os, ikke på F-Droid (tjekket 2026-07-27):** reviewer
  **@linsui** bad 2026-06-30 om tre ændringer der aldrig blev besvaret — derfor
  labelen `waiting-on-response`. Checkboksene i MR-beskrivelsen er sat
  2026-07-27 (5 Required + 2 Strongly Recommended; de 3 Suggested står
  bevidst tomme), men **det løser ikke de tre krav**:

  1. *"Don't add summary and description here. Add them in fastlane structure
     in your repo and they will be pulled from there."* → fjern `AutoName:`
     og `Description:` fra `metadata/dk.bjoerckbraun.deltasync.yml`.
     Ufarligt: `/fastlane/metadata/android/en-US/` er komplet.
  2. *"Build Go from source. You can find examples in other metadata."* → den
     tunge. Recipen henter i dag en **prebuilt** Go-tarball fra go.dev, hvilket
     bryder F-Droids no-prebuilt-binaries-politik. Se
     `metadata/com.nutomic.syncthingandroid.yml` for det gængse mønster:
     `apt-get install -y -t bookworm-backports golang-go`. Der findes ingen
     generisk `Go.yml`-srclib i fdroiddata.
  3. Diff-tråd på `prebuild:`: *"Move build steps to build."* → flyt
     gomobile-bind-trinnene til `build:`-feltet.

  Desuden er branchen ~2200 commits bagud for `master` og skal rebases.

- **LØSNINGEN på krav 2 (verificeret 2026-07-27):** F-Droids build-server er
  Debian **Trixie** (`config.vm.box = "debian/trixie64"` i fdroidservers
  `buildserver/Vagrantfile`), og **trixie-backports har `golang-go` 1.26**
  (`2:1.26~1~bpo13+1`). Der skal altså ikke nedgraderes noget — Debians egen
  Go er ny nok. Den eneste blokering var at `client/go.mod` erklærede
  `go 1.26.3`, altså en eksakt patch-version; det var blot den toolchain der
  tilfældigvis var installeret da `go mod tidy` sidst kørte. Er Debians pakke
  bygget fra en lavere patch, ville buildet fejle med
  `requires go >= 1.26.3`. **`go.mod` er derfor slækket til `go 1.26.0`** —
  verificeret med `GOTOOLCHAIN=go1.26.0`: `go build`, `go vet` og alle tests
  passerer. Bemærk at det IKKE kunne løses ved at gå til 1.24/1.23:
  `golang.org/x/crypto@v0.52.0` og den pinnede `golang.org/x/mobile` kræver
  begge ≥ 1.25, og bookworm-backports har kun 1.23.
- **Metadata-fil:** [`/metadata/dk.bjoerckbraun.deltasync.yml`](../metadata/dk.bjoerckbraun.deltasync.yml) — én `Builds`-post for 0.4.0 (`commit: android/v0.4.0`); `CurrentVersion: 0.4.0` / `CurrentVersionCode: 5`. De gamle 0.1.0–0.3.1 blev aldrig publiceret og er fjernet. Filen er på kanonisk form — kør ALTID `fdroid rewritemeta` og brug dens output som facit; den vil fx have lange `sudo:`/`build:`-kommandoer på ÉN linje, ikke ombrudt ved 80 tegn.
- **Ingen `AutoName:`/`Description:` i yml'en** — linsui: *"Don't add summary and description here. Add them in fastlane structure in your repo and they will be pulled from there."* App-navn og beskrivelse kommer udelukkende fra fastlane.
- **Fastlane-beskrivelser:** [`/fastlane/metadata/android/en-US/`](../fastlane/metadata/android/en-US/) — `title.txt`, `short_description.txt`, `full_description.txt`, changelogs `1.txt`–`5.txt`. F-Droid bruger DISSE til app-sidens tekst. Kun en-US findes; ingen dansk listing endnu.
- **NDK** leveres af F-Droids build-server via `ndk: 25.2.9519653`-feltet (eksponeret som `$$NDK$$`) — ikke længere en `curl`-download.
- **gomobile/gobind pinnet** til `golang.org/x/mobile`-versionen fra `client/go.mod` (ikke `@latest`) — reproducerbarhedskrav.

### Validér de tre metadata-jobs lokalt

`fdroid build` kan ikke køres uden for en provisioneret build-server, men de
tre metadata-jobs (`fdroid lint`, `fdroid rewritemeta`, `schema validation`)
kan valideres 100 % lokalt. `fdroid` er allerede installeret i WSL
(`/usr/bin/fdroid`); kør fra Git Bash på Windows:

```sh
wsl -e sh -lc '
  export GIT_PYTHON_REFRESH=quiet
  SRC=/mnt/d/Kunder/Data/Hans/Keepass-deltasync/metadata/dk.bjoerckbraun.deltasync.yml
  d=$(mktemp -d); mkdir -p $d/metadata; cp $SRC $d/metadata/; cd $d
  touch config.yml                       # fdroid nægter at køre uden

  fdroid lint dk.bjoerckbraun.deltasync  # tavshed = ingen fejl

  # rewritemeta SKAL være en no-op. Er den ikke, så kopiér DENS output
  # tilbage over kildefilen — den er facit, ikke din håndformatering.
  fdroid rewritemeta dk.bjoerckbraun.deltasync
  diff -u $SRC metadata/dk.bjoerckbraun.deltasync.yml && echo IDEMPOTENT

  curl -fsS -o m.json https://gitlab.com/fdroid/fdroiddata/-/raw/master/schemas/metadata.json
  python3 -m check_jsonschema --schemafile m.json metadata/dk.bjoerckbraun.deltasync.yml
'
```

Mangler `check-jsonschema`: `wsl -e sh -lc 'python3 -m pip install check-jsonschema'`.

Fælde: rewritemeta ombryder **ikke** lange `sudo:`/`build:`-kommandoer ved 80
tegn — den vil have dem på én linje. Ombryder du dem selv for læsbarhedens
skyld, fejler `rewritemeta`-jobbet i CI.

## Build-konstruktion

F-Droids build-server kører Debian Trixie. Vores app kræver et gomobile-trin
udover det almindelige Gradle-flow. Det ligger i `.yml`'ens `sudo:`- og
`build:`-felter; det svarer til:

```sh
# 1. (sudo:) Go fra Debian, IKKE en downloadet tarball. F-Droid tillader ikke
#    prebuilt binaries i buildet — derfor backports-pakken, som Debian selv
#    har bygget fra kilde. trixie-backports har 1.26, og client/go.mod kræver
#    kun `go 1.26.0`, så den passer.
echo "deb https://deb.debian.org/debian trixie-backports main" \
    > /etc/apt/sources.list.d/backports.list
apt-get update
apt-get install -y unzip
apt-get install -y -t trixie-backports golang-go

# 2. (build:) NDK kommer fra F-Droids ndk:-felt, eksponeret som $$NDK$$.
#    GOTOOLCHAIN=local forhindrer Go i at hente en anden toolchain over nettet.
export GOPATH=/tmp/go GOTOOLCHAIN=local
export ANDROID_NDK_HOME=$$NDK$$

# 3. Installér en SDK-platform: platforms/ er stadig tom på dette tidspunkt
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

**Hvorfor `build:` og ikke `prebuild:`** (linsui: *"Move build steps to
build."*): F-Droids binær-scanner kører EFTER `prebuild:` men FØR `build:`.
Lå gomobile-trinnene i `prebuild:`, ville scanneren finde de genererede
`deltasync.aar` / `deltasync-classes.jar` / `deltasync-sources.jar` og flagge
dem som prebuilt binaries — hvilket den gamle recipe måtte dæmpe med
`scanignore:`. Flyttet til `build:` opstår artefakterne først efter scanningen,
og `scanignore:` er derfor fjernet helt. Ser scanneren dem alligevel, er det
`scanignore:` der skal tilbage — ikke `prebuild:`.

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
