# keepass-deltasync-android

Android-klient. Genbruger Go-sync-biblioteket via `gomobile bind` (.aar), wrappet i en lille Kotlin/Java-app. Til selve .kdbx-mergen bruges [kotpass](https://github.com/keemobile/kotpass) — eller en simpel entry-level merge implementeret i Go, hvis kotpass ikke håndterer det godt nok.

- **Licens:** GPL-3.0-or-later
- **Status:** Ikke påbegyndt. Implementeres som [Milestone 4](../keepass-deltasync-spec.md#milestone-4--android).

## Distribution

- Google Play (standard Android-signering)
- F-Droid (kræver reproducible builds, fuldt open source-toolchain)

Undgå Google Play Services hvor muligt — proprietære biblioteker kan blokere F-Droid-distribution.
