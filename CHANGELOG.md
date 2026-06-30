# Changelog

All notable changes to this project are documented here. The format
follows [Keep a Changelog](https://keepachangelog.com/) and the
project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- **Web admin panel** — `admin.html`, served by the server, is a
  token-authenticated UI for managing the server from a browser: create users,
  issue enrollment tokens, enable/disable and delete users, and browse the
  audit log (with manual cleanup). It calls the existing admin API with the
  admin token (kept only in the tab's session storage); no new server
  endpoints, and it works under an `APP_BASE_PATH` sub-path.
- **Health-check endpoint** — `GET /api/v1/health` (public, no auth) returns
  `200 {"status":"ok","db":"up"}` when the app and database are reachable, or
  `503` otherwise. The Docker image ships a built-in `HEALTHCHECK` that uses it,
  so `docker ps` and NAS UIs (TrueNAS, Portainer) show real app health.
- **SECURITY.md and CONTRIBUTING.md** — a vulnerability-reporting policy
  (private channels, scope, trust model) and a contributor guide (DCO sign-off,
  per-component build/test, release tagging). A `/.well-known/security.txt`
  is served from the website.

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

## [server/v0.1.0] — 2026-06-30

### Added

- **Docker image + self-hosting** — the server is now published as a
  multi-arch (amd64 + arm64) Docker image to the GitLab Container
  Registry, built by CI on `server/vX.Y.Z` tags. A self-contained
  `compose.yml` (PostgreSQL + app) makes running your own server a
  copy-paste affair — including pasting straight into a NAS UI such as
  TrueNAS SCALE's Custom App. The container entrypoint waits for the DB,
  applies schema migrations idempotently, and mints a first admin token.
  See [`docs/self-hosting-docker.md`](docs/self-hosting-docker.md).

## [android/v0.3.1] — 2026-06-29

### Fixed

- **Duplicate entries after sync** — entries living in a search-disabled
  group (notably KeePassDX's template group, `Meta/EntryTemplatesGroup`,
  which has `EnableSearching=false`) were never recognised as already
  present, because `applyToDatabase` used kotpass' `findEntries` — which
  skips both the recycle bin and search-disabled groups. Every pull
  therefore re-added a copy with the same UUID, accumulating duplicate
  identifiers that KeePassDX flags on open (KeePassXC silently
  de-duplicates on load). Existing entries are now matched via a full
  group-tree walk, so they are updated in place instead of duplicated.
- **Local database not rewritten when a sync pulled only groups** —
  `Synchronizer` now also writes the `.kdbx` back when group changes
  (and not just entry/deletion changes) were pulled.

## [client/v1.5.0] — 2026-06-24

### Added

- **`devices remove <id>`** — revoke an enrolled device from the desktop
  client. Calls `DELETE /api/v1/devices/{id}`, so the device's token is
  invalidated server-side; any of your own devices can be revoked,
  including the current one. This is the command the GUI's *Remove
  device* button invokes — previously the button failed because the
  subcommand did not exist (`devices` rejected all arguments).

## [client/v1.2.0] — 2026-06-04

### Added

- **Desktop `tui` is now bilingual** — the interactive menu defaults to
  **English** and gains a *Language / Sprog* item that switches it to
  Danish; the choice is remembered in `config.toml` (`language`). Every
  menu label, prompt and status message routes through a string table
  (`tui_i18n.go`) rather than being hard-coded.

## [android/v0.2.0] — 2026-06-04

First Android release under the per-component tag scheme (the bare
`0.1.0` tag was the unified product version). Collects everything the
Android app gained since 0.1.0.

### Added

- **Auto-sync in the background, controlled by a visible switch** — the
  main screen now has an *Auto-sync in the background* switch, replacing
  the easy-to-miss "remember password" checkbox in the sync dialog.
  Turning it on confirms your identity once with `BiometricPrompt`
  (biometric or device PIN/pattern) and, after the next successful sync,
  stores the kdbx master password in a new `EncryptedPassphraseStore`
  (Keystore-backed `EncryptedSharedPreferences`, bound to the database
  UUID) and enables the periodic `SyncWorker`. After that the app syncs
  with **no further prompts** — no password, no fingerprint. The Keystore
  key intentionally requires no per-use authentication so the background
  worker can decrypt it with no user present; biometrics gate the
  *opt-in*, not each read. The sync interval is **selectable between
  15 / 30 / 60 minutes** (WorkManager's hard minimum is 15). Turning the
  switch off — or "Forget device credentials" — clears the stored
  password and cancels background sync.
- **Idle ticks no longer decode the whole database** — sync was already
  delta over the wire, but every tick still decoded the local `.kdbx`
  (two Argon2 passes: the kotpass decode and the gomobile session) even
  when nothing had changed. A new `SyncProbeStore` records a cheap file
  fingerprint (last-modified + size via a `ContentResolver` metadata
  query — no file read), and `SyncProbe.nothingToSync` returns early
  when the local file is unchanged **and** the server's `current_seq`
  equals the stored `lastSeq`, skipping the decode entirely. The same
  probe runs once when you open the app, syncing only if something
  actually changed. No server changes — it reuses `current_seq` from the
  existing `/changes` endpoint. Missing file metadata never causes a
  wrong skip, and the fingerprint is stored *after* the sync so the
  worker's own rewrite doesn't trigger the next tick.
- **Live sync progress** — progress bar + label showing
  Opening / Pulling x/total / Pushing x/total / Saving during a sync.
- **Share a database from the app** — a new Share screen
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

### Fixed

- **A transient sync error no longer forgets your saved password** —
  previously any non-network error cleared the remembered password and
  disabled background sync, so a brief server hiccup forced you to
  re-enter it "every time". Now only a genuinely wrong password (kotpass
  `CryptoError.InvalidKey`) clears it and turns auto-sync off; every
  other error keeps the password and retries on the next tick.
- **Master password no longer stored in plaintext** — the
  previous `SyncWorker` received the password through WorkManager's
  `Data`, which persists unencrypted in WorkManager's database. The
  worker now reads it from the Keystore-encrypted `EncryptedPassphraseStore`
  instead, and the in-memory-only `SessionPassphrase` is gone.
- **Local deletions now propagate** — `read()` previously
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

### Changed

- **`minSdk` raised 21 → 23** (Android 6.0) — required by
  `androidx.biometric` and gives stronger Keystore guarantees. Covers
  essentially all active devices.

## [client/v1.1.0] — 2026-06-01

### Added

- **Desktop client `tui` subcommand** — an interactive full-screen menu
  (tview) that runs the common commands without having to remember
  command names, flags or database names. A thin command-selector: it
  reads `config.toml` for state + the database list and shells out to
  the same binary so password prompts work unchanged. Includes a
  switch-account wizard, a `forget` command, and enrollment-token
  generation for a new device.

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
