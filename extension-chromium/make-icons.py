#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Rasteriserer ../extension/icon.svg til de PNG'er Chrome og Edge kraever.
#
# Hvorfor ikke bare genbruge SVG'en: Firefox tager en SVG som ikon, Chromium
# goer ikke. Manifestet skal pege paa rasterbilleder, og butikken afviser
# pakken hvis de mangler.
#
# Hvorfor en tegning i kode frem for et konverteringsvaerktoej: rsvg-convert,
# Inkscape og ImageMagick er ikke installeret paa alle tre platforme vi bygger
# paa, og resultatet er ikke ens paa tvaers af deres versioner. Koordinaterne
# herunder er kopieret 1:1 fra icon.svg, som selv spejler Androids
# ic_launcher_foreground.xml. AEndrer maerket sig ét sted, skal alle foelges ad.
#
#   python3 make-icons.py            # skriver icons/icon-<size>.png
#   python3 make-icons.py --check    # fejler hvis de ligger forskelligt
#
# --check sammenligner bytes og forudsaetter derfor samme Pillow-version. En
# anden version kan kode det samme billede en anelse anderledes; det er en
# forskel i PNG-kodningen, ikke i maerket.
#
# Kraever Pillow:  pip install pillow

import argparse
import hashlib
import math
import pathlib
import sys

try:
    from PIL import Image, ImageChops, ImageDraw, ImageOps
except ImportError:
    sys.exit("make-icons.py needs Pillow: pip install pillow")

HERE = pathlib.Path(__file__).resolve().parent
OUT_DIR = HERE / "icons"

VIEWBOX = 256          # icon.svg's koordinatsystem
SUPERSAMPLE = 4        # tegn 4x op og skalér ned; Pillow kantudjaevner ikke selv
SIZES = (16, 32, 48, 128)

# Butikkernes logo er ikke en del af pakken. Edge Add-ons vil have 300x300,
# og Chrome Web Store bruger de 128 der allerede ligger i manifestet.
STORE_DIR = HERE / "store"
STORE_SIZES = (300,)

BG_FROM, BG_TO = "#1E3A8A", "#0F172A"
BOW_FROM, BOW_TO = "#FDE047", "#F59E0B"
ARC = "#38BDF8"
SHADOW = "#0F172A"
AMBER = "#F59E0B"

S = VIEWBOX * SUPERSAMPLE


def px(v):
    """SVG-koordinat -> pixel i det opskalerede lærred."""
    return v * SUPERSAMPLE


def vertical_ramp(y0, y1, color_from, color_to):
    """Fuldt lærred fyldt med en lodret gradient mellem to SVG-y-vaerdier.

    Uden for [y0, y1] fortsaetter endefarven, praecis som SVG's default
    spreadMethod="pad".
    """
    ramp = Image.new("L", (1, S))
    top, bottom = int(px(y0)), int(px(y1))
    span = max(bottom - top, 1)
    ramp.putdata([
        0 if y <= top else 255 if y >= bottom else int(round((y - top) * 255 / span))
        for y in range(S)
    ])
    return ImageOps.colorize(ramp.resize((S, S)), black=color_from, white=color_to)


def diagonal_ramp(color_from, color_to):
    """Gradienten fra (0,0) til (256,256): t = (x + y) / 2.

    Bygget som gennemsnittet af en lodret og en vandret rampe, saa der ikke
    skal loekkes over en million pixels i Python.
    """
    vertical = Image.linear_gradient("L").resize((S, S))
    horizontal = vertical.transpose(Image.Transpose.ROTATE_90)
    return ImageOps.colorize(
        ImageChops.add(vertical, horizontal, scale=2.0), black=color_from, white=color_to
    )


def arc_points(x0, y0, x1, y1, r, large_arc, sweep, steps=240):
    """SVG-buens endepunkts-form oversat til punkter paa buen.

    Formlerne er implementation notes F.6.5 og F.6.6 i SVG 1.1. Buerne i
    maerket er cirkulaere og uroterede, saa rotationsleddet er udeladt.
    """
    dx, dy = (x0 - x1) / 2.0, (y0 - y1) / 2.0
    # F.6.6: skalér radius op hvis den er for lille til at naa mellem punkterne.
    lam = (dx * dx + dy * dy) / (r * r)
    if lam > 1:
        r *= math.sqrt(lam)
    factor = (r * r - dx * dx - dy * dy) / (dx * dx + dy * dy)
    factor = math.sqrt(max(factor, 0.0))
    if large_arc == sweep:
        factor = -factor
    # F.6.5 med rx = ry = r, hvor radius-leddene forkorter vaek.
    cx = factor * dy + (x0 + x1) / 2.0
    cy = -factor * dx + (y0 + y1) / 2.0

    start = math.atan2(y0 - cy, x0 - cx)
    end = math.atan2(y1 - cy, x1 - cx)
    delta = end - start
    if sweep and delta < 0:
        delta += 2 * math.pi
    if not sweep and delta > 0:
        delta -= 2 * math.pi
    return [
        (cx + r * math.cos(start + delta * i / steps), cy + r * math.sin(start + delta * i / steps))
        for i in range(steps + 1)
    ]


