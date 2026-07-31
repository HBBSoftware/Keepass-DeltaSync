# F-Droid submission notes

DeltaSync er bygget med F-Droid som primær distributionsplatform i tankerne.
Dette dokument samler de praktiske detaljer omkring submission og
reproducible builds.

## Status

- **Submittet:** [fdroiddata MR !41661](https://gitlab.com/fdroid/fdroiddata/-/merge_requests/41661) (fra fork `Star95/fdroiddata`, branch `deltasync`). **Pipelinen er grøn hele vejen** — `fdroid build` bygger gomobile-bind'et i F-Droids egen container, og alle metadata-tjek (`fdroid lint`, `fdroid rewritemeta`, `schema validation`, `check apk`/`check source code`) passerer.
- **Alle reviewkrav besvaret (2026-07-29).** linsuis punkter er lukket i
  rækkefølge: summary/description fjernet (kommer fra fastlane), Go bygges fra
  kilde via `go`-srclib'en, build-trinnene flyttet fra `prebuild:` til
  `build:`, commit pinnet til fuld hash, mere specifikke kategorier, og shell-
  variablerne i `build:` skrevet ud. Sidste krav er `Binaries` +
  `AllowedAPKSigningKeys` til reproducible build — se afsnittet nedenfor.
- **Go-srclib'en (implementeret 2026-07-28):** der findes en **Go-srclib**
  i fdroiddata — `srclibs/go.yml` (lowercase; let at overse). Den kloner
  `golang/go` og sætter `GOTOOLCHAIN=local` i `go.env`. Mønsteret linsui
  henviste til findes i `metadata/com.android.xrayfa.yml` (også en
  gomobile-app): `srclibs: - go@go1.X.Y`, derefter
  `pushd $$go$$/src && ./make.bash && popd` og `export GOROOT=$$go$$`.
  Debians `golang-go` fra trixie-backports installeres kun som
  **bootstrap-toolchain** til `make.bash`. Vi bygger `go1.26.3`.
- **`client/go.mod` slækket fra `go 1.26.3` til `go 1.26.0`** — den eksakte
  patch-version var blot den toolchain der tilfældigvis var installeret da
  `go mod tidy` sidst kørte, og den ville låse builds til én patch.
  Verificeret med `GOTOOLCHAIN=go1.26.0`: `go build`, `go vet` og alle tests
  passerer. Bemærk at man IKKE kan gå til 1.24/1.23:
  `golang.org/x/crypto@v0.52.0` og den pinnede `golang.org/x/mobile` kræver
  begge ≥ 1.25.
- **Buildserveren er Debian Trixie** (`config.vm.box = "debian/trixie64"` i
  fdroidservers `buildserver/Vagrantfile`), og backports er allerede aktiveret
  (`Suites: trixie trixie-updates trixie-backports` i
  `provision-apt-get-install`) — recipes skal derfor IKKE selv tilføje en
  `sources.list`-linje, bare bruge `-t trixie-backports`.
- **Metadata-fil:** [`/metadata/dk.bjoerckbraun.deltasync.yml`](../metadata/dk.bjoerckbraun.deltasync.yml) — én `Builds`-post for 0.4.1 (`commit:` = fuld hash, ikke tagnavn); `CurrentVersion: 0.4.1` / `CurrentVersionCode: 6`. De gamle 0.1.0–0.4.0 blev aldrig publiceret og er fjernet. Filen er på kanonisk form — kør ALTID `fdroid rewritemeta` og brug dens output som facit; den vil fx have lange `sudo:`/`build:`-kommandoer på ÉN linje, ikke ombrudt ved 80 tegn.
- **`Binaries:` står med trailing space og værdien på næste linje — `AllowedAPKSigningKeys:` gør IKKE.** Det ser ud som en inkonsekvens, men det er hvad CI'ens `rewritemeta` kræver: `Binaries:` er en URL med `%v`-pladsholdere der ombrydes, mens signeringsnøglen skal stå inline uanset længde. `fdroid lint` advarer om den trailing space på `Binaries:`, men exit-koden er uændret, så jobbet passerer. `AllowedAPKSigningKeys` som YAML-liste (`- <hash>`, som i `com.nononsenseapps.feeder.yml`) bliver skrevet om til skalar — brug skalaren.
- **Ingen `AutoName:`/`Description:` i yml'en** — linsui: *"Don't add summary and description here. Add them in fastlane structure in your repo and they will be pulled from there."* App-navn og beskrivelse kommer udelukkende fra fastlane.
- **Fastlane-beskrivelser:** [`/fastlane/metadata/android/en-US/`](../fastlane/metadata/android/en-US/) — `title.txt`, `short_description.txt`, `full_description.txt`, changelogs `1.txt`–`6.txt`. F-Droid bruger DISSE til app-sidens tekst. Kun en-US findes; ingen dansk listing endnu.
- **NDK** leveres af F-Droids build-server via `ndk: 25.2.9519653`-feltet (eksponeret som `$$NDK$$`) — ikke længere en `curl`-download.
- **gomobile/gobind pinnet** til `golang.org/x/mobile`-versionen fra `client/go.mod` (ikke `@latest`) — reproducerbarhedskrav.

## Reproducible build

Fra 0.4.1 er release-APK'en **byte-for-byte identisk** med det F-Droid selv
bygger fra samme commit, så F-Droid kan udgive vores egen signerede APK
(`Binaries:` + `AllowedAPKSigningKeys:`) i stedet for at signere med deres
nøgle. Det betyder at Obtainium- og F-Droid-kanalen deler signatur, og at
brugere kan skifte mellem dem uden at afinstallere.

Bekræftet på F-Droids egen infrastruktur (MR !41661, job `fdroid build`):
`dk.bjoerckbraun.deltasync_6.apk` og vores `build:android`-artefakt har begge
sha256 `09963ea3f92dc593fa61d13b69f18f552f171bdd5d861b5f10571e718149a74d`, og
jobbet logger `compared built binary to supplied reference binary
successfully` + `supplied reference binary has allowed signer 476e556a…`.

**Fire ting brød reproducerbarheden.** Alle fire lå i `libgojni.so`; resten af
APK'en (dex, ressourcer, `.so` fra AndroidX) matchede fra starten:

1. **Go-patchversion.** CI kørte `golang:1.26-trixie` (flydende, gav 1.26.5),
   opskriften bygger `go@go1.26.3`. Versionsstrengen ligger i binæren. CI er
   nu pinnet til `golang:1.26.3-trixie` — **bump den sammen med `srclibs:`,
   aldrig alene**.
2. **Checkout-stien og gomobiles temp-dir.** Løst med `-trimpath` på
   `gomobile bind` (fjerner også det tilfældige `/tmp/gomobile-work-NNN`).
3. **gomobiles `replace`-sti.** gomobile genererer et midlertidigt
   `gobind`-modul der `replace`r vores modul ved absolut sti, og den sti
   havner i Go's build info — `-trimpath` rører den ikke. CI bygger derfor fra
   `/home/vagrant/build/dk.bjoerckbraun.deltasync`, som er hvor `fdroid build`
   altid arbejder.
4. **NDK-stien.** cgo giver NDK-stien til clang, som skriver den i DWARF-
   linjetabellen. SDK'et skal derfor ligge i `/opt/android-sdk` (hardcodet i
   fdroidservers `buildserver/Vagrantfile`). **Et symlink er ikke nok** —
   clang resolver sin egen binærsti for at finde `lib64/clang/*/include`, så
   den fysiske sti lækker igennem. SDK'et installeres derfor direkte i `/opt`
   og caches ikke (GitLab kan kun cache stier under projektmappen).

