# keepass-deltasync-android

Android-klient. Sync-service-only — ingen indbygget entry-editor. KeePassXC's
egne kdbx-filer redigeres af brugerens foretrukne Android-KeePass-app
([Keepass2Android](https://github.com/PhilippC/keepass2android) eller
[KeePassDX](https://github.com/Kunzisoft/KeePassDX)), og DeltaSync-app'en
holder `.kdbx`-filen synkroniseret i baggrunden via WorkManager.

- **Licens:** GPL-3.0-or-later
- **Distribution:** F-Droid først (reproducible builds, ingen Google Play
  Services), Play-version kan tilføjes senere uden refactor.
- **Status:** M4 i gang. Go-siden via `mobile`-pakken er på plads
  (`gomobile bind`-vendt API for crypto + canonical wire-format). Kotlin
  app-skelet er ikke skrevet endnu.

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

## Sådan bygges Go-siden

`gomobile bind` producerer en .aar + Kotlin-stubs:

```sh
# Engangs-setup: installer gomobile + Android NDK
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init  # downloader SDK/NDK eller bruger eksisterende ANDROID_HOME

# Byg .aar fra mobile/-pakken
cd client
gomobile bind -target=android -o ../android/libs/deltasync.aar \
    gitlab.com/Star95/keepass-deltasync/client/mobile
```

Resultatet er `deltasync.aar` der lægges i Android-projektets `libs/`-mappe
og dependes via Gradle.

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
