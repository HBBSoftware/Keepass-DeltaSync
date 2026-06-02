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
- **Android: live sync progress** — progress bar + label showing
  Opening / Pulling x/total / Pushing x/total / Saving during a sync.
- **Android: share a database from the app** — a new Share screen
  (owner only) lists current members, adds a member by username, and
  removes one. Mirrors the desktop `share`/`unshare`/`shares` commands:
  it looks the user up (`/users/lookup`), wraps the database master key
  to their device public key with a sealed box, and POSTs it as
  `wrapped_master_key`. A new `mobile.WrapMasterKeyForShare` Go binding
  (the owner-side counterpart to `UnwrapSharedMasterKey`) derives the
  master key via Argon2id and seals it; `ApiClient` gained
  `lookupUser` / `listShares` / `shareDatabase` / `unshareDatabase`. The
  master password comes from the Keystore store if remembered, otherwise
  it is prompted (and not persisted). Non-owners get a clear "only the
  owner can manage sharing" message (server returns 403).
- **Android: opt-in background sync with a securely stored password** —
  ticking "Remember password & sync in the background" now confirms your
  identity with `BiometricPrompt` (biometric or device PIN/pattern) and,
  after the next successful sync, stores the kdbx master password in a
  new `EncryptedPassphraseStore` (Keystore-backed
  `EncryptedSharedPreferences`, bound to the database UUID) and enables
  the periodic `SyncWorker`. The Keystore key intentionally requires no
  per-use authentication so the background worker can decrypt it without
  the user present — biometrics gate the *opt-in*, not each read. A
  failed sync (wrong password, revoked access) clears the stored
  password and cancels background sync so it can't loop. "Forget device
  credentials" also clears it.

### Fixed

- **Android: master password no longer stored in plaintext** — the
  previous `SyncWorker` received the password through WorkManager's
  `Data`, which persists unencrypted in WorkManager's database. The
  worker now reads it from the Keystore-encrypted `EncryptedPassphraseStore`
  instead, and the in-memory-only `SessionPassphrase` is gone.
- **Android: local deletions now propagate** — `read()` previously
  ignored both KDBX' `DeletedObjects` list and entries the user had
  moved to the recycle bin, so deleting an entry on Android never
  removed it on the server or on other devices. The
  `KotpassLocalStateAdapter` now mirrors the desktop client's
  `ParseExport`: it walks the group tree itself (kotpass' `findEntries`
  silently drops recycle-bin entries), synthesizes a tombstone for each
  direct child of the recycle-bin group (`DeletedAt = LocationChanged`),
  and adds a tombstone for every `DeletedObject`. A live entry still
  wins over a stale tombstone of the same UUID. Trade-off, same as
  desktop: moving an entry back out of the recycle bin does not
  resurrect it on other devices.
- **Server returned newline-wrapped base64** — PostgreSQL's
  `encode(…, 'base64')` breaks output at 76 chars per RFC 2045. Strict
  decoders (Android's `java.util.Base64`, Go's `base64.StdEncoding`)
  rejected the embedded `\n` with "Illegal base64 character a". The
  entry changes/versions endpoints now strip the newlines.

### Changed

- **Android: `minSdk` raised 21 → 23** (Android 6.0) — required by
  `androidx.biometric` and gives stronger Keystore guarantees. Covers
  essentially all active devices.
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
