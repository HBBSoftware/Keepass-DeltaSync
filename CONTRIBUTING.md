# Contributing to DeltaSync

Thanks for your interest. This is a monorepo with five code components
(server, desktop client, desktop GUI, Android, Firefox extension) plus shared
docs — see the [README](README.md#repository-layout) for the layout and
per-component licenses.

## Ground rules

- **DCO sign-off is required** on every commit. Add a `Signed-off-by` line by
  committing with `-s`:

  ```sh
  git commit -s -m "your message"
  ```

  By signing off you certify the [Developer Certificate of Origin](https://developercertificate.org/).

- **Security issues do not go in public issues or merge requests.** Follow
  [`SECURITY.md`](SECURITY.md) instead.

- **Crypto and auth code is sensitive.** Changes to encryption, key handling,
  token validation, or the sharing key-wrap require review and sign-off from at
  least two maintainers.

## Versioning & releases

Release tags are namespaced per component so their version lines never collide
(see [`VERSIONING.md`](VERSIONING.md)):

| Tag                | Component         | What CI does on the tag                          |
|--------------------|-------------------|--------------------------------------------------|
| `client/vX.Y.Z`    | Desktop client    | Cross-compiles binaries, publishes a GitLab Release |
| `gui/vX.Y.Z`       | Desktop GUI       | Packages the Linux and Windows builds plus the combined GUI+CLI Windows installer, publishes a GitLab Release |
| `server/vX.Y.Z`    | Server            | Builds & pushes a multi-arch Docker image         |
| `android/vX.Y.Z`   | Android app       | Builds an unsigned APK; signed outside CI, drives the F-Droid recipe |
| `extension/vX.Y.Z` | Firefox extension | Packages an unsigned `.xpi`; signed outside CI    |

Bare `vX.Y.Z` tags are legacy and trigger nothing.

## Building & testing locally

**Desktop client (Go ≥ 1.26):**

```sh
cd client
go vet ./...
go test ./...
go build -o keepass-deltasync ./cmd/keepass-deltasync
```

**Desktop GUI (Go ≥ 1.23, plus CGO and a C toolchain):**

```sh
cd gui
go vet ./...
go build -o keepass-deltasync-gui .
```

The GUI is its own Go module and needs CGO — Fyne binds OpenGL and X11 — so it
also wants `gcc pkg-config libgl1-mesa-dev xorg-dev` on Debian/Ubuntu, a
mingw-w64 gcc on Windows, or the Xcode command line tools on macOS. It owns no
crypto, server or config logic; it shells out to the client binary, so most
behaviour changes belong in `client/`. See [`gui/README.md`](gui/README.md),
and [`gui/installer/`](gui/installer/) for the combined Windows installer.

**Server (PHP 8.2+, ext `pdo_pgsql`, `sodium`, `json`):**

```sh
cd server
composer install            # dev-only: pulls PHPUnit (runtime has no deps)
./vendor/bin/phpunit
```

There is no build step for the server — it is plain PHP with its own PSR-4
autoloader. The quickest way to run the whole stack locally is Docker:

```sh
docker compose -f compose.build.yml up -d --build
```

**Android (JDK from Android Studio's bundled JBR):**

```sh
cd android
./gradlew :sync:test           # pure-JVM sync core tests
./gradlew :app:assembleDebug   # installable debug APK
```

The crypto/canonical-format layer is Go compiled via `gomobile bind`; see
[`android/README.md`](android/README.md) for regenerating the `.aar`.

**Firefox extension (no build step):**

```sh
npx web-ext lint --source-dir extension --ignore-files package.sh README.md
sh extension/package.sh          # → dist/*.xpi
```

The extension talks to `keepass-deltasync browser-host` over native messaging,
so a protocol change touches `client/` in the same commit — see
[`docs/browser-extension.md`](docs/browser-extension.md).

## Commit messages

- Use an imperative, present-tense summary; prefix with the area when it helps
  (`server:`, `client:`, `gui:`, `android:`, `extension:`, `ci:`, `docs:`,
  `fdroid:`).
- Explain the *why* in the body for anything non-obvious.

## Merge requests

- Keep changes focused; one logical change per MR.
- Make sure the relevant component's tests pass.
- Update docs and `CHANGELOG.md` when behaviour changes.
