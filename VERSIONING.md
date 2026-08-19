# Versioning

keepass-deltasync is a **monorepo** with four components that mature at
different rates. Each is versioned **independently**, and release tags are
**namespaced by component** so their version lines can never collide:

| Component | Tag format | Released by |
|-----------|------------|-------------|
| Desktop client (Go, `client/`) | `client/vX.Y.Z` | GitLab CI → cross-compiled binaries + GitLab Release |
| Android app (`android/`) | `android/vX.Y.Z` | CI builds an **unsigned** APK; `android/publish-release.sh` signs it locally and publishes it (the release keystore never enters CI). The app also carries its own `versionCode` + `versionName` in `android/app/build.gradle.kts` |
| Server (PHP, `server/`) | `server/vX.Y.Z` | Deployed directly; the tag is a marker only — no build artifact |
| Firefox extension (`extension/`) | `extension/vX.Y.Z` | CI packages an **unsigned**, byte-reproducible `.xpi` via `extension/package.sh`; signing happens outside CI. The extension also carries its own `version` in `extension/manifest.json` |

The components do **not** share a number. The desktop client being at `1.x`
while the Android app is at `0.x` is correct and expected — they are different
programs.

## Cutting a release

**Desktop client** (CI builds the four binaries and publishes a GitLab Release
attached to the tag):

```
git tag client/v1.1.0
git push origin client/v1.1.0
```

**Android app**: bump `versionCode` and `versionName` in
`android/app/build.gradle.kts`, commit, then tag, build and publish:

```
git tag android/v0.4.0
git push origin android/v0.4.0
```

That triggers `build:android`, which produces `dist/DeltaSync-<version>-unsigned.apk`
as a job artifact. Download and unzip it in the repo root, then:

```
cd android
GITHUB_TOKEN=ghp_xxx ./publish-release.sh # signs locally → GitHub Release
```

CI stops short of signing on purpose: the release keystore is what binds every
future update to this project, and CI variables are readable by every
Maintainer and by any compromised job. To build without CI, run
`./gradlew :app:assembleRelease` locally instead — the script accepts either.

`versionCode` **must** increase on every release — Android refuses to install
an update that doesn't. `publish-release.sh` verifies that the APK's embedded
version matches `build.gradle.kts`, that it is signed, and that the tag exists
before it uploads anything; `DRY_RUN=1` runs the checks alone. See
[`android/README.md`](android/README.md) for the Obtainium setup and
[`android/F-DROID.md`](android/F-DROID.md) for the F-Droid track.

**Firefox extension**: bump `version` in `extension/manifest.json`, commit,
then tag:

```
git tag extension/v0.2.0
git push origin extension/v0.2.0
```

`build:extension` packages `dist/keepass-deltasync-extension-<version>.xpi`.
The job refuses to build if the tag and `manifest.json` disagree — the same
drift check `build:android` does against `build.gradle.kts`.

The extension is versioned separately from the desktop client even though the
two must be protocol-compatible: the native messaging host is a subcommand of
the `keepass-deltasync` binary, so a protocol change touches both components
in one commit. What keeps them honest is the extension ID, which appears in
**both** `extension/manifest.json` and `browserExtensionID` in
`client/cmd/keepass-deltasync/browser_host.go`. Change it one place only and
the host will silently refuse to talk to the extension. See
[`docs/browser-extension.md`](docs/browser-extension.md).

**Server**: deploy `server/`, then tag for the record:

```
git tag server/v1.0.0
git push origin server/v1.0.0
```

## Legacy tags — do not reuse

Two bare `vX.Y.Z` tags predate this scheme and are kept for history only. They
**no longer trigger CI**:

- **`v1.0.0`** (2026-05-28) — the desktop client's first public release. This is
  the real 1.0 of the Go client; future client releases continue **above** it
  as `client/v1.1.0`, `client/v1.2.0`, …
- **`v0.1.0`** (2026-05-29) — an attempt at a single unified product version. It
  re-stamped the *same* client binaries as `0.1.0`, which read as a downgrade
  from `1.0.0` and broke SemVer monotonicity. It is superseded by this
  per-component scheme; do not build on it.

**Rule of thumb:** never create a bare `vX.Y.Z` tag again — always prefix it
with the component (`client/`, `android/`, `server/`, `extension/`).
