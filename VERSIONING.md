# Versioning

keepass-deltasync is a **monorepo** with three components that mature at
different rates. Each is versioned **independently**, and release tags are
**namespaced by component** so their version lines can never collide:

| Component | Tag format | Released by |
|-----------|------------|-------------|
| Desktop client (Go, `client/`) | `client/vX.Y.Z` | GitLab CI → cross-compiled binaries + GitLab Release |
| Android app (`android/`) | `android/vX.Y.Z` | Built & signed outside CI, then uploaded by `android/publish-release.sh` (the release keystore never enters CI). The app also carries its own `versionCode` + `versionName` in `android/app/build.gradle.kts` |
| Server (PHP, `server/`) | `server/vX.Y.Z` | Deployed directly; the tag is a marker only — no build artifact |

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

cd android
./gradlew :app:assembleRelease            # signed with the local keystore
GITHUB_TOKEN=ghp_xxx ./publish-release.sh # → GitHub Release, read by Obtainium
```

`versionCode` **must** increase on every release — Android refuses to install
an update that doesn't. `publish-release.sh` verifies that the APK's embedded
version matches `build.gradle.kts`, that it is signed, and that the tag exists
before it uploads anything; `DRY_RUN=1` runs the checks alone. See
[`android/README.md`](android/README.md) for the Obtainium setup and
[`android/F-DROID.md`](android/F-DROID.md) for the F-Droid track.

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
with the component (`client/`, `android/`, `server/`).
