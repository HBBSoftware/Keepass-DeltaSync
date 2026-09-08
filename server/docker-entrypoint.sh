#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Container entrypoint: wait for PostgreSQL, apply schema migrations (once,
# incrementally), bootstrap the first admin token, then exec the web server.
#
# psql/pg_isready read the standard libpq env vars (PGHOST, PGPORT, PGUSER,
# PGPASSWORD, PGDATABASE) — set in compose.yml. The PHP app reads its own
# DATABASE_URL/DATABASE_USER/DATABASE_PASSWORD; both point at the same DB.

set -eu

APP_DIR=/var/www/app

# A misconfiguration used to end in `exit 1`, which under `restart:
# unless-stopped` means a crash loop: the port never opens and a browser says
# nothing more useful than "unable to connect". The reason was only ever
# visible to whoever thought to run `docker logs`.
#
# Instead we record why, and start the web server anyway. Every request then
# answers 503 with the reason, including /api/v1/health — so the container
# still reports UNHEALTHY to Docker and the NAS, which is correct, while a
# human who opens the address gets told what to fix.
STARTUP_ERROR_FILE="$APP_DIR/.startup-error"

fail_startup() {
    echo "[entrypoint] STARTUP ERROR: $1" >&2
    printf '%s\n' "$1" > "$STARTUP_ERROR_FILE" 2>/dev/null || true
    echo "[entrypoint] serving the diagnostic page instead; the app is not usable until this is fixed." >&2
    exec apache2-foreground
}

