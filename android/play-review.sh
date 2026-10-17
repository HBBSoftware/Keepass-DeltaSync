#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Access for Google Play's reviewers. The app cannot be set up without a
# server, an enrolment token and a .kdbx file, so Play Console's "App access"
# has to hand all three over. Enrolment tokens are single-use and Google
# reviews most updates, so this runs before every submission:
#
#   ./play-review.sh demo-kdbx      # once: build website/review/deltasync-demo.kdbx
#   ./play-review.sh token          # before each submission
#
# `token` deletes the reviewer account (with every device and database it
# collected during the last review) and creates it again, so each review
# starts from an empty account. It prints the text to paste into App access.
#
# Environment:
#   KEEPASS_DELTASYNC_ADMIN_TOKEN  admin token; else read from ADMIN_TOKEN_FILE
#   ADMIN_TOKEN_FILE               default ~/.Deploy/deltasync_review_admin_token.txt
#   SERVER                         default https://review.deltasync.org, a separate
#                                  instance with its own database, so reviewers
#                                  never touch real data. Its tokens live 336 h.
#   REVIEW_USER                    default play-review
#   DELTASYNC_CLI                  CLI binary; default `go run` from client/
set -eu
cd "$(dirname "$0")"

SERVER="${SERVER:-https://review.deltasync.org}"
REVIEW_USER="${REVIEW_USER:-play-review}"
ADMIN_TOKEN_FILE="${ADMIN_TOKEN_FILE:-$HOME/.Deploy/deltasync_review_admin_token.txt}"
DEMO_KDBX=../website/review/deltasync-demo.kdbx
# The demo file is part of the website, not of the review server.
DEMO_URL="https://deltasync.org/review/deltasync-demo.kdbx"
# Not a secret: the file holds made-up entries and is public on the website.
DEMO_PASSWORD="deltasync-review"

die() { echo "error: $*" >&2; exit 1; }

cli() {
    if [ -n "${DELTASYNC_CLI:-}" ]; then
        "$DELTASYNC_CLI" "$@"
    else
        (cd ../client && go run ./cmd/keepass-deltasync "$@")
    fi
}

demo_kdbx() {
    command -v keepassxc-cli >/dev/null || die "keepassxc-cli not found"
    [ -e "$DEMO_KDBX" ] && die "$DEMO_KDBX exists; delete it first to rebuild"
    mkdir -p "$(dirname "$DEMO_KDBX")"
    printf '%s\n%s\n' "$DEMO_PASSWORD" "$DEMO_PASSWORD" \
        | keepassxc-cli db-create -q -p "$DEMO_KDBX"
    kx() { printf '%s\n' "$DEMO_PASSWORD" | keepassxc-cli "$@" -q >/dev/null; }
    kx mkdir "$DEMO_KDBX" Work
    kx add -u alice@example.com --url https://mail.example.com -g "$DEMO_KDBX" "Example mail"
    kx add -u alice --url https://shop.example.com -g "$DEMO_KDBX" "Example shop"
    kx add -u a.smith --url https://intranet.example.com -g "$DEMO_KDBX" "Work/Intranet"
    echo "wrote $DEMO_KDBX (password: $DEMO_PASSWORD)"
    echo "commit it and push; publish:website puts it on $DEMO_URL"
}

token() {
    if [ -z "${KEEPASS_DELTASYNC_ADMIN_TOKEN:-}" ]; then
        [ -r "$ADMIN_TOKEN_FILE" ] || die "no admin token: set KEEPASS_DELTASYNC_ADMIN_TOKEN or create $ADMIN_TOKEN_FILE"
        KEEPASS_DELTASYNC_ADMIN_TOKEN="$(tr -d '[:space:]' < "$ADMIN_TOKEN_FILE")"
    fi
    export KEEPASS_DELTASYNC_ADMIN_TOKEN

    # user-list and user-delete take the server from config.toml, not a flag.
    # Point them at a throwaway config so they cannot reach whatever server
    # this machine happens to be enrolled with.
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT
    printf '[server]\n  url = "%s"\n' "$SERVER" > "$tmp/config.toml"
    export KEEPASS_DELTASYNC_CONFIG="$tmp/config.toml"

    if cli admin user-list | awk '{print $1}' | grep -qx "$REVIEW_USER"; then
        cli admin user-delete "$REVIEW_USER" --yes >/dev/null
    fi
    out="$(cli admin user-create "$REVIEW_USER" --display-name "Google Play review" --server "$SERVER")"
    enroll_token="$(printf '%s\n' "$out" | awk '/enroll --server /{print $NF}')"
    expires="$(printf '%s\n' "$out" | sed -n 's/^Enrollment-token (expires \(.*\)):$/\1/p')"
    [ -n "$enroll_token" ] || die "could not read the enrolment token from:\n$out"

    cat <<EOF
Token expires $expires (ENROLLMENT_TOKEN_TTL_HOURS on the server).
Paste into Play Console > App content > App access:
------------------------------------------------------------------------
DeltaSync syncs a KeePass file through the user's own server, so it needs
a server account and a .kdbx file before it does anything.

1. Download the demo database in the phone's browser:
   $DEMO_URL
   Its master password is: $DEMO_PASSWORD
2. Open DeltaSync, tap "Enroll device", fill in the two fields and tap
   "Enroll":
   Server URL: $SERVER
   Enrollment token: $enroll_token
3. Tap "Set up database". Under "Choose file…" pick the downloaded
   deltasync-demo.kdbx. Tap "List databases on server", then "Create a
   database on the server…", name it "demo" and tap "Create". Select
   "demo" and tap "Save configuration".
4. Tap "Sync now" and enter the master password above. The result dialog
   shows the entries that were sent.

The token works once. If it has been used, contact the developer for a new
one.
------------------------------------------------------------------------
EOF
}

case "${1:-}" in
    demo-kdbx) demo_kdbx ;;
    token) token ;;
    *) die "usage: $0 demo-kdbx | token" ;;
esac
