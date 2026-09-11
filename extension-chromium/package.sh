#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Bygger Chrome/Edge-pakken i dist/ og et udpakket træ i build/unpacked/.
#
# Udvidelsen har ingen egen kopi af logikken. popup, søgning og baggrundskode
# hentes fra ../extension og lægges i pakken uændret, på nær de ganske få
# steder der siger "Firefox" til brugeren. De steder står i SUBSTITUTIONS
# herunder, og scriptet fejler hvis en af dem ikke længere findes ordret —
# ellers ville en rettelse i Firefox-udgaven stille og roligt sende en
# Chrome-bruger hen til en Firefox-vejledning.
#
#   ./package.sh              # version tages fra manifest.json
#   ./package.sh 0.1.0        # og krydstjekkes mod manifest.json
#
# build/unpacked/ er dét man peger chrome://extensions -> Load unpacked på.
#
# Kør fra extension-chromium/ eller fra repo-roden; scriptet finder selv sin
# egen mappe.

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
import shutil
import sys
import zipfile

script_dir, repo_root, expected_version = sys.argv[1], sys.argv[2], sys.argv[3]
shared_dir = os.path.join(repo_root, "extension")

# Filer der hører til denne pakke alene.
LOCAL = [
    "manifest.json",
    "compat.js",
    "sw.js",
    "icons/icon-16.png",
    "icons/icon-32.png",
    "icons/icon-48.png",
    "icons/icon-128.png",
]

# Filer der kommer ordret fra Firefox-udvidelsen. Eksplicit liste frem for en
# glob: pakken bliver publiceret, og en glob ville før eller siden pakke en
# efterladt fil med ud i verden.
SHARED = [
    "background.js",
    "search.js",
    "popup.html",
    "popup.css",
    "popup.js",
]

# De eneste steder hvor den delte kode ikke passer på en Chromium-browser.
# Nøglen er filen, værdien er (før, efter) — begge ordret, så en drift i
# Firefox-udgaven stopper bygget i stedet for at slippe forbi.
SUBSTITUTIONS = {
    # popup.html indlæser popup.js direkte. Skallen skal ligge før, ellers
    # findes `browser` ikke når popup'en begynder at arbejde.
    "popup.html": [
        (
            '    <script src="search.js"></script>',
            '    <script src="compat.js"></script>\n    <script src="search.js"></script>',
        ),
    ],
    "popup.js": [
        # Vejledningen ligger i extension-chromium/README.md indtil der findes
        # en side på hjemmesiden for de to Chromium-browsere. firefox.html
        # forklarer det samme, men ingen Chrome-bruger skal sendes derhen.
        (
            'const SETUP_URL = "https://deltasync.bjoerck-braun.dk/firefox.html";\n'
            'const SETUP_HOST_URL = SETUP_URL + "#host";\n'
            'const SETUP_DATABASE_URL = SETUP_URL + "#standalone";',
            'const SETUP_URL = "https://gitlab.com/Star95/keepass-deltasync/-/blob/main/extension-chromium/README.md";\n'
            'const SETUP_HOST_URL = SETUP_URL + "#set-up-the-native-host";\n'
            'const SETUP_DATABASE_URL = SETUP_URL + "#add-a-local-database";',
        ),
        (
            '      "Firefox cannot reach the keepass-deltasync host. It has to be installed and registered once, outside the browser.",',
            '      "This browser cannot reach the keepass-deltasync host. It has to be installed and registered once, outside the browser.",',
        ),
    ],
}

with open(os.path.join(script_dir, "manifest.json"), encoding="utf-8") as fh:
    manifest = json.load(fh)
version = manifest["version"]

if expected_version and expected_version != version:
    sys.exit(f"tag says {expected_version} but manifest.json says {version}")

missing = [f for f in LOCAL if not os.path.isfile(os.path.join(script_dir, f))]
missing += [f"../extension/{f}" for f in SHARED if not os.path.isfile(os.path.join(shared_dir, f))]
if missing:
    sys.exit("missing files: " + ", ".join(missing))

# Saml træet. build/ er bygge-output og hører ikke i versionsstyring.
stage = os.path.join(script_dir, "build", "unpacked")
if os.path.isdir(stage):
    shutil.rmtree(stage)
os.makedirs(stage)

payload = {}

for name in LOCAL:
    with open(os.path.join(script_dir, name), "rb") as fh:
        payload[name] = fh.read()

for name in SHARED:
    with open(os.path.join(shared_dir, name), encoding="utf-8") as fh:
        text = fh.read()
    for old, new in SUBSTITUTIONS.get(name, []):
        if text.count(old) != 1:
            sys.exit(
                f"{name}: expected exactly one occurrence of:\n  {old.splitlines()[0]}\n"
                f"found {text.count(old)}. The Firefox extension changed — update "
                f"SUBSTITUTIONS in extension-chromium/package.sh."
            )
        text = text.replace(old, new)
    payload[name] = text.encode("utf-8")

for name, blob in payload.items():
    target = os.path.join(stage, name)
    os.makedirs(os.path.dirname(target), exist_ok=True)
    with open(target, "wb") as fh:
        fh.write(blob)

dist = os.path.join(repo_root, "dist")
os.makedirs(dist, exist_ok=True)
zip_path = os.path.join(dist, f"keepass-deltasync-chromium-{version}.zip")

# Fast tidsstempel og faste rettigheder, af samme grund som i
# ../extension/package.sh: to builds af samme commit skal give byte-identiske
# filer, ellers kan ingen efterprøve at pakken svarer til kilden.
with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as pkg:
    for name in sorted(payload):
        info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
        info.compress_type = zipfile.ZIP_DEFLATED
        info.external_attr = 0o644 << 16
        info.create_system = 3
        pkg.writestr(info, payload[name])

with open(zip_path, "rb") as fh:
    digest = hashlib.sha256(fh.read()).hexdigest()

sums_path = os.path.join(dist, "SHA256SUMS-extension-chromium")
with open(sums_path, "w", encoding="utf-8", newline="\n") as fh:
    fh.write(f"{digest}  {os.path.basename(zip_path)}\n")

size = os.path.getsize(zip_path)
print(f"built {os.path.relpath(zip_path, repo_root)} ({size} bytes, {len(payload)} files)")
print(f"sha256 {digest}")
print(f"unpacked {os.path.relpath(stage, repo_root)}")
PYTHON
