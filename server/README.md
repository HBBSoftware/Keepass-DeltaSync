# keepass-deltasync-server

Sync-server til KeePass delta-sync-systemet. Ser kun klient-krypterede blobs — kender aldrig masterpassword. Multi-user med private databaser, ingen deling mellem brugere.

- **Sprog:** PHP 8.2+
- **Database:** PostgreSQL 14+
- **Licens:** [AGPL-3.0-or-later](LICENSE)

## Kom i gang

```sh
# 1. Installer afhængigheder
composer install

# 2. Kopier .env.example til .env og udfyld DB-credentials
cp .env.example .env
$EDITOR .env

# 3. Kør schema-migrationer (manuel i denne tidlige fase)
for f in schema/*.sql; do
  psql "$DATABASE_URL" -f "$f"
done

# 4. Opret første admin-token (ikke implementeret endnu)
bin/admin token:create-admin

# 5. Start dev-server
php -S localhost:8080 -t public/
```

## Struktur

```
public/    HTTP entry point (index.php)
bin/       Admin-CLI til oprettelse af brugere, tokens m.m.
src/       Applikationskode (PSR-4 autoload via composer)
schema/    SQL-migrationer (numerisk ordnet)
tests/     Integrationstests (PHPUnit)
```

## API

Alle endpoints ligger under `/api/v1/`. Auth via `Authorization: Bearer <token>`. Tre token-typer: **admin**, **enrollment** (engangs), **device**.

Den fulde endpoint-liste er registreret i [`src/Router.php`](src/Router.php). For semantik se [spec'en § API-endpoints](../keepass-deltasync-spec.md#api-endpoints).

### Path-prefix (valgfri)

Sæt `APP_BASE_PATH=/sync` i `.env` hvis serveren skal deployes under en under-sti — typisk for at dele subdomæne med en hjemmeside. Klienter peger så deres `server.url` til fx `https://host/sync`. Router'en strippes prefix før route-matching, så `src/Router.php` forbliver uændret. Tom værdi (default) = serveret på roden.

## Sikkerheds-grundregler

- TLS er påkrævet i drift — håndteres typisk af reverse-proxy (nginx, Caddy).
- Tokens lagres som Argon2id-hashes. Selve token-værdien returneres kun ved oprettelse.
- Server-administrator har **ingen** kryptografisk adgang til entry-indhold — blobs er krypteret med klient-afledte nøgler.
- Cross-user-isolation: alle requests scopes til den autentificerede brugers data. Adgang til andres ressourcer returnerer 404 (ikke 403) for at undgå information leak.

## Status

Skelet på plads. Følgende mangler stadig — se også [Milestone 1 i spec'en](../keepass-deltasync-spec.md#milestone-1--server-mvp):

- [ ] Request/Response-håndtering og route-dispatch
- [ ] PDO-forbindelse + transaktioner
- [ ] Token-auth-middleware (admin/enrollment/device)
- [ ] Controllere for hvert endpoint
- [ ] Audit-logger (INFO/DEBUG via `LOG_LEVEL`)
- [ ] Oprydnings-trigger ved opstart (advisory lock + 1-times throttle)
- [ ] Admin-CLI-kommandoer (`token:create-admin`, `user:create`, `user:delete`, ...)
- [ ] Integrationstests (cross-user isolation, version-rotation, log-oprydning)
