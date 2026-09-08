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

    # --- Admin token from the environment (optional) -------------------------
    # ADMIN_TOKEN — or ADMIN_TOKEN_FILE, for a Docker/Kubernetes secret — lets
    # the operator decide the admin token up front instead of fishing it out of
    # this log. TokenHasher::hash() is plain SHA-256 (see src/Crypto/
    # TokenHasher.php for why), so we can compute the very same hash here.
    # Idempotent: ON CONFLICT DO NOTHING, so restarts are a no-op.
    if [ -n "${ADMIN_TOKEN_FILE:-}" ]; then
        if [ ! -r "$ADMIN_TOKEN_FILE" ]; then
            echo "[entrypoint] ERROR: ADMIN_TOKEN_FILE is set but not readable: $ADMIN_TOKEN_FILE" >&2
            exit 1
        fi
        ADMIN_TOKEN="$(head -n 1 "$ADMIN_TOKEN_FILE" | tr -d '\r\n')"
    fi

    if [ -n "${ADMIN_TOKEN:-}" ]; then
        # A generated token is 43 chars (32 random bytes, base64url). Refuse
        # anything short enough to be guessed — this is the master credential.
        if [ "${#ADMIN_TOKEN}" -lt 24 ]; then
            echo "[entrypoint] ERROR: ADMIN_TOKEN is ${#ADMIN_TOKEN} characters; at least 24 are required." >&2
            echo "[entrypoint] Generate one with:" >&2
            echo "[entrypoint]   openssl rand -base64 32 | tr '+/' '-_' | tr -d '='" >&2
            exit 1
        fi

        admin_hash="$(printf '%s' "$ADMIN_TOKEN" | sha256sum | cut -d' ' -f1)"
        if [ "${#admin_hash}" -ne 64 ] || [ -n "$(printf '%s' "$admin_hash" | tr -d '0-9a-f')" ]; then
            echo "[entrypoint] ERROR: could not compute a SHA-256 hash of ADMIN_TOKEN." >&2
            exit 1
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
            echo "[entrypoint] ERROR: ADMIN_PASSWORD_FILE is set but not readable: $ADMIN_PASSWORD_FILE" >&2
            exit 1
        fi
        ADMIN_PASSWORD="$(head -n 1 "$ADMIN_PASSWORD_FILE" | tr -d '\r\n')"
        export ADMIN_PASSWORD
    fi

    admin_login_configured=0
    if [ -n "${ADMIN_USERNAME:-}" ] && [ -n "${ADMIN_PASSWORD:-}" ]; then
        if [ "${#ADMIN_PASSWORD}" -lt 12 ]; then
            echo "[entrypoint] ERROR: ADMIN_PASSWORD is ${#ADMIN_PASSWORD} characters; at least 12 are required." >&2
            exit 1
        fi
        php "$APP_DIR/bin/admin" admin:ensure
        admin_login_configured=1
    elif [ -n "${ADMIN_USERNAME:-}" ] || [ -n "${ADMIN_PASSWORD:-}" ]; then
        echo "[entrypoint] ERROR: set BOTH ADMIN_USERNAME and ADMIN_PASSWORD, or neither." >&2
        exit 1
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
