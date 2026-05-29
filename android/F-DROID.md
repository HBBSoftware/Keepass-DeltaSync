# F-Droid submission notes

DeltaSync er bygget med F-Droid som primær distributionsplatform i tankerne.
Dette dokument samler de praktiske detaljer omkring submission og
reproducible builds.

## Status

- **Metadata-fil:** [`/metadata/dk.bjoerckbraun.deltasync.yml`](../metadata/dk.bjoerckbraun.deltasync.yml)
- **Fastlane-beskrivelser:** [`/fastlane/metadata/android/en-US/`](../fastlane/metadata/android/en-US/)
- **Ikke endnu submittet** — venter på en stabil v0.1.0-tag med UI på plads.

## Build-konstruktion

F-Droid's build-server kører i en Debian-baseret container. Vores app
kræver et lille pre-build-step udover det almindelige Gradle-flow:

```sh
# 1. Engang per CI-runner: hent Go-mobile binær.
go install golang.org/x/mobile/cmd/gomobile@latest

# 2. Pin'et NDK r25c. F-Droid's standard er typisk nyere; vi henter eksplicit.
curl -sL https://dl.google.com/android/repository/android-ndk-r25c-linux.zip \
    -o /tmp/ndk.zip
unzip /tmp/ndk.zip -d /tmp/

# 3. Byg .aar fra Go-pakken (client/mobile).
cd client
gomobile bind -androidapi 21 -target=android \
    -o ../android/libs/deltasync.aar \
    gitlab.com/Star95/keepass-deltasync/client/mobile

# 4. Udpak classes.jar til :sync's compileOnly-classpath.
cd ../android/libs
unzip -o deltasync.aar classes.jar
mv classes.jar deltasync-classes.jar

# 5. Standard Gradle-build.
cd ..
./gradlew :app:assembleRelease
```

F-Droid's `metadata/*.yml` koder dette flow i `prebuild`-feltet. Se
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
4. Ingen ProGuard/R8 i v0.1 — vi har ikke en mappings-fil at fryse mod.
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

## Submission-procedure (når v0.1.0-tag er klar)

1. Tag release: `git tag -s v0.1.0 -m "Initial release"`.
2. Push tag: `git push --tags`.
3. Fork [fdroiddata](https://gitlab.com/fdroid/fdroiddata).
4. Kopiér `metadata/dk.bjoerckbraun.deltasync.yml` ind i forken.
5. Test lokalt: `fdroid build --verbose dk.bjoerckbraun.deltasync`.
6. Submit som merge-request i fdroiddata.

## Inspirationsmateriale

- [F-Droid metadata-format](https://f-droid.org/en/docs/Build_Metadata_Reference/)
- [Reproducible builds-guide](https://f-droid.org/en/docs/Reproducible_Builds/)