# Only run the DB bootstrap when we're about to start the web server. This lets
# you run one-off admin commands without re-triggering migrations, e.g.:
#   docker compose run --rm app php bin/admin user:create alice
if [ "${1:-}" = "apache2-foreground" ]; then
    # A previous boot may have left one behind; a fixed config must clear it.
    rm -f "$STARTUP_ERROR_FILE"

    : "${PGHOST:=db}"
    : "${PGPORT:=5432}"
    : "${PGUSER:=deltasync}"

    echo "[entrypoint] waiting for PostgreSQL at ${PGHOST}:${PGPORT} ..."
    waited=0
    until pg_isready -q -h "$PGHOST" -p "$PGPORT" -U "$PGUSER"; do
        waited=$((waited + 1))
        if [ "$waited" -ge 120 ]; then
            fail_startup "PostgreSQL at ${PGHOST}:${PGPORT} did not become ready within 120s. Check that the database container is running and that PGHOST/PGPORT point at it."
        fi
        sleep 1
    done
    echo "[entrypoint] PostgreSQL is up."

    # --- Schema migrations --------------------------------------------------
    # A small bookkeeping table records which schema/*.sql files have run, so
    # restarts are no-ops and future schema files are picked up incrementally.
    psql -v ON_ERROR_STOP=1 -q -c \
        'CREATE TABLE IF NOT EXISTS _container_migrations (
             filename   text PRIMARY KEY,
             applied_at timestamptz NOT NULL DEFAULT now()
         );'

    for f in "$APP_DIR"/schema/*.sql; do
        name="$(basename "$f")"
        applied="$(psql -tA -c "SELECT 1 FROM _container_migrations WHERE filename = '$name';")"
        if [ "$applied" = "1" ]; then
            continue
        fi
        echo "[entrypoint] applying migration: $name"
        # File + bookkeeping insert run as ONE transaction: if the SQL fails,
        # ON_ERROR_STOP aborts and the insert is rolled back, so it retries
        # cleanly next start.
        if ! {
            cat "$f"
            printf "\nINSERT INTO _container_migrations (filename) VALUES ('%s');\n" "$name"
        } | psql -v ON_ERROR_STOP=1 -q --single-transaction; then
            fail_startup "Schema migration [$name] failed. The database rejected it, so nothing was applied. Check the container log for the SQL error, and that DATABASE_USER may create tables."
        fi
    done
    echo "[entrypoint] migrations up to date."

    # --- Admin token from the environment (optional) -------------------------
    # ADMIN_TOKEN — or ADMIN_TOKEN_FILE, for a Docker/Kubernetes secret — lets
    # the operator decide the admin token up front instead of fishing it out of
    # this log. TokenHasher::hash() is plain SHA-256 (see src/Crypto/
    # TokenHasher.php for why), so we can compute the very same hash here.
    # Idempotent: ON CONFLICT DO NOTHING, so restarts are a no-op.
    if [ -n "${ADMIN_TOKEN_FILE:-}" ]; then
        if [ ! -r "$ADMIN_TOKEN_FILE" ]; then
            fail_startup "ADMIN_TOKEN_FILE is set but not readable: $ADMIN_TOKEN_FILE"
        fi
        ADMIN_TOKEN="$(head -n 1 "$ADMIN_TOKEN_FILE" | tr -d '\r\n')"
    fi

    if [ -n "${ADMIN_TOKEN:-}" ]; then
        # A generated token is 43 chars (32 random bytes, base64url). Refuse
        # anything short enough to be guessed — this is the master credential.
        # The length is logged, not the token. A value that arrives shorter
        # than it was typed is the signature of something eating it on the way
        # in — Docker Compose expands an unescaped $ in a YAML value, and YAML
        # itself drops everything after an unquoted ' #'.
        echo "[entrypoint] ADMIN_TOKEN received: ${#ADMIN_TOKEN} characters."
        if [ "${#ADMIN_TOKEN}" -lt 24 ]; then
            fail_startup "ADMIN_TOKEN is ${#ADMIN_TOKEN} characters; at least 24 are required. If you set a longer one, something shortened it: quote the value, and write a literal dollar sign as \$\$ (Compose expands a single \$ as a variable). Generate a safe one with: openssl rand -base64 32 | tr '+/' '-_' | tr -d '='"
        fi

        admin_hash="$(printf '%s' "$ADMIN_TOKEN" | sha256sum | cut -d' ' -f1)"
        if [ "${#admin_hash}" -ne 64 ] || [ -n "$(printf '%s' "$admin_hash" | tr -d '0-9a-f')" ]; then
            fail_startup "Could not compute a SHA-256 hash of ADMIN_TOKEN."
        fi

        # The hash is hex by construction (checked above), so interpolating it
        # into the statement cannot inject SQL. The token itself never lands
        # in the query, the log, or the process list.
        psql -v ON_ERROR_STOP=1 -q -c \
            "INSERT INTO admin_tokens (token_hash) VALUES ('$admin_hash') ON CONFLICT DO NOTHING;"
        echo "[entrypoint] admin token supplied via the environment is registered."
    fi

    # --- Admin account from the environment (optional) -----------------------
    # ADMIN_USERNAME + ADMIN_PASSWORD give the browser panel a real login.
    # Unlike the token above, the password has to be hashed with Argon2id,
    # which the shell cannot do — so this hands off to the PHP CLI. The
    # command is idempotent and only writes when something actually changed.
    if [ -n "${ADMIN_PASSWORD_FILE:-}" ]; then
        if [ ! -r "$ADMIN_PASSWORD_FILE" ]; then
            fail_startup "ADMIN_PASSWORD_FILE is set but not readable: $ADMIN_PASSWORD_FILE"
        fi
        ADMIN_PASSWORD="$(head -n 1 "$ADMIN_PASSWORD_FILE" | tr -d '\r\n')"
        export ADMIN_PASSWORD
    fi

    admin_login_configured=0
    if [ -n "${ADMIN_USERNAME:-}" ] && [ -n "${ADMIN_PASSWORD:-}" ]; then
        # Same reasoning as ADMIN_TOKEN above: the length is the one detail
        # that catches a value mangled in transit, and it is not a secret.
        echo "[entrypoint] admin account: username=[${ADMIN_USERNAME}], password is ${#ADMIN_PASSWORD} characters."
        if [ "${#ADMIN_PASSWORD}" -lt 12 ]; then
            fail_startup "ADMIN_PASSWORD is ${#ADMIN_PASSWORD} characters; at least 12 are required. If you set a longer one, something shortened it: quote the value, and write a literal dollar sign as \$\$ (Compose expands a single \$ as a variable)."
        fi
        if ! php "$APP_DIR/bin/admin" admin:ensure; then
            fail_startup "Could not create or update the admin account. See the container log for the reason."
        fi
        admin_login_configured=1
    elif [ -n "${ADMIN_USERNAME:-}" ] || [ -n "${ADMIN_PASSWORD:-}" ]; then
        fail_startup "Set BOTH ADMIN_USERNAME and ADMIN_PASSWORD, or neither. Only one of them is set."
    fi

    # --- First-time admin token ---------------------------------------------
    # If no admin token exists yet, mint one and print it ONCE (mirrors the
    # setup.php wizard's final step). Capture it from the container logs.
    admin_count="$(psql -tA -c 'SELECT count(*) FROM admin_tokens;')"
    if [ "$admin_count" = "0" ]; then
        if [ "$admin_login_configured" = "1" ]; then
            # A login exists, so there is nothing to fish out of this log —
            # which was the whole point. Tokens stay available for machines.
            echo "[entrypoint] admin login configured; no admin token minted."
            echo "[entrypoint] Need one for the CLI or API? Run:"
            echo "[entrypoint]   php bin/admin token:create-admin"
        else
            echo "[entrypoint] no admin token found — creating the first one:"
            echo "============================================================"
            php "$APP_DIR/bin/admin" token:create-admin
            echo "============================================================"
            echo "[entrypoint] SAVE THE ADMIN TOKEN ABOVE — it is shown only once."
        fi
    fi
fi

exec "$@"
