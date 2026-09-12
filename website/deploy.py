#!/usr/bin/env python3
"""Upload website/ to the live server over SFTP.

The host serves the website out of the SAME directory as the PHP app: the
pages here land next to admin.html, index.php and .htaccess, which come from
server/public/ instead. That is why this script never mirrors and never
deletes — it only writes the files it is given. A mirror would wipe the API.

Credentials come from the environment so nothing is committed:

    DEPLOY_SFTP_HOST      www21.your-server.de
    DEPLOY_SFTP_USER      the SFTP account
    DEPLOY_SFTP_PASSWORD  its password
    DEPLOY_SFTP_PATH      remote directory, defaults to "public"
    DEPLOY_SFTP_HOSTKEY   expected SHA256 host-key fingerprint

Run it from anywhere:  python3 website/deploy.py [--dry-run]
"""

import base64
import hashlib
import os
import pathlib
import posixpath
import sys

import paramiko

# The account is SFTP-only: exec_command is refused, so there is no way to run
# git or any other command on the far side. Everything happens over file I/O.
HOST = os.environ.get("DEPLOY_SFTP_HOST", "www21.your-server.de")
USER = os.environ.get("DEPLOY_SFTP_USER", "")
PASSWORD = os.environ.get("DEPLOY_SFTP_PASSWORD", "")
REMOTE_ROOT = os.environ.get("DEPLOY_SFTP_PATH", "public")

# Pinned so a redirected DNS name cannot quietly collect the password. Read off
# the live host on 2026-09-13; if the provider rekeys, verify out of band before
# changing it.
EXPECTED_HOSTKEY = os.environ.get(
    "DEPLOY_SFTP_HOSTKEY", "SHA256:35q0bJF/pJdZEtQLUm7uZp9LWSgWhhtEAAb0JLioLYU"
)

LOCAL_ROOT = pathlib.Path(__file__).resolve().parent

# Tooling, not content. deploy.py would be served at /deploy.py otherwise.
EXCLUDE = {"deploy.py", ".gitignore"}


def fingerprint(key) -> str:
    return "SHA256:" + base64.b64encode(hashlib.sha256(key.asbytes()).digest()).decode().rstrip("=")


def local_files():
    for path in sorted(LOCAL_ROOT.rglob("*")):
        if path.is_dir() or path.name in EXCLUDE:
            continue
        yield path, path.relative_to(LOCAL_ROOT).as_posix()


def ensure_dir(sftp, remote_dir):
    """mkdir -p, one segment at a time: SFTP has no recursive mkdir."""
    parts, walked = remote_dir.split("/"), ""
    for part in parts:
        walked = posixpath.join(walked, part) if walked else part
        try:
            sftp.stat(walked)
        except FileNotFoundError:
            sftp.mkdir(walked)


def main() -> int:
    dry_run = "--dry-run" in sys.argv
    if not dry_run and not (USER and PASSWORD):
        print("DEPLOY_SFTP_USER and DEPLOY_SFTP_PASSWORD must be set", file=sys.stderr)
        return 2

    files = list(local_files())
    print(f"{len(files)} files in {LOCAL_ROOT}")
    if dry_run:
        for _, rel in files:
            print("  would upload", posixpath.join(REMOTE_ROOT, rel))
        return 0

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.RejectPolicy())
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
        remote = posixpath.join(REMOTE_ROOT, rel)
        payload = path.read_bytes()

        # Re-uploading an identical file costs a write and a new mtime for no
        # reason, and mtime is what the cache headers hang off.
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

        # Write beside the target and rename over it, so a visitor can never be
        # served half a file.
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
