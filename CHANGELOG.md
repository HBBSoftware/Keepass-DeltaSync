# Changelog

All notable changes to this project are documented here. The format
follows [Keep a Changelog](https://keepachangelog.com/) and the
project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- **Desktop client `tui` subcommand** — an interactive full-screen menu
  (tview) that runs the common commands without having to remember
  command names, flags or database names. It is a thin command-selector:
  reads `config.toml` for state + the database list and shells out to
  the same binary so password prompts work unchanged.
- **Android: "remember password while the app is running"** — optional
  checkbox in the sync dialog; the passphrase is held in memory for the
  process lifetime only (never persisted), and cleared on sync failure
  or when device credentials are forgotten.
- **Android: live sync progress** — progress bar + label showing
  Opening / Pulling x/total / Pushing x/total / Saving during a sync.

### Fixed

- **Server returned newline-wrapped base64** — PostgreSQL's
  `encode(…, 'base64')` breaks output at 76 chars per RFC 2045. Strict
  decoders (Android's `java.util.Base64`, Go's `base64.StdEncoding`)
  rejected the embedded `\n` with "Illegal base64 character a". The
  entry changes/versions endpoints now strip the newlines.

### Changed

- **Per-component release versioning** — release tags are now namespaced
  (`client/vX.Y.Z`, `android/vX.Y.Z`, `server/vX.Y.Z`) so the three
  components' version lines never collide. The bare `v1.0.0` / `v0.1.0`
  tags are legacy and no longer trigger CI. See
  [`VERSIONING.md`](VERSIONING.md).

## [0.1.0] — 2026-05-29

First tagged release. Brings the server, desktop client, and a working
Android client under one roof.

### Added

#### Cross-cutting

- **v3 canonical entry wire-format** — platform-independent JSON
  schema that replaces raw KeePassXC `InnerXML` fragments as the
  encrypted payload on the wire. Designed so Go (desktop) and Kotlin
  (Android) emit byte-compatible blobs.
  ([`docs/v3-canonical-entry-format.md`](docs/v3-canonical-entry-format.md))
- **Format-version envelope byte** — `0x01` prefix for canonical blobs,
  `<` (0x3C) for legacy XML. Dual-read of both during migration.
- **Architecture-diagram SVGs** under [`docs/diagrams/`](docs/diagrams/),
  embedded in the project README.

#### Desktop client (Go)

- `canonical/` package modelling KDBX4 entries with lossless
  `FromInnerXML` / `ToInnerXML` round-trip, JSON round-trip, and an
  envelope (`EncodeCanonical` / `DecodeCanonical` / `DetectFormat`).
- Desktop `push` / `pull` / `init-shared` now emit canonical blobs and
  dual-read legacy.
- Integration tests (`-tags=cli`) drive a real `keepassxc-cli` to catch
  format drift; caught two real bugs (`Protected="True"` SIGSEGV and
  tag-separator mismatch).
- `mobile/` package — gomobile-bind-friendly facade exposing `Session`,
  `EncryptEntry` / `DecryptEntry`, sharing helpers, and `SchemaVersion`.

#### Android client (Kotlin)

- `:sync` Gradle JVM module — pure Kotlin core that mirrors `canonical.Entry`
  via `kotlinx.serialization`, bridges to kotpass entries with
  `Mapper.toCanonical` / `toKotpass`, runs entry-level last-writer-wins
  via `SyncEngine`, and packages everything in `Synchronizer.sync()`.
- HTTP layer via OkHttp — `ApiClient` (sync endpoints) and
  `EnrollmentClient` (first-run device bootstrap).
- `KdbxFile` abstraction with `PathKdbxFile` (filesystem, used by tests)
  and `SafKdbxFile` (Android SAF via ContentResolver).
- 40 green tests covering canonical round-trip, kotpass mapping,
  HTTP via MockWebServer, end-to-end Synchronizer flow.
- `:app` Gradle Android module that builds an installable debug APK
  (~18.8 MB including JNI libs for all four ABIs):
  - `MainActivity` — status + sync trigger.
  - `EnrollActivity` — server URL + enrollment token form.
  - `SetupActivity` — SAF kdbx picker + server-side database matcher.
  - `KeystoreTokenStore` — credentials in EncryptedSharedPreferences.
  - `DataStoreSyncStatePersistence` — sync state in DataStore.
  - `SyncWorker` — periodic background sync via WorkManager.

#### F-Droid

- `metadata/dk.bjoerckbraun.deltasync.yml` build manifest with
  pre-build steps for installing gomobile + NDK r25c and generating
  the `.aar`.
- `fastlane/metadata/android/en-US/` with human-facing copy.
- [`android/F-DROID.md`](android/F-DROID.md) documents the submission
  flow and the dependency licenses.

### Known limitations in 0.1.0

- No app icon — uses the Android default.
- The Sync Now flow prompts for the kdbx master password each run.
  Caching it (with biometric unlock) is on the v0.2 roadmap.
- No "share database" UI on Android — owners must do v2 sharing from
  the desktop client.
- The KotpassLocalStateAdapter does not yet populate tombstones from
  KDBX's `DeletedObjects` list — only deletes made while the
  SyncEngine is running propagate to the server.
