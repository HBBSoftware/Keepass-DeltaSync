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

# The two numbers that count people rather than fetches. A registry pull is a
# machine asking for bytes; these are stores reporting an extension that is
# still installed. Added 3 Oct 2026, after three days of data showed ghcr
# climbing at a flat ~3.2/hour around the clock while Docker Hub — the same
# image, mirrored — moved by exactly zero. That is automation, not users.
AMO = "https://addons.mozilla.org/api/v5/addons/addon/deltasync-keepass-search-go/"
EDGE = ("https://microsoftedge.microsoft.com/addons/getproductdetailsbycrxid/"
        "dpmaneajjlanhipmbgdpdnfljiigdnjo")

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


def amo_stats():
    """Firefox add-on: (average_daily_users, weekly_downloads).

    average_daily_users comes from telemetry and rounds down, so a handful of
    users can still read 0 — it is a floor, not a count.
    """
    d = json.loads(fetch(AMO, "application/json"))
    if "detail" in d:
        raise ValueError(d["detail"])
    return d.get("average_daily_users"), d.get("weekly_downloads")


def edge_installs():
    """Edge add-on: activeInstallCount — installs still present, not downloads."""
    return json.loads(fetch(EDGE, "application/json")).get("activeInstallCount")


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
    for name, fn in (("dockerhub", dockerhub_pulls),
                     ("github_releases", github_release_downloads),
                     ("edge_installs", edge_installs)):
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

    try:
        row["amo_users"], row["amo_weekly"] = amo_stats()
        print(f"  amo: {row['amo_users']} daily users, {row['amo_weekly']} weekly downloads")
    except Exception as e:
        row["amo_users"] = row["amo_weekly"] = None
        print(f"  amo: FAILED ({e})")

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
<title>DeltaSync — udbredelse</title>
<link rel="stylesheet" href="../style.css">
<style>
  /* Kategoriske trin 1-5, i fast raekkefoelge. Lyse og moerke trin er valgt
     hver for sig og begge koert gennem validatoren: lysheds-baand, farvestyrke,
     farveblindheds-adskillelse og kontrast. Lys tema advarer om kontrast under
     3:1 for tre af dem — derfor baerer hver serie baade en farveprik OG sit navn
     i tekst, har sin vaerdi skrevet ved kurvens ende, og findes i tabellen.
     Identitet hviler aldrig paa farve alene. */
  :root { --s1:#2a78d6; --s2:#eb6834; --s3:#1baf7a; --s4:#eda100; --s5:#e87ba4; }
  @media (prefers-color-scheme: dark) {
    :root { --s1:#3987e5; --s2:#d95926; --s3:#199e70; --s4:#c98500; --s5:#d55181; }
  }

  .wrap { max-width: 60rem; margin: 0 auto; padding: var(--space-8) var(--space-4); }
  .lede { color: var(--text-muted); max-width: 44rem; }

  h2.group { font-size: 1.1rem; margin: var(--space-12) 0 var(--space-2); }
  p.group-note { margin: 0 0 var(--space-6); color: var(--text-muted);
                 max-width: 44rem; font-size: 0.9rem; }

  .tiles { display: grid; gap: var(--space-4);
           grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
           margin: 0 0 var(--space-8); }
  .tile { background: var(--bg-elev); border: 1px solid var(--border);
          border-radius: var(--radius-md); padding: var(--space-4); }
  .tile h3 { font-size: 0.8rem; font-weight: 600; margin: 0 0 var(--space-2);
             color: var(--text-muted); display: flex; align-items: center;
             gap: var(--space-2); line-height: 1.3; }
  .dot { width: 9px; height: 9px; border-radius: 50%; flex: none; }
  .big { font-size: 2rem; font-weight: 650; line-height: 1.1;
         font-variant-numeric: tabular-nums; }
  .delta { font-size: 0.8rem; color: var(--text-muted);
           font-variant-numeric: tabular-nums; }

  .panel { margin: 0 0 var(--space-8); }
  .panel h3 { font-size: 0.95rem; margin: 0 0 var(--space-1);
              display: flex; align-items: center; gap: var(--space-2); }
  .panel p { margin: 0 0 var(--space-3); font-size: 0.85rem; color: var(--text-muted); }
  .chart { position: relative; }
  svg { display: block; width: 100%; height: auto; overflow: visible; }
  .grid line { stroke: var(--border); stroke-width: 1; }
  .axis { fill: var(--text-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
  .line { fill: none; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
  .end { stroke: var(--bg); stroke-width: 2; }
  .lbl { fill: var(--text); font-size: 11px; font-weight: 600;
         font-variant-numeric: tabular-nums; }
  .hair { stroke: var(--text-muted); stroke-width: 1; stroke-dasharray: 3 3; opacity: 0; }
  .hit { fill: transparent; }
  .tip { position: absolute; pointer-events: none; opacity: 0;
         transform: translate(-50%, -125%); background: var(--bg-elev);
         border: 1px solid var(--border); border-radius: var(--radius-sm);
         padding: var(--space-2) var(--space-3); font-size: 0.8rem;
         white-space: nowrap; box-shadow: var(--shadow-md);
         font-variant-numeric: tabular-nums; }
  .empty { color: var(--text-muted); font-style: italic; }

  table { border-collapse: collapse; width: 100%; font-size: 0.8rem;
          font-variant-numeric: tabular-nums; }
  th, td { text-align: right; padding: var(--space-2); border-bottom: 1px solid var(--border); }
  th:first-child, td:first-child { text-align: left; }
  details { margin: var(--space-8) 0; }
  summary { cursor: pointer; color: var(--link); font-size: 0.9rem; }
  .note { font-size: 0.85rem; color: var(--text-muted);
          border-left: 3px solid var(--border); padding-left: var(--space-4);
          margin: var(--space-8) 0 0; }
</style>
</head>
<body>
<div class="wrap">
  <h1>Udbredelse</h1>
  <p class="lede">Privat side — ikke linket fra deltasync.org og udelukket fra s&oslash;gemaskiner.
     Opdateres dagligt af et planlagt CI-job. <span id="gen"></span></p>

  <h2 class="group">Installationer</h2>
  <p class="group-note">De eneste tal der t&aelig;ller <em>mennesker</em>. En butik rapporterer
     en udvidelse der stadig er installeret; et registry rapporterer en maskine der bad om bytes.</p>
  <div class="tiles" id="tiles-people"></div>
  <div id="panels-people"></div>

  <h2 class="group">Hentninger fra registries</h2>
  <p class="group-note">Overvejende maskiner. M&aring;lt 30. sep &ndash; 2. okt 2026 voksede ghcr
     med 75 og 76 om dagen &mdash; og en m&aring;ling midt imellem viste <em>samme</em> takt om natten
     som midt p&aring; dagen. Mennesker har en d&oslash;gnrytme. Samtidig stod Docker Hub, som er
     n&oslash;jagtig samme image spejlet, helt stille. Forskellen er at TrueNAS-kataloget peger
     p&aring; ghcr, s&aring; alt der genneml&oslash;ber katalogets apps rammer kun den ene.</p>
  <div class="tiles" id="tiles-machines"></div>
  <div id="panels-machines"></div>

  <details>
    <summary>Vis alle tal som tabel</summary>
    <div id="table"></div>
  </details>

  <p class="note">GitLabs registry er ikke med: det offentligg&oslash;r ingen t&aelig;llere.
     F-Droid heller ikke &mdash; derfor er Android-brugere usynlige her, uanset hvor mange der er.
     AMO\u2019s brugertal kommer fra telemetri og runder ned, s&aring; en h&aring;ndfuld brugere
     kan stadig vise 0. L&aelig;s det som et gulv, ikke som en optælling.</p>
</div>

<script>
const PEOPLE = [
  { key: "amo_users", name: "Firefox \u2014 daglige brugere", color: "var(--s1)",
    note: "addons.mozilla.org. Gennemsnitligt antal installationer der sender telemetri." },
  { key: "edge_installs", name: "Edge \u2014 aktive installationer", color: "var(--s2)",
    note: "Microsofts butik. Installationer der stadig findes, ikke hentninger." },
  { key: "github_releases", name: "GitHub \u2014 hentede filer", color: "var(--s3)",
    note: "Udgivelsesfiler, mest Android-APK\u2019er. En hentning er en handling, ikke et kald." },
];
const MACHINES = [
  { key: "ghcr", name: "ghcr.io \u2014 pulls", color: "var(--s4)",
    note: "Det registry TrueNAS-appen henter fra. Vokser j\u00e6vnt d\u00f8gnet rundt." },
  { key: "dockerhub", name: "Docker Hub \u2014 pulls", color: "var(--s5)",
    note: "Samme image, spejlet. Kontrolgruppen: intet automatisk peger herp\u00e5." },
];
const ALL = PEOPLE.concat(MACHINES);
const nf = new Intl.NumberFormat("da-DK");
const df = (s) => new Date(s + "T00:00:00Z").toLocaleDateString("da-DK", { day: "numeric", month: "short" });

fetch("stats.json", { cache: "no-cache" })
  .then((r) => r.json())
  .then(render)
  .catch((e) => {
    document.getElementById("panels-people").innerHTML =
      '<p class="empty">Kunne ikke l\u00e6se stats.json: ' + e + "</p>";
  });

function tile(rows, s) {
  const vals = rows.filter((r) => r[s.key] != null);
  const last = vals.length ? vals[vals.length - 1][s.key] : null;
  const prev = vals.length > 1 ? vals[vals.length - 2][s.key] : null;
  const d = last != null && prev != null ? last - prev : null;
  let sub = d == null ? "afventer n\u00e6ste m\u00e5ling"
                      : (d > 0 ? "+" : "") + nf.format(d) + " siden sidst";
  if (s.key === "amo_users") {
    const w = vals.length ? vals[vals.length - 1].amo_weekly : null;
    if (w != null) sub += " \u00b7 " + nf.format(w) + "/uge hentet";
  }
  return '<div class="tile"><h3><span class="dot" style="background:' + s.color + '"></span>' +
    s.name + '</h3><div class="big">' + (last == null ? "\u2014" : nf.format(last)) +
    '</div><div class="delta">' + sub + "</div></div>";
}

function render(doc) {
  const rows = doc.samples || [];
  document.getElementById("gen").textContent = rows.length
    ? "Senest opdateret " + df(doc.generated) : "";

  for (const [group, tid, pid] of [[PEOPLE, "tiles-people", "panels-people"],
                                   [MACHINES, "tiles-machines", "panels-machines"]]) {
    document.getElementById(tid).innerHTML = group.map((s) => tile(rows, s)).join("");
    document.getElementById(pid).innerHTML = group.map((s) =>
      '<section class="panel"><h3><span class="dot" style="background:' + s.color + '"></span>' +
      s.name + "</h3><p>" + s.note + '</p><div class="chart" id="c-' + s.key + '"></div></section>'
    ).join("");
    group.forEach((s) => drawChart(document.getElementById("c-" + s.key), rows, s));
  }

  const cols = ["Dato"].concat(ALL.map((s) => s.name));
  document.getElementById("table").innerHTML = "<table><thead><tr>" +
    cols.map((c) => "<th>" + c + "</th>").join("") + "</tr></thead><tbody>" +
    rows.slice().reverse().map((r) => "<tr><td>" + r.date + "</td>" +
      ALL.map((s) => "<td>" + (r[s.key] == null ? "\u2014" : nf.format(r[s.key])) + "</td>").join("") +
      "</tr>").join("") + "</tbody></table>";
}

function drawChart(host, rows, s) {
  const pts = rows.filter((r) => r[s.key] != null).map((r) => ({ date: r.date, v: r[s.key] }));
  if (pts.length < 2) {
    host.innerHTML = '<p class="empty">' + (pts.length ? "\u00c9n m\u00e5ling indtil nu \u2014 " +
      nf.format(pts[0].v) + ". En kurve kr\u00e6ver mindst to." : "Ingen m\u00e5linger endnu.") + "</p>";
    return;
  }
  const lo = Math.min.apply(null, pts.map((p) => p.v));
  const hi = Math.max.apply(null, pts.map((p) => p.v));
  // En serie der ikke har rykket sig baerer én oplysning, og en graf er den
  // forkerte form til én oplysning: den ville vise en vandret streg midt i et
  // tomt felt. Sig det i stedet. Flytter serien sig senere, kommer grafen af
  // sig selv.
  if (hi === lo) {
    host.innerHTML = '<p class="empty">U\u00e6ndret p\u00e5 ' + nf.format(hi) +
      " gennem " + pts.length + " m\u00e5linger (" + df(pts[0].date) + " \u2013 " +
      df(pts[pts.length - 1].date) + ").</p>";
    return;
  }
  const W = 720, H = 170, L = 52, R = 46, T = 14, B = 28;
  const iw = W - L - R, ih = H - T - B;
  // En helt flad serie ville give nul spaendvidde og dividere med nul. Den er
  // ogsaa et resultat i sig selv — Docker Hub staar stille — saa den skal tegnes
  // som en vandret streg midt i feltet, ikke skjules.
  const pad = (hi - lo) * 0.12;
  const y0 = lo - pad, y1 = hi + pad;
  const X = (i) => L + (pts.length === 1 ? iw / 2 : (i / (pts.length - 1)) * iw);
  const Y = (v) => T + ih - ((v - y0) / (y1 - y0)) * ih;

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
  svg += '<line class="hair" y1="' + T + '" y2="' + (T + ih) + '"/>';
  svg += '<rect class="hit" x="' + L + '" y="' + T + '" width="' + iw + '" height="' + ih + '"/>';
  svg += "</svg>";
  host.innerHTML = svg + '<div class="tip"></div>';

  const el = host.querySelector("svg"), hair = host.querySelector(".hair"),
        tip = host.querySelector(".tip"), hit = host.querySelector(".hit");
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
    tip.innerHTML = "<strong>" + nf.format(pts[i].v) + "</strong> \u00b7 " + df(pts[i].date);
  });
  hit.addEventListener("pointerleave", () => { hair.style.opacity = 0; tip.style.opacity = 0; });
}
</script>
</body>
</html>
"""


if __name__ == "__main__":
    raise SystemExit(main())
