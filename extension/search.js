// SPDX-License-Identifier: GPL-3.0-or-later
//
// Søgningen over det cachede indeks. Delt mellem baggrundssiden (omnibox) og
// popup'en, så de to altid rangerer ens.
//
// Indekset indeholder kun titel, URL'er og gruppesti — se
// docs/browser-extension.md. Der er derfor ikke noget at søge i som er
// hemmeligt, og hele søgningen kan køre lokalt uden at spørge hosten.
//
// En entry kan have flere URL'er (KeePassXC' "Additional URLs", gemt som
// KP2A_URL_*). Alle indekseres, alle er søgbare, og hvert hit husker HVILKEN
// af dem søgningen ramte — ellers ville et match på entry'ens anden adresse
// sende brugeren til den første.

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

// scoreEntry vurderer hele søgestrengen mod én entry, og udpeger samtidig
// den URL der skal åbnes. null = ingen match.
//
// Flere søgeord er et AND: hvert ord skal ramme et sted i entry'en (titel,
// gruppe eller en af URL'erne), ellers ryger entry'en ud. Det gør "bank web"
// til en indsnævring frem for en udvidelse.
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

  // Åbn den URL søgningen faktisk pegede på.
  //
  // Men kun når en konkret adresse ER det stærkeste signal. Matcher man
  // primært på titlen, peger søgningen på entry'en som helhed, og så skal vi
  // åbne dens primære adresse — ikke den sekundære, der tilfældigvis også
  // indeholdt ordet. Søger man derimod på noget der kun findes i den anden
  // URL, er det den, man vil hen til.
  let chosen = 0;
  for (let i = 1; i < urls.length; i++) {
    if (perUrl[i] > perUrl[chosen]) chosen = i;
  }
  if (perUrl[chosen] <= textTotal) chosen = 0;

  return {
    score: total,
    url: urls.length ? urls[chosen] : null,
    matchedURL: urls.length > 0 && perUrl[chosen] > 0,
  };
}

// searchIndex returnerer de bedst matchende entries, bedste først. Hvert hit
// bærer `url` — den adresse dette match peger på, ikke nødvendigvis entry'ens
// primære.
function searchIndex(index, query, limit = 20) {
  const tokens = query.toLowerCase().trim().split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return [];

  const hits = [];
  for (const entry of index) {
    const scored = scoreEntry(entry, tokens);
    if (!scored) continue;
    hits.push({ entry, score: scored.score, url: scored.url, matchedURL: scored.matchedURL });
  }

  hits.sort((a, b) => b.score - a.score || a.entry.title.localeCompare(b.entry.title));
  return hits.slice(0, limit);
}

if (typeof module !== "undefined") {
  module.exports = { searchIndex, scoreEntry, urlScore, textScore, hostOf };
}
