// SPDX-License-Identifier: GPL-3.0-or-later
//
// Søgningen over det cachede indeks. Delt mellem baggrundssiden (omnibox) og
// popup'en, så de to altid rangerer ens.
//
// Indekset indeholder kun titel, URL'er og gruppesti — se
// docs/browser-extension.md. Der er derfor ikke noget at søge i som er
// hemmeligt, og hele søgningen kan køre lokalt uden at spørge hosten.

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

// scoreToken giver ét søgeord en score mod én entry. -1 betyder "matcher
// ikke", og så ryger hele entry'en ud: flere ord er et AND, ikke et OR.
//
// Vægtningen afspejler hvad folk faktisk skriver. Man husker domænet før
// titlen, og titlen før hvilken gruppe entry'en ligger i.
function scoreToken(entry, token) {
  let best = -1;

  for (const url of entry.urls) {
    const host = hostOf(url);
    if (host.startsWith(token)) best = Math.max(best, 100);
    else if (host.includes(token)) best = Math.max(best, 70);
    // En del af stien tæller også, bare svagere end værten.
    else if (url.toLowerCase().includes(token)) best = Math.max(best, 25);
  }

  const title = entry.title.toLowerCase();
  if (title.startsWith(token)) best = Math.max(best, 90);
  else if (title.includes(token)) best = Math.max(best, 55);

  if (entry.group && entry.group.toLowerCase().includes(token)) {
    best = Math.max(best, 20);
  }
  if (entry.db && entry.db.toLowerCase() === token) {
    best = Math.max(best, 15);
  }

  return best;
}

// searchIndex returnerer de bedst matchende entries, bedste først.
function searchIndex(index, query, limit = 20) {
  const tokens = query.toLowerCase().trim().split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return [];

  const hits = [];
  for (const entry of index) {
    let total = 0;
    let matchedAll = true;
    for (const token of tokens) {
      const s = scoreToken(entry, token);
      if (s < 0) {
        matchedAll = false;
        break;
      }
      total += s;
    }
    if (!matchedAll) continue;
    // Entries uden navigerbar URL er stadig værd at vise (brugeren kan se
    // at de findes), men de skal ikke skubbe brugbare hits ned.
    if (entry.urls.length === 0) total -= 30;
    hits.push({ entry, score: total });
  }

  hits.sort((a, b) => b.score - a.score || a.entry.title.localeCompare(b.entry.title));
  return hits.slice(0, limit);
}

if (typeof module !== "undefined") {
  module.exports = { searchIndex, scoreToken, hostOf };
}
