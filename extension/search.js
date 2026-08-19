// SPDX-License-Identifier: GPL-3.0-or-later
//
// Søgningen over det cachede indeks. Delt mellem baggrundssiden (omnibox) og
// popup'en, så de to altid rangerer ens.
//
// Indekset indeholder kun titel, URL'er og gruppesti — se
// docs/browser-extension.md. Der er derfor ikke noget at søge i som er
// hemmeligt, og hele søgningen kan køre lokalt uden at spørge hosten.
//
// Et SØGERESULTAT er et (entry, url)-par, ikke en entry. En entry med flere
// adresser — KeePassXC' "Additional URLs", gemt som KP2A_URL_* under
// Yderligere attributter — kan derfor optræde med en række pr. adresse der
// matchede. Ellers ville man kun kunne nå den højest rangerede af dem.

"use strict";

// Loft for hvor mange rækker én entry må fylde. Uden det kunne et bredt
// søgeord som "login" lade en enkelt entry med mange adresser fylde hele
// resultatlisten.
const MAX_ROWS_PER_ENTRY = 3;

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

// rowsFor oversætter én matchende entry til de rækker den skal fylde.
function rowsFor(entry, { total, perUrl, textTotal }) {
  const urls = entry.urls || [];
  if (urls.length === 0) {
    return [{ entry, url: null, score: total, matchedURL: false }];
  }

  let bestIdx = 0;
  for (let i = 1; i < urls.length; i++) {
    if (perUrl[i] > perUrl[bestIdx]) bestIdx = i;
  }

  // Bar titlen matchet, peger søgningen på entry'en som helhed frem for på en
  // bestemt adresse — så skal den primære ligge øverst. Ramte en konkret
  // adresse hårdere end titlen, er det den, brugeren ledte efter.
  const titleCarried = textTotal >= perUrl[bestIdx];

  // Den primære adresse er altid med: den er entry'ens hovedindgang, også når
  // det var en anden af dens adresser der matchede.
  const rows = [
    {
      entry,
      url: urls[0],
      score: titleCarried ? total : total + perUrl[0] - perUrl[bestIdx],
      matchedURL: perUrl[0] > 0,
    },
  ];

  // Hver ANDEN adresse der selv matchede får sin egen række, stærkeste først,
  // så den kan vælges med piletasterne i stedet for at være uopnåelig.
  const others = [];
  for (let i = 1; i < urls.length; i++) {
    if (perUrl[i] > 0) others.push(i);
  }
  others.sort((a, b) => perUrl[b] - perUrl[a]);

  for (const i of others.slice(0, MAX_ROWS_PER_ENTRY - 1)) {
    rows.push({
      entry,
      url: urls[i],
      score: total + perUrl[i] - perUrl[bestIdx],
      matchedURL: true,
    });
  }
  return rows;
}

// searchIndex returnerer de bedst matchende (entry, url)-par, bedste først.
function searchIndex(index, query, limit = 20) {
  const tokens = query.toLowerCase().trim().split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return [];

  const hits = [];
  for (const entry of index) {
    const scored = scoreEntry(entry, tokens);
    if (scored) hits.push(...rowsFor(entry, scored));
  }

  // Array.sort er stabil, og rowsFor lægger altid den primære adresse først.
  // Går to rækker fra samme entry i lige score, bevarer den primære derfor
  // sin plads øverst.
  hits.sort((a, b) => b.score - a.score || a.entry.title.localeCompare(b.entry.title));
  return hits.slice(0, limit);
}

if (typeof module !== "undefined") {
  module.exports = { searchIndex, scoreEntry, rowsFor, urlScore, textScore, hostOf };
}
