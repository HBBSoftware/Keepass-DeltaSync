#!/usr/bin/env python3
"""Collect DeltaSync download counters and render a private dashboard page.

The three registries we publish to are not equally open about numbers:

    ghcr.io         total and per-version, but ONLY in the package page's HTML.
                    The packages API needs a read:packages token; scraping does
                    not, so scraping is what we do.
    Docker Hub      pull_count straight out of a public JSON API.
    GitLab registry nothing at all — its registry/repositories API has no field
                    for it. Not a gap we can fill, so it is simply absent here.

GitHub release assets are counted too: that is where the Android APKs live, and
they are the only figure that maps to a person rather than a machine.

History lives on the website itself, not in git. Each run fetches the previous
stats.json over HTTPS, appends today's row and writes the file back; the deploy
step uploads it. That keeps the series going without a push token, and makes the
live site the single source of truth — if it is lost, so is the history.

The output directory comes from STATS_PATH and is deliberately NOT committed:
the GitLab project is public, so a path in the repo would be a published path.
Set it as a masked CI variable.

    STATS_PATH=stats-<random>  python3 website/collect_stats.py
"""

import datetime
import html
import json
import os
import pathlib
import re
import sys
import urllib.error
import urllib.request

DOCKERHUB = "https://hub.docker.com/v2/repositories/hbbsoftware/deltasync-server/"
GITHUB_RELEASES = "https://api.github.com/repos/HBBSoftware/Keepass-DeltaSync/releases?per_page=100"
GHCR_PAGE = "https://github.com/orgs/HBBSoftware/packages/container/package/deltasync-server"

SITE = os.environ.get("STATS_SITE", "https://deltasync.org")
STATS_PATH = os.environ.get("STATS_PATH", "")

WEBSITE_ROOT = pathlib.Path(__file__).resolve().parent
TIMEOUT = 30


def fetch(url, accept=None):
    # GitHub serves a stripped page to unknown agents, and Docker Hub rate-limits
    # them harder, so identify as a browser rather than as urllib.
    req = urllib.request.Request(url, headers={
        "User-Agent": "Mozilla/5.0 (compatible; deltasync-stats/1.0)",
        **({"Accept": accept} if accept else {}),
    })
    with urllib.request.urlopen(req, timeout=TIMEOUT) as r:
        return r.read().decode("utf-8", "replace")


def dockerhub_pulls():
    return json.loads(fetch(DOCKERHUB, "application/json")).get("pull_count")


def github_release_downloads():
    data = json.loads(fetch(GITHUB_RELEASES, "application/vnd.github+json"))
    if not isinstance(data, list):
        raise ValueError(f"unexpected releases payload: {str(data)[:120]}")
    return sum(a.get("download_count", 0) for r in data for a in r.get("assets", []))


def ghcr_downloads():
    """Scrape the package page. Returns (total, {version: count})."""
    text = re.sub(r"\s+", " ", html.unescape(re.sub(r"<[^>]+>", " ", fetch(GHCR_PAGE))))
    # The label sits AFTER the number for per-version counts and BEFORE it for
    # the total. That asymmetry is GitHub's, not a typo.
    total = re.search(r"Total downloads ([\d,]+)", text)

    # A version card reads, flattened:
    #   latest 0.5.0 Published 21 days ago · Digest … sha256:<hex> 420 Version downloads
    # so the tag list comes BEFORE its own count and immediately before the NEXT
    # card's. Reading forward from the number therefore labels every count with
    # the following version — which is why we walk backwards to the nearest
    # "Published" instead, and pick the version-looking token ("latest" is a
    # second tag on the same card, not a version).
    per = {}
    for m in re.finditer(r"([\d,]+) Version downloads", text):
        head = text[:m.start()]
        cut = head.rfind("Published")
        if cut < 0:
            continue
        tags = [t for t in head[max(0, cut - 60):cut].split() if re.fullmatch(r"\d[\w.+-]*", t)]
        if tags:
            per[tags[-1]] = int(m.group(1).replace(",", ""))
    return (int(total.group(1).replace(",", "")) if total else None), per


def previous_history():
    """Read the series already published. A missing file is a fresh start."""
    url = f"{SITE}/{STATS_PATH}/stats.json"
    try:
        data = json.loads(fetch(url, "application/json"))
    except (urllib.error.HTTPError, urllib.error.URLError, ValueError) as e:
        print(f"  no previous history at {url} ({e}) — starting a new series")
        return []
    # A 404 is answered by the PHP app as a JSON error object, not as HTML, so a
    # successful parse does not yet mean we got our own file.
    if not isinstance(data, dict) or "samples" not in data:
        print("  previous file was not a stats document — starting a new series")
        return []
    return data["samples"]


