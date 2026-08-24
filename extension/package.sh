#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Pakker extension/ til en USIGNERET XPI i dist/.
#
# Signering sker ikke her. Samme model som android/publish-release.sh: CI
# producerer artefaktet, og signeringsnøglen — hos AMO eller lokalt — kommer
# aldrig i nærheden af pipelinen. Se VERSIONING.md.
#
#   ./package.sh              # version tages fra manifest.json
#   ./package.sh 0.1.0        # og krydstjekkes mod manifest.json
#
# Kør fra extension/ eller fra repo-roden; scriptet finder selv sin egen mappe.

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
EXPECTED_VERSION="${1:-}"

# python3 hedder bare python under Git Bash på Windows.
PYTHON=$(command -v python3 || command -v python || true)
if [ -z "$PYTHON" ]; then
    echo "package.sh needs python3 (or python) on PATH" >&2
    exit 1
fi

"$PYTHON" - "$SCRIPT_DIR" "$REPO_ROOT" "$EXPECTED_VERSION" <<'PYTHON'
import hashlib
import json
import os
import sys
import zipfile

script_dir, repo_root, expected_version = sys.argv[1], sys.argv[2], sys.argv[3]

# Eksplicit liste frem for en glob: XPI'en bliver publiceret, og en glob ville
# før eller siden komme til at pakke en efterladt fil med ud i verden.
FILES = [
    "manifest.json",
    "background.js",
    "search.js",
    "popup.html",
    "popup.css",
    "popup.js",
    "icon.svg",
]

with open(os.path.join(script_dir, "manifest.json"), encoding="utf-8") as fh:
    manifest = json.load(fh)
version = manifest["version"]

if expected_version and expected_version != version:
    sys.exit(f"tag says {expected_version} but manifest.json says {version}")

missing = [f for f in FILES if not os.path.isfile(os.path.join(script_dir, f))]
if missing:
    sys.exit("missing files: " + ", ".join(missing))

dist = os.path.join(repo_root, "dist")
os.makedirs(dist, exist_ok=True)
xpi_path = os.path.join(dist, f"keepass-deltasync-extension-{version}.xpi")

# Fast tidsstempel og faste rettigheder: to builds af samme commit skal give
# byte-identiske filer, ellers kan ingen efterprøve at den XPI de har hentet
# svarer til kilden. 1980-01-01 er nulpunktet i zip-formatet.
with zipfile.ZipFile(xpi_path, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as xpi:
    for name in FILES:
        info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
        info.compress_type = zipfile.ZIP_DEFLATED
        info.external_attr = 0o644 << 16
        # create_system defaults to the host: 0 on Windows, 3 elsewhere. It is
        # one byte per entry in the central directory, and it was enough to
        # make a Windows build differ from CI's while every file inside was
        # identical. Pin it, or "byte-reproducible" only holds per platform.
        info.create_system = 3
        with open(os.path.join(script_dir, name), "rb") as fh:
            xpi.writestr(info, fh.read())

with open(xpi_path, "rb") as fh:
    digest = hashlib.sha256(fh.read()).hexdigest()

sums_path = os.path.join(dist, "SHA256SUMS-extension")
with open(sums_path, "w", encoding="utf-8", newline="\n") as fh:
    fh.write(f"{digest}  {os.path.basename(xpi_path)}\n")

size = os.path.getsize(xpi_path)
print(f"built {os.path.relpath(xpi_path, repo_root)} ({size} bytes, {len(FILES)} files)")
print(f"sha256 {digest}")
PYTHON