def stroke_arc(draw, points, width, color):
    """Polylinje med runde hjoerner og runde ender, som stroke-linecap="round"."""
    draw.line([(px(x), px(y)) for x, y in points], fill=color, width=int(px(width)), joint="curve")
    cap = px(width) / 2.0
    for x, y in (points[0], points[-1]):
        draw.ellipse(
            [px(x) - cap, px(y) - cap, px(x) + cap, px(y) + cap], fill=color
        )


def masked(canvas, paint, shape):
    """Maler `paint` gennem en maske tegnet af `shape` ned paa `canvas`."""
    mask = Image.new("L", (S, S), 0)
    shape(ImageDraw.Draw(mask))
    canvas.paste(paint, (0, 0), mask)


def render():
    canvas = Image.new("RGB", (S, S), BG_TO)

    # Baggrund: afrundet flade med diagonal gradient.
    masked(
        canvas,
        diagonal_ramp(BG_FROM, BG_TO),
        lambda d: d.rounded_rectangle([0, 0, S - 1, S - 1], radius=px(56), fill=255),
    )

    draw = ImageDraw.Draw(canvas)

    # Sync-buerne.
    stroke_arc(draw, arc_points(44, 128, 196, 80, 84, 0, 1), 14, ARC)
    stroke_arc(draw, arc_points(212, 128, 60, 176, 84, 0, 1), 14, ARC)

    # Delta-formede pilespidser.
    for tri in ([(196, 60), (220, 86), (184, 98)], [(60, 196), (36, 170), (72, 158)]):
        draw.polygon([(px(x), px(y)) for x, y in tri], fill=ARC)

    # Noeglens greb og skaft, begge med den samme lodrette gradient som SVG'en
    # giver dem hver for sig over deres eget interval.
    masked(
        canvas,
        vertical_ramp(98, 158, BOW_FROM, BOW_TO),
        lambda d: d.ellipse([px(70), px(98), px(130), px(158)], fill=255),
    )
    masked(
        canvas,
        vertical_ramp(119, 137, BOW_FROM, BOW_TO),
        lambda d: d.rectangle([px(124), px(119), px(188), px(137)], fill=255),
    )

    # Noeglens tænder, og hullet i grebet.
    draw.rectangle([px(156), px(137), px(166), px(153)], fill=AMBER)
    draw.rectangle([px(174), px(137), px(182), px(149)], fill=AMBER)
    draw.ellipse([px(88), px(116), px(112), px(140)], fill=SHADOW)

    return canvas


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="verify the PNGs on disk match")
    args = parser.parse_args()

    master = render()
    OUT_DIR.mkdir(exist_ok=True)

    stale = []
    for size in SIZES:
        path = OUT_DIR / f"icon-{size}.png"
        icon = master.resize((size, size), Image.Resampling.LANCZOS)
        # optimize=True holder filerne smaa; butikkerne vejer pakken.
        tmp = path.with_suffix(".tmp")
        icon.save(tmp, "PNG", optimize=True)
        fresh = tmp.read_bytes()
        if args.check:
            tmp.unlink()
            current = path.read_bytes() if path.exists() else b""
            if hashlib.sha256(current).digest() != hashlib.sha256(fresh).digest():
                stale.append(path.name)
            continue
        tmp.replace(path)
        print(f"wrote {path.relative_to(HERE)} ({len(fresh)} bytes)")

    STORE_DIR.mkdir(exist_ok=True)
    for size in STORE_SIZES:
        path = STORE_DIR / f"logo-{size}.png"
        icon = master.resize((size, size), Image.Resampling.LANCZOS)
        tmp = path.with_suffix(".tmp")
        icon.save(tmp, "PNG", optimize=True)
        fresh = tmp.read_bytes()
        if args.check:
            tmp.unlink()
            current = path.read_bytes() if path.exists() else b""
            if hashlib.sha256(current).digest() != hashlib.sha256(fresh).digest():
                stale.append(path.name)
            continue
        tmp.replace(path)
        print(f"wrote {path.relative_to(HERE)} ({len(fresh)} bytes)")

    if args.check:
        if stale:
            sys.exit("out of date, run make-icons.py: " + ", ".join(stale))
        print("icons are up to date")


if __name__ == "__main__":
    main()