**Signering:** `android/publish-release.sh` bruger
`apksigner sign --alignment-preserved`. Uden flaget om-padder apksigner alle
stored ZIP-entries under signeringen (16K for `.so`, 4 bytes ellers), og
`apksigcopier` — som F-Droid bruger til at transplantere vores signatur over
på deres egen build — kan ikke gendanne den padding. Verifikationen fejler så,
selvom builden er identisk. APK'en består stadig `zipalign -c -P 16 -v 4`.

### Verificér reproducerbarheden uden Docker

F-Droids MR-pipeline efterlader deres byggede APK som artifact, så de to kan
sammenlignes direkte:

```sh
# deres build (fra jobbet "fdroid build" i MR-pipelinen)
curl -sL -o fd.zip "https://gitlab.com/Star95/fdroiddata/-/jobs/<JOB_ID>/artifacts/download"
unzip -p fd.zip tmp/dk.bjoerckbraun.deltasync_<VERSIONCODE>.apk > fdroid.apk

# vores build (fra jobbet "build:android")
curl -sL -o ci.zip "https://gitlab.com/Star95/keepass-deltasync/-/jobs/<JOB_ID>/artifacts/download"
unzip -p ci.zip "dist/DeltaSync-<VERSION>-unsigned.apk" > ours.apk

sha256sum fdroid.apk ours.apk        # skal være ens

# og at signaturen kan transplanteres (det F-Droid gør ved udgivelse):
pip install apksigcopier
apksigcopier compare DeltaSync-<VERSION>.apk --unsigned fdroid.apk
```