def main() -> int:
    if not STATS_PATH:
        print("STATS_PATH is not set — refusing to write to a guessable path", file=sys.stderr)
        return 2

    today = datetime.datetime.now(datetime.timezone.utc).date().isoformat()
    row = {"date": today}

    # One dead source must not cost us the whole day's sample, so each is tried
    # on its own and a failure is recorded as null rather than raised.
    for name, fn in (("dockerhub", dockerhub_pulls), ("github_releases", github_release_downloads)):
        try:
            row[name] = fn()
            print(f"  {name}: {row[name]}")
        except Exception as e:
            row[name] = None
            print(f"  {name}: FAILED ({e})")

    try:
        total, per_version = ghcr_downloads()
        row["ghcr"] = total
        row["ghcr_versions"] = per_version
        print(f"  ghcr: {total} (versions: {per_version})")
    except Exception as e:
        row["ghcr"] = None
        row["ghcr_versions"] = {}
        print(f"  ghcr: FAILED ({e})")

    samples = [s for s in previous_history() if s.get("date") != today]
    samples.append(row)
    samples.sort(key=lambda s: s["date"])

    out_dir = WEBSITE_ROOT / STATS_PATH
    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / "stats.json").write_text(
        json.dumps({"generated": today, "samples": samples}, indent=1) + "\n", encoding="utf-8")
    (out_dir / "index.html").write_text(PAGE, encoding="utf-8")
    print(f"wrote {len(samples)} samples to {out_dir}/stats.json")
    return 0


