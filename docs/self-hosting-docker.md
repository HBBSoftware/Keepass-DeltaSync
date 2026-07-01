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
