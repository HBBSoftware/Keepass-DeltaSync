# Self-hosting DeltaSync with Docker

The easiest way to run your own DeltaSync server. You get a prebuilt image, so
there is **no source checkout and no build** — just a bit of YAML, a password,
and a port.

The image is published to the project's GitLab Container Registry and is
pullable anonymously:

```
registry.gitlab.com/star95/keepass-deltasync/server:latest
registry.gitlab.com/star95/keepass-deltasync/server:0.1.0   # pin a version
```

It is multi-arch (`linux/amd64` + `linux/arm64`), so it runs on x86 servers and
NAS boxes as well as ARM devices (Raspberry Pi, ARM VPS, Apple Silicon).

---

## Option A — `docker compose` (any Linux host / VPS)

1. Grab [`compose.yml`](../compose.yml) from this repo (or copy its contents).
2. Change the one line marked `CHANGE_THIS` to a long random password.
3. Start it and read the one-time admin token from the logs:

```sh
docker compose up -d
docker compose logs app        # look for the "admin token" banner
```

The server is now on `http://<host>:8080`.

### Setting the admin token yourself

Digging a token out of a log is awkward, and it is gone once the log rotates.
Set `ADMIN_TOKEN` in the `app` service instead and you decide it up front:

```sh
openssl rand -base64 32 | tr '+/' '-_' | tr -d '='   # generate one
```

The server registers that token on every start (it is stored as a SHA-256
hash, so re-running is a no-op) and skips the generated-token banner
entirely. It must be at least 24 characters — the server refuses to start
otherwise, because this is the credential that administers every user.

`ADMIN_TOKEN_FILE=/run/secrets/admin_token` does the same thing from a file,
for Docker or Kubernetes secrets. It wins over `ADMIN_TOKEN` when both are set.

Setting it later works too: the token is added alongside any existing one, so
nothing is lost if you already have a token you like.

### A real login for the admin panel

Pasting a token at every visit gets old, and the token has to live somewhere in
the browser while you work. Set `ADMIN_USERNAME` and `ADMIN_PASSWORD` instead
and the panel gets an ordinary sign-in form:

```yaml
    environment:
      ADMIN_USERNAME: "admin"
      ADMIN_PASSWORD: "a-long-passphrase"
```

The password is hashed with Argon2id (tune it with `ARGON2_MEMORY_COST`,
`ARGON2_TIME_COST` and `ARGON2_THREADS`), and signing in sets an `HttpOnly`
session cookie — so unlike a pasted token, the credential is never reachable
from JavaScript. The session lasts `ADMIN_SESSION_TTL_HOURS` (8 by default) and
slides forward while you work, so a day in the panel needs one sign-in.
Changing the password logs every open session out.

Login attempts are rate-limited per IP to `RATE_LIMIT_AUTH_PER_MINUTE`
(10 by default). A password is guessable in a way a 256-bit token is not, which
is why the limit matters here and not for token auth.

Both `ADMIN_PASSWORD_FILE` and the CLI work as alternatives:

```sh
docker compose exec app php bin/admin admin:set-password admin
```

**Bearer tokens keep working.** `bin/admin` and the client's `keepass-deltasync
admin` commands still authenticate with a token — the login is a second way in
for humans with a browser, not a replacement.

### Behind a reverse proxy

The session cookie only gets the `Secure` flag when the connection is actually
HTTPS. The server itself always speaks plain HTTP, so behind a TLS-terminating
proxy that fact can only arrive in the `X-Forwarded-Proto` header — which any
client can send. So it is honoured only from addresses you list:

```yaml
      TRUSTED_PROXIES: "172.16.0.0/12"
```

Leave it empty on a plain-HTTP LAN install. The cookie is then set without
`Secure`, which is what makes signing in possible at all — a `Secure` cookie
over HTTP is discarded by the browser. The same setting makes `X-Forwarded-For`
trusted, so the audit log and the rate limiter see real client IPs instead of
the proxy's.

---

## Option B — TrueNAS SCALE (Custom App)

TrueNAS SCALE (Electric Eel and newer) runs native Docker and accepts a
compose file directly.

1. **Apps → Discover Apps → Custom App → Install via YAML.**
2. Paste the contents of [`compose.yml`](../compose.yml).
3. Change the `CHANGE_THIS` password line. (One edit — a YAML anchor feeds it to
   all services.)
4. *(Optional)* change the published port `8080:80` if 8080 is taken.
5. Install. When it's running, open the app's **Logs** and copy the one-time
   **admin token** from the startup banner.

The data lives in the `pgdata` Docker volume, so it survives restarts and
updates. (For a host-path dataset instead of a named volume, point the
`db` volume at a TrueNAS dataset.)

---

## After it's running: create your first user

The admin token authenticates the admin API. Use it to create a user and get an
enrollment token for your phone/desktop client. Either call the admin API
directly, or run the bundled CLI inside the container:

```sh
docker compose exec app php bin/admin user:create alice
# → prints an enrollment token; on the device:  keepass-deltasync enroll <token>
```

(`docker compose run --rm app php bin/admin ...` works too and does not touch
the running web server.)

## Web admin panel

Prefer a browser? Open `https://<your-server>/admin.html` and paste your admin
token. From there you can create users, issue enrollment tokens,
enable/disable and delete users, and browse the audit log — no command line
needed. The token is held only in the browser tab's session storage and sent
as a bearer token to the existing admin API.

Each issued enrollment token is also shown as a QR code that bundles the server
URL and token. On Android, open **Enroll → Scan QR code** to bind the device by
scanning it instead of typing the token by hand.

---

## HTTPS

The container speaks plain HTTP on port 80 (published as 8080). For anything
reachable from the internet, terminate TLS in front of it:

- a reverse proxy (Caddy/Traefik/nginx), or
- your NAS's built-in reverse proxy / Let's Encrypt integration.

Point the proxy at the container and serve it over `https://`. Clients then use
that HTTPS URL as their server address.

---

## Updating

```sh
docker compose pull && docker compose up -d
```

Schema migrations run automatically on start (idempotently — already-applied
migrations are skipped, and new ones in a newer image are picked up). Pin a
version tag instead of `:latest` if you want to control exactly when you move.

---

## Backups

All state is in PostgreSQL. Back up the `pgdata` volume, or dump the database:

```sh
docker compose exec db pg_dump -U deltasync keepass_deltasync > deltasync-backup.sql
```

Remember: the server only ever stores **client-encrypted** blobs — a backup
contains no readable passwords. But it does contain your data, so store it
safely.

---

## Notes

- **No public/default server exists** — DeltaSync only works against a server
  you (or someone you trust) runs. This is that server.
- The image is **stateless**; you can delete and recreate the `app` container
  freely. Only the `db` volume holds data.
- Developers who want to build from local source use
  [`compose.build.yml`](../compose.build.yml) instead (see `.env.example`).
