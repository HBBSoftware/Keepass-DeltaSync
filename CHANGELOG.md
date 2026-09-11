# Changelog

All notable changes to this project are documented here. The format
follows [Keep a Changelog](https://keepachangelog.com/) and the
project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- **Chrome and Edge extension** (`extension-chromium/`) — the same search &
  go extension, ported to the two Chromium browsers. The popup, the search and
  the background logic are not copied: `extension-chromium/package.sh` reads
  them out of `extension/` at build time, so the two browsers cannot drift
  apart, and it fails the build if one of the few strings that say "Firefox"
  to the user has been reworded. What this directory holds is what Chromium
  needs differently — a service worker entry point, raster icons redrawn from
  the same SVG, and `compat.js`, which builds the `browser` namespace the
  shared code calls and bridges the one difference that is not cosmetic:
  Firefox lets a message listener answer by returning a promise, Chromium
  wants `sendResponse`. Without that bridge every popup would open empty.

  `install-browser-host` now registers with Chrome, Chromium and Edge
  alongside Firefox, writing one manifest per browser — the two families
  disagree about whether the caller is named in `allowed_extensions` or in
  `allowed_origins`, and the wrong form means the host never starts, silently.
  Chromium derives an extension's ID from the store's signing key, so the ID
  does not exist until the extension is uploaded and differs between the two
  stores; until then it comes in with `--extension-id`, and those browsers are
  skipped rather than registered with an empty allow-list. See
  [`extension-chromium/README.md`](extension-chromium/README.md).

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

- **The admin panel explains what a user is.** The Users tab said only what the
  button did — "creates the user and issues a one-time enrollment token" — and
  never what a user *is*, so the model had to be inferred from behaviour. It now
  says it: a user is a person, their phone and computer each enroll as a device
  under that user, and all of a user's devices see the same databases with
  nothing to share between them. Sharing is for two people, not two devices.

  The Devices tab says the same thing from the other side. Between them the two
  sentences answer the question that actually comes up — why a freshly enrolled
  phone sees no databases — which is usually that the device belongs to a
  different user than the one owning them.

- **The Firefox extension connects by itself** (`extension/`) — on a machine
  where everything is set up, opening the popup (toolbar button or
  Alt+Shift+K) used to land on an *Unlock* button whose only job was to ask
  the host for a masterpassword it fetches from the OS keyring itself. The
  popup now does that on its own and shows the search field as soon as the
  index is built, and the address bar's `kp` keyword warms the same index when
  it is asked to search with nothing unlocked. The button stays for the cases
  where a click actually decides something: after you pressed **Lock** —
  auto-connect then stays off for the rest of the browser session, since
  otherwise the button would be pointless — and after an attempt failed. A
  database with no keyring entry opens the password field straight away rather
  than costing a wasted click first, and a failed attempt is not repeated on
  every popup open: each one costs an Argon2 run and would answer the same.
  Nothing in the security model moves; the extension still never sees the
  masterpassword.
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

- **Deleting a folder left its entries alive on every other device.** Deleting
  a group in KeePass moves the group *with its contents* into the recycle bin,
  so the entries end up in a subgroup of the bin rather than directly in it.
  Both clients only treated the bin's direct children as deleted, so those
  entries were still collected as live objects — and pushed, with a parent
  group that the same sync was tombstoning. On every other device the group
  disappeared while its entries resurfaced in the root, and the passwords you
  meant to delete stayed in the database. The recycle-bin marker now carries
  down the whole subtree on both desktop (`ParseExport`) and Android
  (`KotpassLocalStateAdapter.read`), so deleting a folder deletes what is in
  it, subfolders included.

  Unchanged, and worth knowing: undelete still does not propagate. Dragging an
  entry — or now a folder — back out of the recycle bin does not resurrect it
  elsewhere, because it is already tombstoned on the server.

- **A large folder deletion was refused as if it were corruption.** The guard
  against mass group deletion counted every group missing from the export,
  including the ones plainly sitting in the recycle bin, so deleting one folder
  with five or more subfolders could trip it and silently sync nothing. It now
  counts only groups that vanished *without a trace*, which is the case it was
  built for (a failed merge, a restored backup, the wrong file). A deletion the
  export can account for is always carried out, however large. This mirrors how
  entries have always worked: an entry is tombstoned because we can see it in
  the recycle bin, never because it is missing.

- **Lock in the Firefox popup did not survive the next sync.** The host keeps
  watching the `.kdbx` after a `lock`, and the extension re-indexed on every
  change event it got — including for a database it no longer held an index
  for. A sync landing after you pressed Lock therefore unlocked the database
  again, silently, and rebuilt the index from the OS keyring. A change event
  now only refreshes an index that already exists.

- **Dark-theme contrast (Android)** — the saturated brand blue (`#1E40AF`)
  was hard to read against the near-black dark-theme background, affecting
  links, switches and buttons. A `values-night` override lightens it to
  `#A8C7FF`.

## [android/v0.4.2] — 2026-09-09

### Added

- **The Android app can create a database (Android)** — a phone could not be
  someone's first device. The setup screen listed what the server had and
  offered no way to add to it, so a new user starting on Android reached "no
  databases on the server, create one with the desktop client first" and
  stopped. Creating one lived only in the desktop client's `init`.

  Nothing about it needed a privileged client: `POST /databases` takes a name,
  ownership comes from the device token, and the master key is derived locally
  with Argon2id and never leaves the device. The app had every credential and
  lacked only a button.

  The button follows the situation, because an option that is always there is
  one that gets skipped. Before the list is fetched, nothing shows. If the list
  is empty it appears filled and explained, as the only way forward. If there
  are databases it drops to a quiet text button, so it does not compete with
  the choice being made. The red "no databases on the server" is gone from the
  empty case — an empty list is a starting point, not a failure, and that
  wording was misleading anyway: it said *the server* when it meant *your
  account*.

### Fixed

- **A self-hosted server could not be reached over plain HTTP (Android)** — the
  manifest set `usesCleartextTraffic="false"`, which rules out exactly the setup
  the app is built for: a NAS on the LAN with no certificate. The project's own
  compose files serve plain HTTP, so a self-hoster met it on the first attempt,
  and what they saw was the platform's raw *CLEARTEXT communication … not
  permitted by network security policy* — which reads like a broken server.

  Limiting cleartext to private ranges turns out not to be expressible:
  `<domain>` takes a hostname or one IP literal, never a CIDR block. The choice
  is between allowing it and refusing to work on a LAN. So the enrollment screen
  carries the weight instead, marking an `http://` address as unencrypted while
  it is typed. What the transport carries is ciphertext and a bearer token
  either way.

- **An optional device name made enrollment fail (Android)** — leaving the field
  empty meant the server answered `"name": null`, and the response model
  declared it non-nullable, so parsing a valid response threw. The failure
  landed after the server had created the device and consumed the one-time
  token, leaving the user with a spent token and an error about JSON offsets.
  There is now a test for the null case; the old one always sent a name.

- **The keyboard covered the field being typed into (Android)** — from
  targetSdk 35, Android 15 draws edge-to-edge and no longer honours
  `windowSoftInputMode="adjustResize"` as before; an app must read the IME inset
  itself. This one read no insets at all. All four screens now pad by the system
  bars, or by the keyboard when it is up.

## [server/v0.5.0] — 2026-09-09

### Added

- **`bin/admin database:create <username> <name>`** — an administrator can
  create a database on a user's behalf. `POST /databases` derives the owner
  from the auth context and so needs a *device* token, which left a gap with
  no way across it: an admin could create users and issue enrollment tokens,
  but not the database those users are supposed to sync. A freshly enrolled
  Android device lands on a screen that says "no databases on the server —
  create one with the desktop client first", which is not an answer if the
  desktop client is not what you have.

  Nothing about it needs a client. A database is a name, an owner row and a
  sequence counter; the master key is derived locally from the passphrase with
  Argon2id and never reaches the server, which is why `wrapped_master_key` is
  NULL for owners. So an admin can create the row without gaining access to
  anything stored in it.

- **`compose.truenas.yml`** — a TrueNAS-shaped variant of `compose.yml`, and the
  file the self-hosting guide now points at for that platform. Every difference
  in it was learned by getting it wrong on a real box: a named volume, because
  a bind mount to a dataset that does not exist yet fails with
  `bind source path does not exist` and Docker will not create it; the Postgres
  18 data directory mounted at `/var/lib/postgresql` rather than
  `.../data`, which is correct for Postgres 16 and makes 18 nest a volume inside
  a volume so the container never reports healthy; a published port outside the
  range TrueNAS uses; and the admin login pre-wired so there is no token to read
  out of a container log.

  Its header carries the password rules, which is the one that costs the most
  time to work out alone: Docker Compose expands an unescaped `$` after YAML is
  parsed, so `"Kode$xyz"` reaches the server as `Kode` — a server that works
  perfectly and rejects the password you are certain you set.

## [server/v0.4.1] — 2026-09-08

### Added

- **A Next steps tab in the admin panel.** Creating a user and issuing a token
  is the easy half; the admin was then left to work out what to send the person
  and what they should do with it. The tab lays out the four steps, shows this
  server's address as the page itself reached it — so it stays right behind a
  proxy or under `APP_BASE_PATH` instead of being something to work out — and
  names the exact release assets per platform: one installer on Windows, and on
  Linux the client and the GUI as two downloads, because the GUI drives the
  client. It closes with a link to the website.

  Only the graphical route is described. The CLI is what the GUI runs
  underneath, but presenting it as the way in excludes everyone who does not
  want a terminal. The previous *Get clients* link in the header is gone; it
  read as a stray button next to Log out rather than as part of the panel's
  four tabs.

  When the panel itself was reached over plain HTTP, the tab says so: the
  Android app sets `usesCleartextTraffic="false"` and cannot enroll against an
  `http://` server, which surfaces as a raw platform message that reads like
  the server is broken. The note appears only when it applies — behind an HTTPS
  proxy it would be noise, and a warning that fires when it should not is soon
  ignored.

  Android points at the GitHub mirror's releases, where the signed APK actually
  is. The APK is built unsigned in CI and signed outside it, so it is the one
  artefact that does not land on GitLab with the rest — and the website only
  mentions the pending F-Droid submission, so sending people there would have
  been a dead end.

  macOS is listed too, pointing at the website and saying plainly that there is
  no prebuilt macOS app — `release:gui` cross-compiles Linux and Windows only.
  Sending Mac users to the CLI would contradict the point of the tab, and
  leaving them off the table entirely reads as an oversight.

## [server/v0.4.0] — 2026-09-08

### Added

- **The admin panel lists devices** — `GET /api/v1/admin/devices`, and a
  Devices tab showing every enrolled device, its owner, when it was enrolled
  and when it last contacted the server, least recently seen first. The data
  was already there: `TokenAuthenticator` writes `devices.last_seen` on every
  successful device auth. It just was not reachable from an admin session —
  `/api/v1/devices` requires a *device* token and only ever shows the caller's
  own user, so an administrator could see that Alice had three devices but not
  which three, or whether any of them had been in touch since spring.

  Read-only, deliberately. An admin can already read the audit log with its
  device ids, create users, issue enrollment tokens and delete users, so a
  listing grants nothing new — it makes visible data legible. Revoking someone
  else's device is a genuinely new power and belongs to its own decision.
  `devices.token_hash` never leaves the query; columns are listed explicitly.

  A device enrolled before v2 has no X25519 key and cannot receive a shared
  database until it updates, so the table shows that per row.

- **The panel says where to get a client.** An enrollment token is half of what
  a new user needs; the other half is the software to paste it into. The token
  box now links to the desktop releases and the site covering Android and
  Firefox, and there is a *Get clients* link in the header for when no token is
  being issued.

- **A misconfigured server explains itself instead of refusing connections** —
  the entrypoint used to `exit 1` on a bad setting, which under
  `restart: unless-stopped` is a crash loop: the port never opens, the browser
  says "unable to connect", and the reason exists only for whoever thinks to
  run `docker logs`. On a NAS, where the log is a web page you have to know to
  open, that is most of the debugging cost. It now records the reason and
  starts the web server anyway; every route answers 503 with that text, so the
  container still reports **unhealthy** — it genuinely is — but says why. The
  admin panel shows it as a banner and disables the sign-in button rather than
  sitting there looking ordinary. Fixing the setting and restarting clears it.

  The wait for PostgreSQL is now bounded at 120s instead of looping forever, so
  a database that never arrives is reported rather than waited on silently.

- **`ADMIN_TOKEN` and `ADMIN_PASSWORD` log their length at startup.** Not their
  value. A credential that arrives shorter than it was typed is the signature
  of something eating it in transit — Docker Compose expands an unescaped `$`
  in a YAML value, and YAML drops everything after an unquoted ` #`. Both
  produce a working server that rejects the password you are certain you set,
  and the length is the one detail that names the problem instantly. The error
  for a too-short value says so outright.

- **Troubleshooting keyed on the exact error text.** The two failures that cost
  the most time — a bind path that does not exist, and `db` never reporting
  healthy because the Postgres 18 volume was mounted one directory too deep —
  happen before any of our code runs, so no amount of reporting inside the
  container reaches them. What can be done is to meet people where they land:
  each section is titled with the literal string Docker prints, so it can be
  searched for. Includes the detail that TrueNAS' log viewer truncates the
  lines, which hides the part that names the cause.

### Fixed

- **An unreachable database returned an opaque 500 on every route**, including
  `/api/v1/health`, whose whole purpose is to report exactly that. The
  connection is opened before routing, so the failure was caught by the generic
  handler and answered `an unexpected error occurred`. It now answers 503
  `database_unavailable` with a message naming the settings to check.

## [server/v0.3.0] — 2026-09-08

### Added

- **The admin panel has a real login** — username and password instead of
  pasting a bearer token at every visit. The token had to live in the tab's
  session storage while you worked, which put the credential that administers
  every user within reach of any script on the page; the session now rides in
  an `HttpOnly` cookie that JavaScript cannot read. Set it with
  `ADMIN_USERNAME` + `ADMIN_PASSWORD` (or `ADMIN_PASSWORD_FILE`, or
  `bin/admin admin:set-password`). Passwords are Argon2id-hashed, which is what
  the `ARGON2_*` settings were always for — a password is guessable in a way
  32 random bytes are not, so the reasoning in `TokenHasher` does not apply
  here. For the same reason login is rate-limited per IP with
  `RATE_LIMIT_AUTH_PER_MINUTE`, emitting the `auth.rate_limited` audit event
  that the enum already defined. Sessions last `ADMIN_SESSION_TTL_HOURS`
  (8 by default) and slide forward with use; changing the password revokes
  every open one.

  **Bearer tokens are unaffected.** `bin/admin` and the client's `admin`
  commands authenticate exactly as before — admin routes now accept either.

  `TRUSTED_PROXIES` decides when the session cookie gets the `Secure` flag:
  behind a TLS-terminating proxy the only evidence of HTTPS is
  `X-Forwarded-Proto`, and any client can send that, so it counts only from
  listed addresses. It also resolves the standing TODO in `Request::clientIp()`
  — `X-Forwarded-For` is now honoured on the same terms, so the audit log and
  the rate limiter can see real client IPs.

- **The admin token can be set up front** — `ADMIN_TOKEN` (or
  `ADMIN_TOKEN_FILE`, for a Docker secret) on the server container registers
  that token at startup instead of minting a random one and printing it to the
  log. Reading a credential out of a container log is awkward on a NAS, where
  the log view is a web page, and it is unrecoverable once the log rotates.
  `TokenHasher::hash()` is plain SHA-256, so the entrypoint computes the same
  hash the application would and inserts it with `ON CONFLICT DO NOTHING` —
  restarts are a no-op, and setting the variable on an existing deployment
  adds a token rather than replacing one. Tokens shorter than 24 characters
  are refused at startup: this credential administers every user, and the
  generated ones are 43 characters. Unset, the previous behaviour is
  unchanged.

### Changed

- **The server image is published to GHCR as well** —
  `ghcr.io/hbbsoftware/deltasync-server`, alongside the GitLab registry and
  Docker Hub. TrueNAS' app catalog prefers ghcr.io, and the reason is
  practical: a NAS pulls anonymously and so shares Docker Hub's rate limit
  with everything else on the box. All three registries receive the same
  multi-arch manifest from one build. (They would also carry the same cosign
  signature, but `COSIGN_PRIVATE_KEY` has never been configured, so that step
  has always been skipped — see VERSIONING.md.)

## [client/v1.8.1] — 2026-08-26

A group reorganisation on a live database was rolled back by the client's own
sync: nineteen entries jumped out of their groups and into the root. Nothing
was lost — every title and every group survived — but the placement did not,
and the same pull would have done it again on the next device to sync.

Four separate faults were behind it. Three are in the sync path and one is in
the safety of deletion. All four are covered by tests, and the fixed client has
been verified against a live server: a delta pull of 451 entries carrying only
36 groups landed without moving a single entry to the root, which is precisely
the shape of the pull that caused the damage.

### Fixed

- **A pull could move entries into the root.** The staging builder rewrites a
  reference to a group it does not know into Root, and the pull only handed it
  the groups that had changed since `last_seq`. Any entry whose parent group
  happened not to change in the same window was therefore reparented, and the
  merge carried that into the local database. The pull now supplies the whole
  local group tree, so a path exists for every entry it places. This costs no
  extra `keepassxc-cli` round: the export it needs was already being taken a
  few lines earlier to find the root UUID.

- **A moved entry re-pushed for ever.** Moving an entry between groups bumps
  `LocationChanged`, not `LastModificationTime`, so the push filter correctly
  compares the later of the two — but it sends the plain modification time to
  the server, and the pull recorded whatever came back. The entry's tracked
  state therefore rolled backwards on every pull, the filter said yes again,
  and the cycle never closed. Recorded state can now only move forward. The
  wire format is unchanged, so other devices still see honest timestamps.

- **Moving a group was never detected.** The group loop compared modification
  time alone, even though the type has carried `LocationChanged` since group
  sync was built. Rearranging your group tree simply did not propagate.

- **A failed merge could delete your groups on the server.** The push
  tombstoned every known group missing from the export, unconditionally. When a
  merge failed and restored an old backup, the export was a fraction of the
  real database — and 27 groups and 22 entries were deleted server-side, which
  then propagated to every other device. A mass deletion is now declined: more
  than five groups *and* more than a quarter of the known set means the local
  database is not what it should be, not that the user removed them all at
  once. The known set is kept intact so the next run can still tell.

- **Groups taken out of search results kept that setting through a pull.** The
  first fix puts every local group into the staging tree, and the staging
  header hardcoded `EnableSearching` to inherit — which would have silently
  reset every group the user had hidden from search. The flag now travels with
  the group, including for groups the server changed, since the wire format has
  no field for it.

### Added

- **Read-only diagnostics** (`servercheck_test.go`) — four reports that answer
  what the server actually believes: where it thinks each entry lives, which
  groups the next sync would delete, which local objects never reached it, and
  what a push would send right now. They decrypt locally and write nothing.
  Skipped unless `DELTASYNC_CHECK=1`, so they stay out of CI and out of the
  shipped binary.

## [gui/v0.3.5] — 2026-08-26

### Added

- **Update check.** The GUI had no way to tell you it was out of date. On
  startup it now asks GitLab whether a newer `gui/*` release exists and, if so,
  shows a line at the top of the window with a button to the release page.
  Every failure path is silent — offline, DNS, rate limit, a changed API — so a
  password tool never nags about the network. The check is on by default and
  can be switched off in Settings; that switch exists because this is an
  outbound call to gitlab.com on every start, which anyone self-hosting has a
  fair reason to decline. Nothing about the user or their databases is sent.

### Fixed

- **A server-only database could only be deleted from behind the ⋮ menu.** A
  database that exists on the server but is not bound locally has neither sync
  nor forget, so clearing it away was hidden in the overflow menu on the one
  row that needs it most. The row now carries a visible button. It uses the
  trash icon rather than the ✕ that sits on bound rows: there ✕ means forget
  the local binding, which touches neither the file nor the server, and one
  glyph must not stand for both a harmless unbind and a permanent delete.

## [gui/v0.3.4] — 2026-08-24

The release that makes the app start. Everything below the first entry was
already written; none of it reached anyone, because the two releases before
this one shipped a binary that could not launch on a machine that had not
built it.

### Fixed

- **The Windows app did not start: `libssp-0.dll` was not found.** The
  cross-build linked Debian mingw-w64's stack-protector library dynamically,
  and that file exists on no user's machine — so `gui/v0.3.2` and `gui/v0.3.3`
  both shipped an app that failed before drawing a window. Reported from a
  clean Windows Sandbox install. `-static` now pulls gcc's own runtime into the
  binary; Windows' own DLLs are imported exactly as before. The command-line
  client was never affected — it is pure Go, built with `CGO_ENABLED=0`, which
  is why running the commands by hand worked while the app did not. CI now
  reads the import table after packaging and fails on any `lib*.dll`, because a
  build that succeeds while producing an unstartable program is not a failure
  the user should be the first to find.

- **A local-only database appeared in the GUI as a database named `L`.** The
  list parser knew the `*` and `?` markers and nothing else, so the `L` that
  `add-local` introduced was read as the name, the name as the ID, and the path
  as the timestamp. It now recognises the marker, carries `LocalOnly` on the
  row, and handles both shapes of the table — with a server, where `(local
  only)` fills the ID column, and without one, where the columns are name and
  path alone.

- **The wizard's bottom button was clipped.** Adding a third way out of the
  welcome screen gave it a third row of buttons, and the layout pushes them
  flush to the window's edge — so the last one was cut off. The two secondary
  buttons now share a row, and a closing spacer keeps the block off the edge at
  any window height.

### Added

- **A Firefox section in the GUI** (`gui/`) — the extension stands or falls on
  two commands, `add-local` and `install-browser-host`, and both existed only
  on the command line. That is the wrong way round: the user who would rather
  not open a terminal is exactly the user who installs a program with windows
  and buttons, and on Windows the installer does not put the client on `PATH`
  either. The new tab carries both steps, a *Test* button per database that
  prints the index the browser would actually receive (`browser-host --probe`),
  a dry run of the registration, the uninstall, and a link to the setup guide.
  It also states, at the bottom, **where the command-line program is** — the
  concrete dead end behind this: the program was installed, the guide said to
  run `keepass-deltasync add-local`, and nothing anywhere said where that file
  was. The wizard gains a way in, too, because search needs no account and a
  welcome screen whose only offer is to get one answers a question the user
  did not ask.

## [gui/v0.3.3] — 2026-08-24

The application is unchanged from 0.3.2. This release exists to re-cut the
Windows installer, because the command-line client it bundles was older than
the extension that depends on it.

**Superseded by 0.3.4:** the Windows app in this release still cannot start —
see the `libssp-0.dll` entry there. The command-line client inside the
installer is fine, and was the point of the release.

### Fixed

- **The installer shipped a client without `add-local`.** `gui/v0.3.2` was
  built on 2026-08-20 and pairs the CLI from the newest `client/*` tag at the
  time, which predates the command by a day. The Firefox extension is public
  on addons.mozilla.org, its popup tells the user to run
  `keepass-deltasync add-local`, and a fresh Windows install answered `unknown
  command` — reproduced in Windows Sandbox on 2026-08-24, which is exactly
  what a new user, or an AMO reviewer following the submission notes, would
  have hit. The installer now carries client v1.8.0.

## [extension/v0.2.1] — 2026-08-24

Two corrections to what 0.2.0 put in front of users, and the first extension
release whose package was checked against the tree it claims to come from.

### Added

- **A setup button in the extension's popup** (0.2.0, corrected in 0.2.1) —
  the two dead ends
  ("cannot start the native host" and "no databases are registered") now say
  what is wrong in a sentence and offer a button to the setup guide, instead of
  printing a raw CLI command. The guide deliberately lives outside the
  extension: a signed add-on cannot be corrected without another AMO review,
  and sandbox paths are exactly the kind of instruction that goes stale.
  0.2.0 was packaged from a stale `dist/` build whose buttons still opened the
  repository's markdown file rather than the guide's `#host` / `#standalone`
  anchors; 0.2.1 ships the intended constants and nothing else.

### Fixed

- **Arrow keys in the extension popup while the pointer rests over it** — the
  result list selected on `mouseover`, and moving the selection rebuilds that
  list. The browser then fires `mouseover` on whatever element ends up under a
  completely stationary pointer, which put the selection straight back on the
  hovered row. Up/down looked dead whenever the mouse happened to sit over the
  popup — which it usually does, having just clicked the toolbar button. The
  list now selects on `mousemove`, guarded against a repeat of the same
  coordinates, so only real movement moves the selection.

- **0.2.0 shipped the wrong package.** The `.xpi` uploaded to AMO was built
  from an older working tree, so both setup buttons opened the repository's
  copy of the guide — a long markdown file, with no anchor for the dead end
  the reader had actually hit. Downloading the signed file and diffing it
  against the tree showed `popup.js` as the only real difference, `manifest.json`
  aside, which AMO re-serialises when it signs. `dist/` is git-ignored scratch
  and nothing stopped a stale build from being the newest file in it;
  [`extension/amo-submission.md`](extension/amo-submission.md) now carries the
  check that catches it in ten seconds.

## [client/v1.8.0] — 2026-08-24

The release that makes the Firefox extension usable. Everything the browser
half needs lives in the client — the native messaging host, the registration,
and a way to point at a `.kdbx` without an account — and until now the last of
those did not exist as a command. A user who only wanted to *find* the right
tab had to hand-edit `config.toml`, which is not an onboarding path.

Nothing about syncing changed, and no existing command behaves differently.

### Added

- **`browser-host` — the native messaging host for the Firefox extension.**
  The client half of the search-and-go feature: it opens the local `.kdbx`
  through `keepassxc-cli`, builds a title-and-URL index, and answers the
  extension over stdio. What it returns is an explicit allow-list — uuid,
  title, URLs and group path — so no future bug in the extension can surface a
  field the host never sent, and the masterpassword is read from the OS keyring
  by the host rather than passing through Firefox. A database without a keyring
  entry falls back to a prompt in the popup, held in memory under a 15-minute
  idle lock. Entries in the recycle bin and in groups with searching disabled
  are excluded, as are values that cannot be navigated to (`{REF:…}`
  placeholders, `cmd://`, non-http schemes). The index is paged, because
  Firefox caps a message from the host at 1 MB, and a `changed` push follows
  the file through fsnotify so an edited database re-indexes by itself.
  `--probe <name>` prints the whole index as JSON with no browser involved,
  which is both the debugging harness and the way to verify the boundary
  without taking anyone's word for it.

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

- **A *Firefox search* section in the menu** (`keepass-deltasync tui`) —
  `add-local` and `install-browser-host` were command-line only, which put the
  whole search-only path behind flags the user who wants it least is least
  likely to type. The section carries both, plus `--probe` to check the index
  and the uninstall, and it is the one section shown **without enrollment**:
  searching needs no account, so a menu whose only offer is to get one was
  answering a question nobody asked. The masterpassword checkbox prompts in the
  suspended terminal, the same way sync's prompt already does.

### Fixed

- **`install-browser-host` no longer registers with a Firefox that is not
  there.** Detection asked the very directory the command writes into —
  `~/.var/app/org.mozilla.firefox` for flatpak, `~/snap/firefox` for snap —
  and both are user data that outlives the package: `flatpak uninstall` keeps
  them without `--delete-data`, `snap remove` leaves `~/snap/<name>` behind,
  and on KDE plasma-browser-integration creates the flatpak one unprompted.
  The Linux test run on 2026-08-22 hit it: `flatpak list --app` was empty,
  while the command reported two variants and told the user to run a
  `flatpak override` for an app they did not have. Detection now asks flatpak
  and snapd through their own deploy directories, which disappear with the
  package; the manifest still goes where Firefox reads it. All three targets
  stay in the list with a `Detected` flag, so `uninstall-browser-host` can
  still clean up after a variant removed since registration, and `--all`
  covers an unusual layout.

### Changed

- **Go toolchain requirement relaxed to `go 1.26.0`** — `client/go.mod`
  declared `go 1.26.3`, an exact patch release that was simply whatever
  toolchain happened to be installed when `go mod tidy` last ran. Nothing
  needs that patch level, and it forced the F-Droid recipe to download a
  prebuilt Go tarball from go.dev — which F-Droid rejects. Debian
  trixie-backports ships `golang-go` 1.26, so the distribution's own package
  now suffices. Verified with `GOTOOLCHAIN=go1.26.0`: build, vet and the full
  test suite pass.

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

## [client/v1.7.0] — 2026-06-26

The terminal menu catches up with the desktop GUI, so a machine without a
graphical session is no longer a second-class way to run this.

### Added

- **The `tui` front page mirrors the GUI's tabs** — *Databases*, *Devices*,
  *Log*, *Admin* and *Settings* are now top-level entries, each opening a
  submenu with that area's actions, instead of one flat list of commands.
- **A full admin section** — list, create, enable, disable and delete users,
  issue enrolment tokens and print `token-sql`, all on top of the existing
  `admin` subcommands. The admin token is cached for the session and passed
  to the subprocess through the environment, never on the command line, where
  it would be visible to every other process on the machine.
- **Advanced enrolment from the not-enrolled screen** — an admin can issue a
  token and enrol the device in one step, the same flow the GUI wizard
  offers. Previously the terminal route required issuing the token somewhere
  else first.
- **Smaller additions** — *Databases* gained *delete on server*, and *Log*
  gained 24-hour, 7-day and 30-day filters.

## [client/v1.6.0] — 2026-06-26

### Fixed

- **`admin user-create` and `admin user-enrollment` could not run on a machine
  that was not enrolled yet.** Both resolved the server URL from
  `config.toml` alone, which is precisely the file that does not exist yet on
  a fresh machine — so the one command you need to bootstrap a device could
  not be run on a device that needed bootstrapping. Both now take an optional
  `--server` flag and fall back to the config, as `enroll` already did. This
  is what unblocks the GUI's advanced enrolment, which issues a token and
  enrols the PC before any config file is written.

## [client/v1.5.0] — 2026-06-24

### Added

- **`devices remove <id>`** — revoke an enrolled device from the desktop
  client. Calls `DELETE /api/v1/devices/{id}`, so the device's token is
  invalidated server-side; any of your own devices can be revoked,
  including the current one. This is the command the GUI's *Remove
  device* button invokes — previously the button failed because the
  subcommand did not exist (`devices` rejected all arguments).

## [client/v1.4.0] — 2026-06-22

**Folders sync.** Until now the sync was purely entry-based, keyed on UUID,
and the group tree was flattened away: renaming a group, moving an entry
between groups, or rearranging the tree simply did not reach your other
devices. New entries landed in a `deltasync` group and you filed them by hand,
on every device, every time.

This release makes the group structure a first-class synchronised object on
the desktop side. The design, including the wire format and the compatibility
rules, is in [`docs/v4-group-sync.md`](docs/v4-group-sync.md).

### Added

- **Groups on the wire** — `canonical.Group` is a self-contained object kind
  with its own envelope byte (`0x02`) and schema version, alongside entries.
  Entries gain `parent_group`, added without bumping the entry schema
  version: an older client ignores the unknown field, and an empty value is
  left out entirely, so blobs it does understand stay byte-identical.
- **Root as a sentinel** — every database has its own root UUID, so "lives in
  the root" travels as an empty string and each device maps it to its own
  root on arrival. Without that, objects would point at a group UUID that
  does not exist on the receiving device.
- **Push sends the tree** — the export now yields the group tree along with
  each entry's parent, and groups are uploaded before the entries that point
  at them. Moving an entry is detected too: a move bumps `LocationChanged`
  and not necessarily the modification time, so the push trigger is the later
  of the two.
- **Pull rebuilds the tree** — the pull asks for groups explicitly, and the
  staging database it hands to `keepassxc-cli merge` now carries the real
  group tree with each entry nested under its parent, rather than one flat
  group. Orphaned parents fall back to the root, group names are escaped, and
  cycles are broken rather than followed.
- **Deleting a group propagates** — group UUIDs seen in a previous sync are
  tracked per database, and one that is gone from the local export is
  tombstoned on the server. Verified against a live server: deleting a group
  on one device sends exactly one group tombstone.

### Known limitation

- On the desktop **receiving** side, `keepassxc-cli merge` honours
  `DeletedObjects` for entries but not for empty groups, so a deleted group's
  empty shell can linger on other desktops. Android applies it correctly.
  Entries inside a deleted group are removed everywhere, so this is cosmetic
  rather than data loss.

## [client/v1.3.0] — 2026-06-10

### Added

- **`delete-database <name|uuid>`** — delete a database on the server from
  the client. `forget` only ever dropped the local binding, so the database
  itself, its entries, versions, shares and history could only be removed
  with a hand-written HTTP call. The command takes a local name or a raw
  UUID — the latter so an unbound duplicate with no local name can still be
  removed — cross-checks the server's listing to show you the real name
  before you confirm, and refuses a database that is shared with you rather
  than owned by you. Any matching local binding is removed afterwards; the
  `.kdbx` file itself is left alone.

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
