#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Laver de PNG'er Chrome og Edge kraever, ud af icon.svg.
#
# Hvorfor ikke bare bruge SVG'en: Firefox tager en SVG som ikon, Chromium goer
# ikke. Manifestet skal pege paa rasterbilleder, og butikken afviser pakken
# hvis de mangler.
#
# Rasteriseringen bruger den browser der i forvejen skal til for at koere
# udvidelsen. Enhver Chromium duer, og den er per definition installeret paa en
# maskine hvor nogen arbejder paa den her pakke. Alternativerne -- rsvg-convert,
# Inkscape, ImageMagick, cairosvg -- kan vi ikke regne med paa alle tre
# platforme, og de giver ikke samme resultat paa tvaers af deres versioner.
#
# Der gengives ÉN gang i 1024 og skaleres ned derfra. En direkte gengivelse i
# 16 pixel er en anelse skarpere og en anelse mere urolig; nedskaleringen giver
# et roligere ikon, og alle stoerrelser kommer saa fra samme gengivelse.
#
#   python3 make-icons.py            # skriver icons/ og store/
#   python3 make-icons.py --check    # fejler hvis de ligger forskelligt
#   KP_CHROMIUM=/sti/til/browser python3 make-icons.py
#
# --check sammenligner bytes og forudsaetter derfor samme browser- og
# Pillow-version. En anden version kan kode det samme billede en anelse
# anderledes; det er en forskel i gengivelsen, ikke i maerket.
#
# Kraever Pillow:  pip install pillow

import argparse
import hashlib
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile

try:
    from PIL import Image
except ImportError:
    sys.exit("make-icons.py needs Pillow: pip install pillow")

HERE = pathlib.Path(__file__).resolve().parent
SVG = HERE / "icon.svg"

# Udvidelsens egne ikoner, som manifestet peger paa.
ICON_DIR = HERE / "icons"
ICON_SIZES = (16, 32, 48, 128)

# Butikkernes logo er ikke en del af pakken. Edge Add-ons vil have 300x300;
# Chrome Web Store bruger de 128 der allerede ligger i manifestet.
STORE_DIR = HERE / "store"
STORE_SIZES = (300,)

MASTER = 1024

# Steder en Chromium plejer at ligge. Raekkefoelgen er ligegyldig: de gengiver
# den samme SVG ens.
CANDIDATES = {
    "win32": [
        r"C:\Program Files\Google\Chrome\Application\chrome.exe",
        r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
        r"C:\Program Files\Microsoft\Edge\Application\msedge.exe",
        r"C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe",
    ],
    "darwin": [
        "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
        "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
        "/Applications/Chromium.app/Contents/MacOS/Chromium",
        "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
    ],
}
ON_PATH = [
    "chromium",
    "chromium-browser",
    "google-chrome",
    "google-chrome-stable",
    "microsoft-edge",
    "brave-browser",
]


def find_browser():
    env = os.environ.get("KP_CHROMIUM")
    if env:
        if pathlib.Path(env).is_file():
            return env
        sys.exit(f"KP_CHROMIUM points at {env}, which is not a file")
    for path in CANDIDATES.get(sys.platform, []):
        if pathlib.Path(path).is_file():
            return path
    for name in ON_PATH:
        found = shutil.which(name)
        if found:
            return found
    sys.exit(
        "no Chromium found to render the SVG with. Install one, or point at it:\n"
        "  KP_CHROMIUM=/path/to/chrome python3 make-icons.py"
    )


def render_master(browser, work):
    """Gengiver icon.svg i MASTER pixel og returnerer billedet."""
    svg = SVG.read_text(encoding="utf-8")
    if 'width="256" height="256"' not in svg:
        sys.exit("icon.svg no longer declares width=256 height=256 — check the file before scaling it")
    svg = svg.replace('width="256" height="256"', f'width="{MASTER}" height="{MASTER}"', 1)

    page = work / "icon.html"
    page.write_text(
        "<!doctype html><meta charset=utf-8>"
        "<style>html,body{margin:0;padding:0;background:transparent}svg{display:block}</style>" + svg,
        encoding="utf-8",
    )
    shot = work / "master.png"

    subprocess.run(
        [
            browser,
            "--headless",
            "--disable-gpu",
            "--hide-scrollbars",
            # Uden denne bliver de afrundede hjoerner hvide i stedet for
            # gennemsigtige.
            "--default-background-color=00000000",
            # Egen profilmappe: uden den kan gengivelsen fejle mens browseren
            # koerer i forvejen, hvilket den typisk goer paa den maskine hvor
            # nogen sidder og arbejder paa ikonet.
            f"--user-data-dir={work / 'profile'}",
            # CI koerer som root i en container, og dér naegter sandkassen at
            # starte. Filen der gengives er vores egen.
            "--no-sandbox",
            f"--window-size={MASTER},{MASTER}",
            f"--screenshot={shot}",
            page.as_uri(),
        ],
        check=True,
        capture_output=True,
    )

    if not shot.is_file():
        sys.exit("the browser exited without writing a screenshot")
    master = Image.open(shot).convert("RGBA")
    if master.size != (MASTER, MASTER):
        sys.exit(f"expected a {MASTER}x{MASTER} render, got {master.size}")
    return master


def write(master, path, size, check, stale):
    icon = master.resize((size, size), Image.Resampling.LANCZOS)
    # optimize=True holder filerne smaa; butikkerne vejer pakken.
    tmp = path.with_suffix(".tmp")
    icon.save(tmp, "PNG", optimize=True)
    fresh = tmp.read_bytes()
    if check:
        tmp.unlink()
        current = path.read_bytes() if path.exists() else b""
        if hashlib.sha256(current).digest() != hashlib.sha256(fresh).digest():
            stale.append(path.name)
        return
    tmp.replace(path)
    print(f"wrote {path.relative_to(HERE)} ({len(fresh)} bytes)")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="verify the PNGs on disk match")
    args = parser.parse_args()

    browser = find_browser()
    with tempfile.TemporaryDirectory() as tmpdir:
        master = render_master(browser, pathlib.Path(tmpdir))

    if not args.check:
        print(f"rendered {SVG.name} with {browser}")

    ICON_DIR.mkdir(exist_ok=True)
    STORE_DIR.mkdir(exist_ok=True)

    stale = []
    for size in ICON_SIZES:
        write(master, ICON_DIR / f"icon-{size}.png", size, args.check, stale)
    for size in STORE_SIZES:
        write(master, STORE_DIR / f"logo-{size}.png", size, args.check, stale)

    if args.check:
        if stale:
            sys.exit("out of date, run make-icons.py: " + ", ".join(stale))
        print("icons are up to date")


if __name__ == "__main__":
    main()
