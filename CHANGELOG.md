# Changelog

All notable changes to this project are documented here. The format
follows [Keep a Changelog](https://keepachangelog.com/) and the
project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- **The Firefox extension works without a server** — `add-local <name>
  <path.kdbx>` registers a database for local search only. Until now the sole
  way into the client's config was `init`, which requires enrollment, so
  someone who just wanted to find the right tab had to hand-edit
  `config.toml`. The binding gets no `remote_id`, and every command that talks
  to the server refuses it by name rather than sending an empty UUID; `daemon`
  skips such databases instead of failing on them. The keyring is keyed on the
  server UUID, which a local-only database does not have, so `add-local` mints
  a local id for that slot — without it every local-only database would share
  one keyring entry. `--save-password` verifies the masterpassword by opening
  the database before storing it. `databases` no longer requires enrollment
  when there is something local to show. See
  [`docs/install-browser.md`](docs/install-browser.md).

- **`install-browser-host` registers with every Firefox it finds.** On Linux
  `~/.mozilla/native-messaging-hosts` is only correct for a packaged Firefox: a
  snap reads `~/snap/firefox/common/…` and a flatpak reads
  `~/.var/app/org.mozilla.firefox/…`. The command used to write one path and
  print "Installed browser host for Firefox" regardless, leaving snap and
  flatpak users with a success message, a file on disk, and an extension that
  still could not start the host. It now writes one manifest and launcher per
  detected variant, prints which ones, and carries the variant-specific catch:
  the flatpak launcher goes through `flatpak-spawn --host` and prints the
  `flatpak override` that the sandbox requires, the snap note explains why the
  binary must stay out of dot-directories, and macOS prints the `xattr` command
  that clears Gatekeeper's quarantine on a downloaded binary.
  `uninstall-browser-host` cleans up every variant, including ones since
  removed. `--all` installs for variants that are not present yet.

- **A setup button in the extension's popup** (0.2.0) — the two dead ends
  ("cannot start the native host" and "no databases are registered") now say
  what is wrong in a sentence and offer a button to the setup guide, instead of
  printing a raw CLI command. The guide deliberately lives outside the
  extension: a signed add-on cannot be corrected without another AMO review,
  and sandbox paths are exactly the kind of instruction that goes stale.

- **Firefox extension — search & go** (`extension/`) — search your KeePass
  entries from Firefox' address bar (`kp` keyword) or a popup, and open the
  entry's website. Filling in credentials deliberately stays with
  KeePassXC-Browser; this closes the gap it does not cover, namely *finding*
  the right page. The extension talks to a new `keepass-deltasync
  browser-host` subcommand over native messaging and only ever receives
  titles, URLs and group paths — an allow-list enforced in the host, so no
  future bug in the extension can leak a field the host never sent. It
  requests no host permissions at all.

  The masterpassword does not pass through the browser: the host reads it from
  the OS keyring itself, builds the index and wipes the key material again. A
  database without a keyring entry falls back to a prompt in the popup, held in
  the host's memory under an idle lock. Entries in the recycle bin and in
  groups with searching disabled are excluded, and values that cannot be
  navigated to (`{REF:…}` placeholders, `cmd://`, non-http schemes) never reach
  `tabs.update`. `install-browser-host` / `uninstall-browser-host` register the
  native messaging manifest on Linux, macOS and Windows.

  Entries with several URLs (KeePassXC' *Additional URLs*) are searchable on
  every one of them. Such an entry takes one row with a `2 URLs` badge, and
  selecting it unfolds every address beneath, best match first — so no address
  is unreachable, and the result list is not padded with the same entry twice.
  Which one counts as best depends on the search: a match carried by the title
  keeps the primary address on top, while an address that matched harder than
  the title wins. The extension carries the DeltaSync mark, rebuilt as SVG
  from the Android launcher icon. See
  [`docs/browser-extension.md`](docs/browser-extension.md).

- **QR-code enrollment (Android)** — the enrollment screen gains a *Scan QR
  code* button (ZXing, camera) that reads the enrollment QR shown by the web
  admin panel (`deltasync://enroll?server=…&token=…`) and fills the form, so a
  new device is bound by scanning instead of typing. Manual entry is unchanged;
  camera use is optional (`android.hardware.camera` not required). Ships in a
  future `android/*` release. (The QR itself is produced server-side — see
  server/v0.2.0.)
- **Obtainium as an Android distribution channel** — signed release APKs are
  now published to GitHub Releases, so [Obtainium](https://github.com/ImranR98/Obtainium)
  can install the app and track updates while the F-Droid submission is
  pending. `android/publish-release.sh` uploads a locally built + signed APK
  (the release keystore deliberately stays off CI) and refuses to publish an
  unsigned APK, one whose embedded version disagrees with
  `app/build.gradle.kts`, or one without a matching `android/v*` tag.
  `android/README.md` carries the Obtainium deep link and the signing
  certificate's SHA-256. Note that switching between this channel and a future
  F-Droid build requires a reinstall — the signing keys differ.
- **CI builds the Android APK** — a `build:android` job on `android/v*` tags
  builds the gomobile `.aar` and runs `assembleRelease`, publishing
  `DeltaSync-<version>-unsigned.apk` as an artifact. It deliberately stops
  short of signing: the release keystore would otherwise be readable by every
  Maintainer and by any compromised job, and that key is the only thing tying
  future updates to this project. `publish-release.sh` now zipaligns and signs
  an unsigned APK locally before uploading, so the key never leaves the
  maintainer's machine. The job also fails if the tag and
  `build.gradle.kts`'s `versionName` disagree.
- **SECURITY.md and CONTRIBUTING.md** — a vulnerability-reporting policy
  (private channels, scope, trust model) and a contributor guide (DCO sign-off,
  per-component build/test, release tagging). A `/.well-known/security.txt`
  is served from the website.

### Changed

- **Go toolchain requirement relaxed to `go 1.26.0`** — `client/go.mod`
  declared `go 1.26.3`, an exact patch release that was simply whatever
  toolchain happened to be installed when `go mod tidy` last ran. Nothing
  needs that patch level, and it forced the F-Droid recipe to download a
  prebuilt Go tarball from go.dev — which F-Droid rejects. Debian
  trixie-backports ships `golang-go` 1.26, so the distribution's own package
  now suffices. Verified with `GOTOOLCHAIN=go1.26.0`: build, vet and the full
  test suite pass.
- **F-Droid recipe reworked** per review feedback on
  [fdroiddata!41661](https://gitlab.com/fdroid/fdroiddata/-/merge_requests/41661):
  Go is now built from source via fdroiddata's `go` srclib and `make.bash`,
  with Debian's `golang-go` serving only as the bootstrap toolchain, instead
  of downloading a prebuilt tarball from go.dev; the gomobile steps moved from
  `prebuild:` to `build:` so the generated `.aar` is created after the binary
  scanner runs (`scanignore:` dropped entirely); and `AutoName:`/`Description:`
  were removed so the app's name and description are pulled from `fastlane/`
  alone.
- **Per-component release versioning** — release tags are now namespaced
  (`client/vX.Y.Z`, `android/vX.Y.Z`, `server/vX.Y.Z`) so the three
  components' version lines never collide. The bare `v1.0.0` / `v0.1.0`
  tags are legacy and no longer trigger CI. See
  [`VERSIONING.md`](VERSIONING.md).

### Fixed

- **Dark-theme contrast (Android)** — the saturated brand blue (`#1E40AF`)
  was hard to read against the near-black dark-theme background, affecting
  links, switches and buttons. A `values-night` override lightens it to
  `#A8C7FF`.

## [gui/v0.3.2] — 2026-08-20

The first GUI release out of the monorepo, and the first whose Windows
installer is built by CI instead of by hand on one machine. The application
itself is unchanged from 0.3.1 — only where it is built from, and how it
reaches you, is different.

### Added

- **Desktop GUI moved into the monorepo** (`gui/`) — the Fyne GUI that wraps
  the command-line client used to live in its own repository
  (`gitlab.com/Star95/keepass-deltasync-gui`, no longer developed); its history came
  along via `git subtree`. It is the sixth component here and stays its own Go
  module (Go 1.23 + CGO, against the client's Go 1.26 pure-Go build) — it does
  not import the client, it shells out to the binary.

  The move closes a dependency that was real but written down nowhere: the
  combined Windows installer, `gui/installer/build.ps1`, resolved the CLI as
  `..\..\Keepass-deltasync` — so `KeePass-Delta-Sync-Setup-<ver>.exe`, the file
  users actually download, could only be built on a machine that happened to
  have both repositories cloned side by side under exactly those directory
  names. It now reads `client/` from the same checkout, and takes both source
  zips out of one `git archive`. Because the GUI's version comes from
  `gui/FyneApp.toml` and the CLI's from the latest `client/v*` tag, an
  installer is pinned to whatever the two components are standing at in the
  commit it was built from.

  Releases move to `gui/vX.Y.Z` tags, continuing the version line from the old
  repository (last release there: a bare `v0.3.1`). `build:gui` cross-compiles
  the Linux `.tar.xz` and Windows `.exe` with the `fyne` tool, and `test:gui`
  vets the module whenever `gui/` is touched. The old bare `vX.Y.Z` tags were
  deliberately not imported: `v0.1.0` already means something else here.

  The installer is now built by the same tag, so it stops being a thing only
  one machine can produce. `build:installer-stage` assembles what the Inno
  Setup script expects — the packaged GUI, a fresh Windows CLI, a source zip
  per component out of `git archive`, and the icon — and `build:installer`
  compiles it, running Inno Setup under wine because ISCC is a Windows program.
  `release:gui` attaches the installer alongside the bare binaries and names it
  first: it is what Windows users should take. `build.ps1` still builds the
  same installer locally and is unchanged apart from the path fix. See
  [`VERSIONING.md`](VERSIONING.md) and [`gui/README.md`](gui/README.md).

## [android/v0.4.1] — 2026-07-29

No functional changes — the app is identical to 0.4.0. Only the release
build changed, so that F-Droid can publish the APK signed with our own key
instead of theirs.

### Changed

- **The release APK is byte-for-byte reproducible** against an `fdroid build`
  of the same commit (verified: both sides produce
  `sha256 a127e6d1…` before signing). Four sources of nondeterminism were
  removed: the Go patch release is pinned to the one the F-Droid recipe's
  `go` srclib builds, `gomobile bind` runs with `-trimpath` (which also drops
  its random `/tmp/gomobile-work-NNN` work dir), the build runs from
  `/home/vagrant/build/<applicationId>` because gomobile's generated `gobind`
  module records our module's absolute path in the binary's build info, and
  the Android SDK sits at `/opt/android-sdk` because cgo hands the NDK path
  to clang, which writes it into `libgojni.so`'s debug info. A symlink is not
  enough there — clang resolves its own binary to find its resource dir.
- **`android/publish-release.sh` signs with `--alignment-preserved`.** F-Droid
  verifies a reproducible build by transplanting our signature onto their own
  build with `apksigcopier`; without the flag `apksigner` re-pads stored ZIP
  entries while signing, `apksigcopier` cannot reproduce that, and the
  verification fails. The signed APK still passes `zipalign -c -P 16 -v 4`.

## [server/v0.2.0] — 2026-07-01

### Added

- **Web admin panel** — `admin.html`, served by the server, is a
  token-authenticated UI for managing the server from a browser: create users,
  issue enrollment tokens, enable/disable and delete users, and browse the
  audit log (with manual cleanup). It calls the existing admin API with the
  admin token (kept only in the tab's session storage); no new server
  endpoints, and it works under an `APP_BASE_PATH` sub-path.
- **QR codes for enrollment tokens** — each issued enrollment token is rendered
  as a QR (server URL + token bundled) so Android devices can enroll by
  scanning. The QR is produced entirely client-side by a self-contained
  byte-mode encoder embedded in `admin.html` — no new dependency and no server
  code.
- **Health-check endpoint** — `GET /api/v1/health` (public, no auth) returns
  `200 {"status":"ok","db":"up"}` when the app and database are reachable, or
  `503` otherwise. The Docker image ships a built-in `HEALTHCHECK` that uses it,
  so `docker ps` and NAS UIs (TrueNAS, Portainer) show real app health.

### Fixed

- **Server returned newline-wrapped base64** — PostgreSQL's
  `encode(…, 'base64')` breaks output at 76 chars per RFC 2045. Strict
  decoders (Android's `java.util.Base64`, Go's `base64.StdEncoding`)
  rejected the embedded `\n` with "Illegal base64 character a". The
  entry changes/versions endpoints now strip the newlines.

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
