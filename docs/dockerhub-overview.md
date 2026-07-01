# DeltaSync server

Self-hostable server for **end-to-end-encrypted delta-sync of KeePass `.kdbx`
databases** across your devices (desktop CLI, Android). The server only ever
stores ciphertext — it never sees your master password or keys.

Canonical project & source: **https://gitlab.com/Star95/keepass-deltasync**
(GPL-3.0). Mirror: https://github.com/HBBSoftware/Keepass-DeltaSync

## Tags

- `latest` — newest release
- `0.2.0`, … — pinned versions (recommended for production)

Multi-arch: `linux/amd64` + `linux/arm64` (x86 servers, NAS boxes, Raspberry
Pi, ARM VPS, Apple Silicon).

## Quick start (docker compose)

```yaml
services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_PASSWORD: CHANGE_THIS
    volumes: [ "pgdata:/var/lib/postgresql/data" ]
  app:
    image: hbbsoftware/deltasync-server:latest
    depends_on: [ db ]
    environment:
      DB_HOST: db
      DB_PASSWORD: CHANGE_THIS
    ports: [ "8080:80" ]
volumes:
  pgdata:
```

```sh
docker compose up -d
docker compose logs app     # copy the one-time admin token from the banner
```

The server is then on `http://<host>:8080`. A built-in `HEALTHCHECK`
(`GET /api/v1/health`) surfaces real status in `docker ps` and NAS UIs. Manage
it from the browser at `/admin.html` (token-authenticated admin panel: create
users, issue enrollment tokens — shown as scannable QR codes — and browse the
audit log).

Full guide (TrueNAS, HTTPS/reverse-proxy, and the canonical `compose.yml` with
the exact env-var names):
**https://gitlab.com/Star95/keepass-deltasync/-/blob/main/docs/self-hosting-docker.md**

> The compose snippet above is a minimal illustration — the repo's real
> [`compose.yml`](https://gitlab.com/Star95/keepass-deltasync/-/blob/main/compose.yml)
> uses a YAML anchor so you only set the password once.

## License

GPL-3.0-or-later.