Afviger de, så pak `libgojni.so` ud af begge og kig efter stier:
`strings lib/arm64-v8a/libgojni.so | grep -E '/(builds|home|opt|tmp)/'`.

### Validér de tre metadata-jobs lokalt

`fdroid build` kan ikke køres uden for en provisioneret build-server, men de
tre metadata-jobs (`fdroid lint`, `fdroid rewritemeta`, `schema validation`)
kan køres lokalt. `fdroid` er allerede installeret i WSL (`/usr/bin/fdroid`);
kør fra Git Bash på Windows:

> **Lokal validering er en indikation, ikke et facit.** WSL har fdroidserver
> **2.4.5**, mens CI-jobbene henter **master** friskt ved hver kørsel
> (`curl .../fdroidserver/-/archive/master/...`). De to er ikke enige om
> formateringen: 2.4.5 ombryder `AllowedAPKSigningKeys:` til to linjer og
> kalder det idempotent, hvor master kræver den på én linje. Det kostede en
> rød pipeline. Er den lokale kørsel grøn og CI rød, så er **CI facit** — læs
> `rewritemeta`-jobbets diff og anvend den ordret.

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
# 1. (sudo:) Debians Go — kun som BOOTSTRAP til at bygge Go fra kilde.
#    Backports er allerede aktiveret på buildserveren; tilføj ikke sources.list.
#    `unzip` installeres IKKE — den findes i imaget, og linsui bad om at
#    linjen blev fjernet (tom `suggestion:-0+0` på MR'en, note 3617176222).
apt-get update
apt-get install -y -t trixie-backports golang-go

# 2. (prebuild:) Installér en SDK-platform: platforms/ er stadig tom på dette
#    tidspunkt (Gradle installerer dem først senere), så gomobile bind ville
#    ellers fejle med "failed to find android SDK platform".
sdkmanager "platforms;android-35" "build-tools;35.0.0"

# 3. (build:) Byg Go fra kilde. srclibs: go@go1.26.3 kloner golang/go og
#    eksponerer den som $$go$$; srclibbens Prepare-step sætter selv
#    GOTOOLCHAIN=local i go.env, så Go ikke henter en anden toolchain.
pushd $$go$$/src && ./make.bash && popd
export GOROOT=$$go$$ GOPATH="$HOME/go"
export PATH="$GOROOT/bin:$GOPATH/bin:$PATH"
export ANDROID_NDK_HOME=$$NDK$$        # fra ndk:-feltet