# Rewritten on every run so a change here ships with the next scheduled job.
# It links ../style.css so the page inherits the site's light/dark tokens, and
# carries noindex because the only thing keeping it private is that nobody links
# to it — robots.txt would have published the path instead of hiding it.
PAGE = """<!DOCTYPE html>
<html lang="da">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow, noarchive">
<title>DeltaSync — hentningstal</title>
<link rel="stylesheet" href="../style.css">
<style>
  /* Series identity. Both steps validated against the site's light (#ffffff)
     and dark (#0a0f1c) surfaces: lightness band, chroma, CVD separation and
     3:1 contrast all pass in both, so one palette serves both themes. */
  :root { --s-ghcr: #0284c7; --s-hub: #b45309; --s-rel: #7c3aed; }

  .wrap { max-width: 60rem; margin: 0 auto; padding: var(--space-8) var(--space-4); }
  .lede { color: var(--text-muted); max-width: 42rem; }

  .tiles { display: grid; gap: var(--space-4); grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
           margin: var(--space-8) 0; }
  .tile { background: var(--bg-elev); border: 1px solid var(--border);
          border-radius: var(--radius-md); padding: var(--space-4); }
  .tile h2 { font-size: 0.8rem; font-weight: 600; letter-spacing: 0.02em; margin: 0 0 var(--space-2);
             color: var(--text-muted); display: flex; align-items: center; gap: var(--space-2); }
  .dot { width: 9px; height: 9px; border-radius: 50%; flex: none; }
  .big { font-size: 2rem; font-weight: 650; line-height: 1.1; font-variant-numeric: tabular-nums; }
  .delta { font-size: 0.8rem; color: var(--text-muted); font-variant-numeric: tabular-nums; }

  .panel { margin: var(--space-8) 0; }
  .panel h2 { font-size: 1rem; margin: 0 0 var(--space-1);
              display: flex; align-items: center; gap: var(--space-2); }
  .panel p { margin: 0 0 var(--space-3); font-size: 0.85rem; color: var(--text-muted); }
  .chart { position: relative; }
  svg { display: block; width: 100%; height: auto; overflow: visible; }
  .grid line { stroke: var(--border); stroke-width: 1; }
  .axis { fill: var(--text-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
  .line { fill: none; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
  .end { stroke: var(--bg); stroke-width: 2; }
  .lbl { fill: var(--text); font-size: 11px; font-weight: 600; font-variant-numeric: tabular-nums; }
  .hair { stroke: var(--text-muted); stroke-width: 1; stroke-dasharray: 3 3; opacity: 0; }
  .hit { fill: transparent; }
  .tip { position: absolute; pointer-events: none; opacity: 0; transform: translate(-50%, -125%);
         background: var(--bg-elev); border: 1px solid var(--border); border-radius: var(--radius-sm);
         padding: var(--space-2) var(--space-3); font-size: 0.8rem; white-space: nowrap;
         box-shadow: var(--shadow-md); font-variant-numeric: tabular-nums; }
  .empty { color: var(--text-muted); font-style: italic; }

  table { border-collapse: collapse; width: 100%; font-size: 0.85rem;
          font-variant-numeric: tabular-nums; }
  th, td { text-align: right; padding: var(--space-2) var(--space-3);
           border-bottom: 1px solid var(--border); }
  th:first-child, td:first-child { text-align: left; }
  details { margin: var(--space-8) 0; }
  summary { cursor: pointer; color: var(--link); font-size: 0.9rem; }
  .note { font-size: 0.85rem; color: var(--text-muted); border-left: 3px solid var(--border);
          padding-left: var(--space-4); margin: var(--space-8) 0 0; }
</style>
</head>
<body>
<div class="wrap">
  <h1>Hentningstal</h1>
  <p class="lede">Privat side — ikke linket fra deltasync.org og udelukket fra s&oslash;gemaskiner.
     Opdateres dagligt af et planlagt CI-job. <span id="gen"></span></p>

  <div class="tiles" id="tiles"></div>
  <div id="panels"></div>

  <details>
    <summary>Vis tallene som tabel</summary>
    <div id="table"></div>
  </details>

  <p class="note">Et <em>pull</em> er ikke en bruger. En TrueNAS-installation henter
     mindst to images, hver opgradering henter igen, og din egen CI t&aelig;ller med.
     L&aelig;s kurvens <em>retning</em>, ikke dens absolutte niveau.
     GitLabs registry er ikke med: det offentligg&oslash;r ingen t&aelig;llere.
     Kun hentninger af udgivelsesfiler p&aring; GitHub svarer nogenlunde til mennesker.</p>
</div>

<script>
const SERIES = [
  { key: "ghcr",            name: "ghcr.io",          color: "var(--s-ghcr)",
    note: "Det registry TrueNAS-appen henter fra \\u2014 tallet der betyder mest." },
  { key: "dockerhub",       name: "Docker Hub",       color: "var(--s-hub)",
    note: "Spejling. H\\u00f8jere tal, fordi den har samlet CI-pulls siden 1. juli 2026." },
  { key: "github_releases", name: "GitHub-udgivelser", color: "var(--s-rel)",
    note: "Hentede filer, mest Android-APK\\u0027er. T\\u00e6ttest p\\u00e5 rigtige personer." },
];
const nf = new Intl.NumberFormat("da-DK");
const df = (s) => new Date(s + "T00:00:00Z").toLocaleDateString("da-DK", { day: "numeric", month: "short" });

fetch("stats.json", { cache: "no-cache" })
  .then((r) => r.json())
  .then(render)
  .catch((e) => {
    document.getElementById("panels").innerHTML =
      '<p class="empty">Kunne ikke l\\u00e6se stats.json: ' + e + "</p>";
  });

function render(doc) {
  const rows = doc.samples || [];
  document.getElementById("gen").textContent = rows.length
    ? "Senest opdateret " + df(doc.generated) : "";

  document.getElementById("tiles").innerHTML = SERIES.map((s) => {
    const vals = rows.filter((r) => r[s.key] != null);
    const last = vals.length ? vals[vals.length - 1][s.key] : null;
    const prev = vals.length > 1 ? vals[vals.length - 2][s.key] : null;
    const d = last != null && prev != null ? last - prev : null;
    return '<div class="tile"><h2><span class="dot" style="background:' + s.color + '"></span>' +
      s.name + '</h2><div class="big">' + (last == null ? "\\u2014" : nf.format(last)) +
      '</div><div class="delta">' +
      (d == null ? "afventer n\\u00e6ste m\\u00e5ling" : (d > 0 ? "+" : "") + nf.format(d) + " siden sidst") +
      "</div></div>";
  }).join("");

  // Small multiples, one panel per channel: the three counters differ by two
  // orders of magnitude, and a shared axis would flatten the smallest to a
  // straight line. A second y-axis is never the answer.
  document.getElementById("panels").innerHTML = SERIES.map((s) =>
    '<section class="panel"><h2><span class="dot" style="background:' + s.color + '"></span>' +
    s.name + "</h2><p>" + s.note + '</p><div class="chart" id="c-' + s.key + '"></div></section>'
  ).join("");
  SERIES.forEach((s) => drawChart(document.getElementById("c-" + s.key), rows, s));

  const cols = ["Dato"].concat(SERIES.map((s) => s.name));
  document.getElementById("table").innerHTML = "<table><thead><tr>" +
    cols.map((c) => "<th>" + c + "</th>").join("") + "</tr></thead><tbody>" +
    rows.slice().reverse().map((r) => "<tr><td>" + r.date + "</td>" +
      SERIES.map((s) => "<td>" + (r[s.key] == null ? "\\u2014" : nf.format(r[s.key])) + "</td>").join("") +
      "</tr>").join("") + "</tbody></table>";
}

function drawChart(host, rows, s) {
  const pts = rows.filter((r) => r[s.key] != null).map((r) => ({ date: r.date, v: r[s.key] }));
  if (pts.length < 2) {
    host.innerHTML = '<p class="empty">' + (pts.length ? "\\u00c9n m\\u00e5ling indtil nu \\u2014 " +
      nf.format(pts[0].v) + ". En kurve kr\\u00e6ver mindst to." : "Ingen m\\u00e5linger endnu.") + "</p>";
    return;
  }
  const W = 720, H = 200, L = 52, R = 46, T = 14, B = 28;
  const iw = W - L - R, ih = H - T - B;
  const lo = Math.min.apply(null, pts.map((p) => p.v));
  const hi = Math.max.apply(null, pts.map((p) => p.v));
  // A flat series would collapse to zero range and divide by zero; give it air.
  const pad = hi === lo ? Math.max(1, hi * 0.05) : (hi - lo) * 0.12;
  const y0 = lo - pad, y1 = hi + pad;
  const X = (i) => L + (pts.length === 1 ? iw / 2 : (i / (pts.length - 1)) * iw);
  const Y = (v) => T + ih - ((v - y0) / (y1 - y0)) * ih;

  // Label the real minimum, midpoint and maximum rather than the padded scale
  // ends: the padding exists to keep the line off the edges, and printing it
  // would put numbers on the axis that never occurred in the data.
  const ticks = [lo, (lo + hi) / 2, hi];
  let svg = '<svg viewBox="0 0 ' + W + " " + H + '" role="img" aria-label="' +
    s.name + ', udvikling over tid">';
  svg += '<g class="grid">' + ticks.map((t) =>
    '<line x1="' + L + '" y1="' + Y(t).toFixed(1) + '" x2="' + (W - R) + '" y2="' + Y(t).toFixed(1) + '"/>'
  ).join("") + "</g>";
  svg += '<g class="axis" text-anchor="end">' + ticks.map((t) =>
    '<text x="' + (L - 8) + '" y="' + (Y(t) + 4).toFixed(1) + '">' + nf.format(Math.round(t)) + "</text>"
  ).join("") + "</g>";
  svg += '<g class="axis"><text x="' + L + '" y="' + (H - 8) + '">' + df(pts[0].date) +
    '</text><text x="' + (W - R) + '" y="' + (H - 8) + '" text-anchor="end">' +
    df(pts[pts.length - 1].date) + "</text></g>";
  svg += '<path class="line" stroke="' + s.color + '" d="' +
    pts.map((p, i) => (i ? "L" : "M") + X(i).toFixed(1) + " " + Y(p.v).toFixed(1)).join(" ") + '"/>';
  const lastI = pts.length - 1;
  svg += '<circle class="end" cx="' + X(lastI).toFixed(1) + '" cy="' + Y(pts[lastI].v).toFixed(1) +
    '" r="5" fill="' + s.color + '"/>';
  svg += '<text class="lbl" x="' + (W - R + 8) + '" y="' + (Y(pts[lastI].v) + 4).toFixed(1) + '">' +
    nf.format(pts[lastI].v) + "</text>";
  svg += '<line class="hair" id="h-' + s.key + '" y1="' + T + '" y2="' + (T + ih) + '"/>';
  svg += '<rect class="hit" x="' + L + '" y="' + T + '" width="' + iw + '" height="' + ih + '"/>';
  svg += "</svg>";
  host.innerHTML = svg + '<div class="tip"></div>';

  const el = host.querySelector("svg"), hair = host.querySelector(".hair"), tip = host.querySelector(".tip");
  const hit = host.querySelector(".hit");
  hit.addEventListener("pointermove", (ev) => {
    const box = el.getBoundingClientRect();
    const sx = (ev.clientX - box.left) / box.width * W;
    let i = Math.round(((sx - L) / iw) * (pts.length - 1));
    i = Math.max(0, Math.min(pts.length - 1, i));
    hair.setAttribute("x1", X(i).toFixed(1));
    hair.setAttribute("x2", X(i).toFixed(1));
    hair.style.opacity = 1;
    tip.style.opacity = 1;
    tip.style.left = (X(i) / W * box.width) + "px";
    tip.style.top = (Y(pts[i].v) / H * box.height) + "px";
    tip.innerHTML = "<strong>" + nf.format(pts[i].v) + "</strong> \\u00b7 " + df(pts[i].date);
  });
  hit.addEventListener("pointerleave", () => { hair.style.opacity = 0; tip.style.opacity = 0; });
}
</script>
</body>
</html>
"""


if __name__ == "__main__":
    raise SystemExit(main())
