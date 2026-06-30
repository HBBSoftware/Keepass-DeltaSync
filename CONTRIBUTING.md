# Contributing to DeltaSync

Thanks for your interest. This is a monorepo with three code components
(server, desktop client, Android) plus shared docs — see the
[README](README.md#repository-layout) for the layout and per-component
licenses.

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

| Tag             | Component      | What CI does on the tag                          |
|-----------------|----------------|--------------------------------------------------|
| `client/vX.Y.Z` | Desktop client | Cross-compiles binaries, publishes a GitLab Release |
| `server/vX.Y.Z` | Server         | Builds & pushes a multi-arch Docker image         |
| `android/vX.Y.Z`| Android app    | Built & signed outside CI; drives the F-Droid recipe |

Bare `vX.Y.Z` tags are legacy and trigger nothing.

## Building & testing locally

**Desktop client (Go ≥ 1.22):**

```sh
cd client
go vet ./...
go test ./...
go build -o keepass-deltasync ./cmd/keepass-deltasync
```

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

## Commit messages

- Use an imperative, present-tense summary; prefix with the area when it helps
  (`server:`, `client:`, `android:`, `ci:`, `docs:`, `fdroid:`).
- Explain the *why* in the body for anything non-obvious.

## Merge requests

- Keep changes focused; one logical change per MR.
- Make sure the relevant component's tests pass.
- Update docs and `CHANGELOG.md` when behaviour changes.
