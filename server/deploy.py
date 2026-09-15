#!/usr/bin/env python3
"""Upload the PHP server to a file-based host over SFTP.

Recipe 1 in docs/deployment.md — the code in ~/server/, the web root at
~/server/public/ — has no shell, so the pieces docker-entrypoint.sh normally
takes care of do not happen here:

  * schema/*.sql is NOT applied. The files land on disk; applying them is a
    separate, deliberate step. Run any new migration BEFORE uploading the code
    that needs it, or the app will answer 500 for the gap in between.
  * ADMIN_USERNAME / ADMIN_PASSWORD belong in the host's own .env, which this
    script never touches. The request path reads them: with no account in the
    database, the first login attempt creates it from those two, and changing
    the password there is what rotates it. The entrypoint's admin:ensure is
    the container's equivalent and does not run here.

The web root is shared with the marketing site, which is deployed separately
from website/. That is why this script never mirrors and never deletes: a
mirror would erase the site. It also refuses to touch .env, which holds this
host's own database credentials and is not ours to overwrite.

    DEPLOY_SFTP_HOST      www21.your-server.de
    DEPLOY_SFTP_USER      the SFTP account
    DEPLOY_SFTP_PASSWORD  its password
    DEPLOY_SFTP_PATH      remote directory, defaults to "" (the account root)
    DEPLOY_SFTP_HOSTKEY   expected SHA256 host-key fingerprint

    python3 server/deploy.py [--dry-run]
"""

import base64
import hashlib
import os
import pathlib
import posixpath
import sys

import paramiko

HOST = os.environ.get("DEPLOY_SFTP_HOST", "www21.your-server.de")
USER = os.environ.get("DEPLOY_SFTP_USER", "")
REMOTE_ROOT = os.environ.get("DEPLOY_SFTP_PATH", "")


def _password() -> str:
    """DEPLOY_SFTP_PASSWORD, or the contents of DEPLOY_SFTP_PASSWORD_FILE.

    The file form keeps the secret off the command line, where it would other-
    wise be visible to every process on the machine and land in shell history.
    CI has masked variables and uses the plain one.
    """
    path = os.environ.get("DEPLOY_SFTP_PASSWORD_FILE", "")
    if path:
        return pathlib.Path(path).read_text().strip()
    return os.environ.get("DEPLOY_SFTP_PASSWORD", "")


PASSWORD = _password()

EXPECTED_HOSTKEY = os.environ.get(
    "DEPLOY_SFTP_HOSTKEY", "SHA256:35q0bJF/pJdZEtQLUm7uZp9LWSgWhhtEAAb0JLioLYU"
)

LOCAL_ROOT = pathlib.Path(__file__).resolve().parent

# .env belongs to the host and carries its database password — never send ours.
# The container files describe a deployment this host is not. tests/ is not
# something a web root should be able to serve.
# setup.php is the first-run wizard, and the app's own page tells you to delete
# it once setup is done. An update must therefore not put it back: doing so
# would silently restore attack surface on every deploy, and the wizard is
# older than the panel login anyway — it creates an admin token, never the
# admin account. A genuinely new install uploads it by hand, once.
EXCLUDE_NAMES = {".env", ".gitignore", ".dockerignore", "Dockerfile",
                 "docker-entrypoint.sh", "deploy.py", "setup.php"}
EXCLUDE_DIRS = {"tests", "vendor"}


def fingerprint(key) -> str:
    return "SHA256:" + base64.b64encode(hashlib.sha256(key.asbytes()).digest()).decode().rstrip("=")


def local_files():
    for path in sorted(LOCAL_ROOT.rglob("*")):
        if path.is_dir():
            continue
        rel = path.relative_to(LOCAL_ROOT)
        if path.name in EXCLUDE_NAMES or set(rel.parts[:-1]) & EXCLUDE_DIRS:
            continue
        yield path, rel.as_posix()


def remote_path(rel: str) -> str:
    return posixpath.join(REMOTE_ROOT, rel) if REMOTE_ROOT else rel


def ensure_dir(sftp, remote_dir):
    parts, walked = [p for p in remote_dir.split("/") if p], ""
    for part in parts:
        walked = posixpath.join(walked, part) if walked else part
        try:
            sftp.stat(walked)
        except FileNotFoundError:
            sftp.mkdir(walked)


def main() -> int:
    dry_run = "--dry-run" in sys.argv
    files = list(local_files())

    # A bug in the exclusion list would overwrite the host's credentials with a
    # developer's. Cheap to assert, expensive to discover afterwards.
    assert not any(rel.endswith(".env") for _, rel in files), "refusing to upload .env"

    print(f"{len(files)} files in {LOCAL_ROOT}")
    if dry_run:
        for _, rel in files:
            print("  would upload", remote_path(rel))
        return 0
    if not (USER and PASSWORD):
        print("DEPLOY_SFTP_USER and DEPLOY_SFTP_PASSWORD must be set", file=sys.stderr)
        return 2

    transport = paramiko.Transport((HOST, 22))
    transport.connect(username=USER, password=PASSWORD)
    got = fingerprint(transport.get_remote_server_key())
    if got != EXPECTED_HOSTKEY:
        transport.close()
        print(f"host key mismatch\n  expected {EXPECTED_HOSTKEY}\n  got      {got}", file=sys.stderr)
        return 1
    print(f"connected to {HOST}, host key {got}")

    sftp = paramiko.SFTPClient.from_transport(transport)
    changed = skipped = 0
    for path, rel in files:
        remote = remote_path(rel)
        payload = path.read_bytes()
        try:
            with sftp.open(remote) as handle:
                if handle.read() == payload:
                    skipped += 1
                    continue
        except FileNotFoundError:
            pass

        parent = posixpath.dirname(remote)
        if parent:
            ensure_dir(sftp, parent)

        staging = posixpath.join(parent, "." + posixpath.basename(remote) + ".upload")
        with sftp.open(staging, "wb") as handle:
            handle.write(payload)
        sftp.posix_rename(staging, remote)
        print(f"  uploaded {rel} ({len(payload)} bytes)")
        changed += 1

    sftp.close()
    transport.close()
    print(f"done: {changed} uploaded, {skipped} already current")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
