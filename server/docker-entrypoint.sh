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

# Only run the DB bootstrap when we're about to start the web server. This lets
# you run one-off admin commands without re-triggering migrations, e.g.:
#   docker compose run --rm app php bin/admin user:create alice
if [ "${1:-}" = "apache2-foreground" ]; then
    : "${PGHOST:=db}"
    : "${PGPORT:=5432}"
    : "${PGUSER:=deltasync}"

    echo "[entrypoint] waiting for PostgreSQL at ${PGHOST}:${PGPORT} ..."
    until pg_isready -q -h "$PGHOST" -p "$PGPORT" -U "$PGUSER"; do
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
        {
            cat "$f"
            printf "\nINSERT INTO _container_migrations (filename) VALUES ('%s');\n" "$name"
        } | psql -v ON_ERROR_STOP=1 -q --single-transaction
    done
    echo "[entrypoint] migrations up to date."

    # --- First-time admin token ---------------------------------------------
    # If no admin token exists yet, mint one and print it ONCE (mirrors the
    # setup.php wizard's final step). Capture it from the container logs.
    admin_count="$(psql -tA -c 'SELECT count(*) FROM admin_tokens;')"
    if [ "$admin_count" = "0" ]; then
        echo "[entrypoint] no admin token found — creating the first one:"
        echo "============================================================"
        php "$APP_DIR/bin/admin" token:create-admin
        echo "============================================================"
        echo "[entrypoint] SAVE THE ADMIN TOKEN ABOVE — it is shown only once."
    fi
fi

exec "$@"
