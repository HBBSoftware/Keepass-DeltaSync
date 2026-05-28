# Deployment

Tre konkrete recipes til at deploye keepass-deltasync-server. Vælg ud fra hvor
du vil have hjemmesiden (hvis nogen) ift. API'en.

## Recipe 1: API'en alene på et subdomæne (default)

Klassisk setup: `https://deltasync.example.com/` viser API'en (eller en
landing page hvis du lægger `index.html` ved siden af `index.php` i `public/`).

```
~/server/
  bootstrap.php
  .env
  src/
  public/                     ← Apache DocumentRoot
    index.php
    .htaccess
    setup.php                 ← slet efter første admin-token-bootstrap!
```

**Konfiguration:**
- Web root i Hetzner control panel: `~/server/public/`
- `.env`: ingen `APP_BASE_PATH` (eller tom)
- Klient: `url = "https://deltasync.example.com"`

API svarer på `https://deltasync.example.com/api/v1/...`.

## Recipe 2: API under sub-path, hjemmeside på roden

Brug-case: dele subdomæne mellem en hjemmeside og API'en.
`https://deltasync.example.com/` viser hjemmesiden;
`https://deltasync.example.com/sync/api/v1/...` er API'en.

```
~/server/                     ← server-koden bevares hvor den er
  bootstrap.php
  .env
  src/
  public/                     ← ikke længere DocumentRoot, men intakt
    index.php                 ← kan bruges, men bruges normalt ikke

~/web-root/                   ← ny Apache DocumentRoot
  index.html                  ← din hjemmeside-forside
  style.css
  about.html
  sync/                       ← API-prefix
    index.php                 ← kopi af ~/server/public/index.php
    .htaccess                 ← kopi af ~/server/public/.htaccess
```

**Trin:**

1. Opret `~/web-root/sync/` og kopiér disse to filer fra `~/server/public/`:
   - `index.php`
   - `.htaccess`

2. Fortæl `index.php` hvor server-koden bor — tilføj én linje øverst i
   `~/web-root/sync/.htaccess`:
   ```apache
   SetEnv APP_ROOT /usr/home/<dig>/server
   ```
   (Brug absolut sti til mappen der indeholder `bootstrap.php`.)

3. I `~/server/.env`:
   ```
   APP_BASE_PATH=/sync
   ```

4. Web root i Hetzner control panel: `~/web-root/`.

5. Klient-config: `url = "https://deltasync.example.com/sync"`.

6. Læg din hjemmeside-content i `~/web-root/` (index.html etc.).

`.htaccess`'en kan symlinkes i stedet for kopieres hvis Hetzner tillader
det. Den modificerede `index.php` skal være en kopi, ikke symlink (for at
have separate filer pr. deployment).

**Verifikation:**
```sh
curl -H "Authorization: Bearer $TOKEN" https://deltasync.example.com/sync/api/v1/me
# → 200 OK + user/device-info
```

## Recipe 3: Subdomæne-separation (cleanest, kræver vhost-adgang)

`api.example.com` for API, `www.example.com` for hjemmeside. To separate
vhosts, hver med eget DocumentRoot.

```
~/api/                        ← vhost: api.example.com
  bootstrap.php
  src/
  public/                     ← DocumentRoot for api.example.com
    index.php
    .htaccess

~/www/                        ← vhost: www.example.com
  index.html
  ...
```

**Konfiguration:**
- Web roots: separat pr. vhost
- `.env`: ingen `APP_BASE_PATH`
- Klient: `url = "https://api.example.com"`

Det er den reneste model — ingen path-magi, ingen overlap. Men kræver at du
kan oprette ekstra vhosts/subdomæner hos din hoster.

## Fælles deployment-trin (alle recipes)

1. **Upload server/-koden** (PSR-4 fallback i `bootstrap.php` betyder du
   ikke behøver `composer install` på hosten — vendor/ er ikke krævet).
2. **Opret database** i PostgreSQL 14+. Kør migrationer:
   ```sh
   for f in schema/*.sql; do psql "$DATABASE_URL" -f "$f"; done
   ```
   Eller paste hver migration i DBeaver hvis du ikke har shell-adgang.
3. **Udfyld `.env`** baseret på `.env.example`.
4. **Første admin-token**: kør `setup.php` ÉN gang via browser, eller brug
   `keepass-deltasync admin token-sql` (paste SQL'en i DBeaver).
5. **Slet `setup.php`** fra `public/` (eller `web-root/sync/` for Recipe 2)
   så det ikke kan misbruges.

## Path-prefix mekanik (Recipe 2 detaljer)

Når `APP_BASE_PATH=/sync` er sat:

- Router strippes prefix fra `request->path` før route-matching, så
  `/sync/api/v1/me` matcher den eksisterende route `/api/v1/me`.
- Sub-stier udenfor prefix returnerer 404 (normal opførsel).
- `/syncplus/foo` matcher IKKE prefix (boundary-tjek kræver "/" efter).

Klient-koden er prefix-agnostisk: den concatener bare `cfg.Server.URL` med
endpoint-stier. Du behøver kun at ændre URL'en i hver klients `config.toml`.
