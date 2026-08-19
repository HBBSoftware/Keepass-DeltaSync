// SPDX-License-Identifier: GPL-3.0-or-later
//
// Søgningen over det cachede indeks. Delt mellem baggrundssiden (omnibox) og
// popup'en, så de to altid rangerer ens.
//
// Indekset indeholder kun titel, URL'er og gruppesti — se
// docs/browser-extension.md. Der er derfor ikke noget at søge i som er
// hemmeligt, og hele søgningen kan køre lokalt uden at spørge hosten.
//
// Ét søgeresultat = én entry, også når entry'en har flere adresser
// (KeePassXC' "Additional URLs", gemt som KP2A_URL_* under Yderligere
// attributter). Hittet bærer den bedst matchende adresse i `url`, og
// `urlScores` fortæller hvor godt hver enkelt adresse matchede, så
// brugerfladen kan folde dem ud i rigtig rækkefølge når entry'en vælges.
//
// Adressen skal kunne vælges et sted — ellers er de øvrige uopnåelige — men
// listen skal ikke fyldes med den samme entry flere gange.

"use strict";

// hostOf trækker værtsnavnet ud af en URL uden at bygge et URL-objekt for
// hvert opslag — søgningen kører på hvert tastetryk.
function hostOf(url) {
  const start = url.indexOf("://");
  if (start === -1) return url.toLowerCase();
  const rest = url.slice(start + 3);
  const end = rest.search(/[/?#]/);
  return (end === -1 ? rest : rest.slice(0, end)).toLowerCase();
}

// urlScore vurderer ét søgeord mod én URL. -1 = matcher ikke.
//
// Værten vejer tungest, fordi det er den, folk husker. Et match længere inde
// i stien tæller stadig, men må ikke kunne udkonkurrere et værtsmatch.
function urlScore(url, token) {
  const host = hostOf(url);
  if (host.startsWith(token)) return 100;
  if (host.includes(token)) return 70;
  if (url.toLowerCase().includes(token)) return 25;
  return -1;
}

// textScore vurderer ét søgeord mod alt det ved en entry der ikke er en URL.
function textScore(entry, token) {
  let best = -1;

  const title = entry.title.toLowerCase();
  if (title.startsWith(token)) best = 90;
  else if (title.includes(token)) best = 55;

  if (entry.group && entry.group.toLowerCase().includes(token)) {
    best = Math.max(best, 20);
  }
  if (entry.db && entry.db.toLowerCase() === token) {
    best = Math.max(best, 15);
  }
  return best;
}

// scoreEntry vurderer hele søgestrengen mod én entry. null = ingen match.
//
// Flere søgeord er et AND: hvert ord skal ramme et sted i entry'en (titel,
// gruppe eller en af URL'erne), ellers ryger entry'en ud. Det gør "bank web"
// til en indsnævring frem for en udvidelse. Bemærk at AND'et gælder entry'en
// som helhed — ordene behøver ikke ramme den SAMME adresse.
function scoreEntry(entry, tokens) {
  const urls = entry.urls || [];
  const perUrl = new Array(urls.length).fill(0);
  let total = 0;
  let textTotal = 0;

  for (const token of tokens) {
    const fromText = textScore(entry, token);
    if (fromText > 0) textTotal += fromText;

    let best = fromText;
    for (let i = 0; i < urls.length; i++) {
      const s = urlScore(urls[i], token);
      if (s < 0) continue;
      perUrl[i] += s;
      if (s > best) best = s;
    }
    if (best < 0) return null;
    total += best;
  }

  // Entries uden navigerbar URL er stadig værd at vise — brugeren kan se at
  // de findes — men de skal ikke skubbe brugbare hits ned.
  if (urls.length === 0) total -= 30;

  return { total, perUrl, textTotal };
}

// bestURL udpeger den adresse et hit skal åbne som standard.
//
// Bar titlen matchet, peger søgningen på entry'en som helhed frem for på en
// bestemt adresse — så vinder den primære. Ramte en konkret adresse hårdere
// end titlen, er det den, brugeren ledte efter.
function bestURL(entry, { perUrl, textTotal }) {
  const urls = entry.urls || [];
  if (urls.length === 0) return null;

  let bestIdx = 0;
  for (let i = 1; i < urls.length; i++) {
    if (perUrl[i] > perUrl[bestIdx]) bestIdx = i;
  }
  if (textTotal >= perUrl[bestIdx]) bestIdx = 0;
  return urls[bestIdx];
}

// rankedURLs returnerer entry'ens adresser i den rækkefølge de skal vises,
// bedst matchende først. Alle er med — også dem der ikke matchede — for det
// er hele pointen med udfoldningen: at kunne se og vælge dem alle.
//
// Array.sort er stabil, så adresser med samme score beholder deres
// oprindelige rækkefølge, og den primære bliver liggende først blandt lige.
function rankedURLs(entry, urlScores) {
  return (entry.urls || [])
    .map((url, i) => ({ url, score: urlScores[i] }))
    .sort((a, b) => b.score - a.score);
}

// searchIndex returnerer de bedst matchende entries, bedste først — én
// række pr. entry.
function searchIndex(index, query, limit = 20) {
  const tokens = query.toLowerCase().trim().split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return [];

  const hits = [];
  for (const entry of index) {
    const scored = scoreEntry(entry, tokens);
    if (!scored) continue;
    hits.push({
      entry,
      score: scored.total,
      url: bestURL(entry, scored),
      urlScores: scored.perUrl,
    });
  }

  hits.sort((a, b) => b.score - a.score || a.entry.title.localeCompare(b.entry.title));
  return hits.slice(0, limit);
}

if (typeof module !== "undefined") {
  module.exports = { searchIndex, scoreEntry, bestURL, rankedURLs, urlScore, textScore, hostOf };
}