# 4. gomobile/gobind UDEN @version: kommandoerne kører efter `cd ../../client`,
#    så versionen kommer fra client/go.mod — stadig pinnet, blot implicit
#    (linsui: "Don't use variables when it's not necessary...").
cd ../../client
go install golang.org/x/mobile/cmd/gomobile
go install golang.org/x/mobile/cmd/gobind
gomobile init

# 5. Byg .aar fra Go-pakken (./mobile). android/libs/ er gitignored →
#    mappen findes ikke i en frisk klon, så opret den først. `-trimpath` er et
#    reproducerbarhedskrav — se "Reproducible build".
mkdir -p ../android/libs
gomobile bind -trimpath -androidapi 21 -o deltasync.aar ./mobile
mv deltasync.aar ../android/libs/

# 6. Udpak classes.jar til :sync's compileOnly-classpath. `unzip -p` skriver til
#    stdout, så udpakning og omdøbning bliver én kommando i stedet for to.
cd ../android/libs
unzip -p deltasync.aar classes.jar > deltasync-classes.jar

# 7. Standard Gradle-build (F-Droid kører selv assembleRelease).
```

**Hold linjerne korte — men ikke med shell-variabler.** `fdroid rewritemeta`
(2.4.5) ombryder et felt der overstiger ~80 tegn, og dens egen ombrydning
efterlader **trailing spaces** som `fdroid lint` derefter advarer om (kun en
advarsel — lint exit'er 0, så jobbet passerer; se `Binaries:` og
`AllowedAPKSigningKeys:` i yml'en, der begge ER ombrudt sådan). Recipen brugte
tidligere `XM`/`XV`/`AAR`/`PKG` til at holde `build:`-linjerne korte, men
linsui bad om at få dem fjernet (*"Don't use variables when it's not
necessary..."*) — så løsningen er at gøre kommandoerne kortere i stedet: `cd`
ind i mappen først og bruge relative stier (`./mobile`, `-o deltasync.aar` +
`mv`) frem for absolutte pakke- og outputstier.

**Hvorfor `build:` og ikke `prebuild:`** (linsui: *"Move build steps to
build."*): F-Droids binær-scanner kører EFTER `prebuild:` men FØR `build:`.
Lå gomobile-trinnene i `prebuild:`, ville scanneren finde de genererede
`deltasync.aar` / `deltasync-classes.jar` / `deltasync-sources.jar` og flagge
dem som prebuilt binaries — hvilket den gamle recipe måtte dæmpe med
`scanignore:`. Flyttet til `build:` opstår artefakterne først efter scanningen,
og `scanignore:` er derfor fjernet helt. Ser scanneren dem alligevel, er det
`scanignore:` der skal tilbage — ikke `prebuild:`.

## Hvad der IKKE var et problem for reproducerbarheden

Nyttigt at vide, så man ikke fejlsøger de forkerte steder (verificeret ved at
sammenligne entry for entry mellem F-Droids build og vores):

- **Gradle-siden var deterministisk fra starten.** `classes*.dex`,
  `resources.arsc` og AndroidX' `.so`-filer matchede i første forsøg. AGP
  nulstiller selv ZIP-tidsstempler (alle entries står som `1981-01-01 01:01`),
  så der er ingen `archivesBaseName`- eller timestamp-flag at sætte.
- **R8/ProGuard er slået fra** (`isMinifyEnabled = false`), så der er ingen
  mappings-fil at fryse. Slås minificering til senere, er det et nyt
  reproducerbarhedsproblem der skal verificeres forfra.
- **NDK-versionen** kommer fra `ndk:`-feltet og var aldrig i tvivl — det var
  NDK'ets *sti*, ikke dets version, der lækkede ind i `libgojni.so`.

Alt der faktisk brød reproducerbarheden lå i `libgojni.so` og er beskrevet
under [Reproducible build](#reproducible-build).

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
