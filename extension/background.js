// SPDX-License-Identifier: GPL-3.0-or-later
//
// Baggrundssiden: eneste sted der taler med native messaging-hosten, og
// eneste sted der ejer indekset. Popup og omnibox går begge gennem den.
//
// Sikkerhedsgrænsen ligger i hosten (se docs/browser-extension.md): den
// sender kun titel, URL'er og gruppesti, så der er intet hemmeligt i det
// indeks denne fil cacher. Masterpasswordet passerer kun herigennem hvis
// databasen mangler en entry i OS-keyringen — normalvejen henter hosten det
// selv, og så ser browseren det aldrig.

"use strict";

const HOST_NAME = "dk.hbb.keepass_deltasync";
const REQUEST_TIMEOUT_MS = 3 * 60 * 1000; // Argon2 + keepassxc-cli må gerne tage tid.
const INDEX_KEY = "index";
const AUTO_KEY = "autoconnect";

let port = null;
let nextRequestId = 1;
const pending = new Map();

// ---------------------------------------------------------------- transport

function connect() {
  if (port) return port;

  const p = browser.runtime.connectNative(HOST_NAME);
  p.onMessage.addListener(onHostMessage);
  p.onDisconnect.addListener(() => {
    const reason = p.error ? p.error.message : "the native host closed the connection";
    port = null;
    for (const [, waiter] of pending) {
      clearTimeout(waiter.timer);
      waiter.reject(new Error(reason));
    }
    pending.clear();
  });
  port = p;
  return p;
}

function send(message) {
  return new Promise((resolve, reject) => {
    let p;
    try {
      p = connect();
    } catch (err) {
      reject(new Error(`cannot start the native host: ${err.message}`));
      return;
    }

    const id = nextRequestId++;
    const timer = setTimeout(() => {
      pending.delete(id);
      reject(new Error("the native host did not answer in time"));
    }, REQUEST_TIMEOUT_MS);

    pending.set(id, { resolve, reject, timer });
    try {
      p.postMessage({ ...message, id });
    } catch (err) {
      clearTimeout(timer);
      pending.delete(id);
      reject(err);
    }
  });
}

function onHostMessage(msg) {
  if (msg.event) {
    handleHostEvent(msg);
    return;
  }

  const waiter = pending.get(msg.id);
  if (!waiter) return; // Svar på en request vi allerede har givet op på.
  clearTimeout(waiter.timer);
  pending.delete(msg.id);

  if (msg.ok) {
    waiter.resolve(msg);
    return;
  }
  const err = new Error(msg.error || "the native host reported an error");
  err.needPassword = Boolean(msg.need_password);
  err.db = msg.db;
  waiter.reject(err);
}

async function handleHostEvent(msg) {
  if (msg.event !== "changed") return;

  // Databasen er skrevet på disken — typisk fordi en sync lige er landet.
  // Genindekser lydløst, men kun hvis vi allerede HAVDE et indeks: efter et
  // "Lock" bliver hostens fil-watch stående, og uden dette ville den første
  // sync bagefter låse databasen op igen bag ryggen på brugeren.
  const cache = await readCache();
  if (!cache[msg.db]) return;

  // Har databasen kun et fallback-password, og har idle-låsen taget det,
  // fejler det her; så bliver det gamle indeks stående, hvilket er bedre end
  // at prompte brugeren uopfordret.
  try {
    await unlockDatabase(msg.db);
  } catch (err) {
    console.warn(`re-index of ${msg.db} after a change failed: ${err.message}`);
  }
}

// -------------------------------------------------------------- index cache

async function readCache() {
  const stored = await browser.storage.session.get(INDEX_KEY);
  return stored[INDEX_KEY] || {};
}

async function writeCache(cache) {
  await browser.storage.session.set({ [INDEX_KEY]: cache });
}

// allEntries fladgør alle ulåste databaser til én liste, så et søgeresultat
// kan spænde over flere databaser.
async function allEntries() {
  const cache = await readCache();
  const out = [];
  for (const db of Object.keys(cache)) {
    const list = cache[db] && cache[db].entries;
    // En halvskrevet cache-post må ikke vælte søgningen med en TypeError.
    if (Array.isArray(list)) out.push(...list);
  }
  return out;
}

// unlockDatabase låser op og henter hele indekset side for side. Firefox
// tillader højst 1 MB pr. besked fra hosten, så et stort indeks kommer i
// flere bidder.
async function unlockDatabase(db, password) {
  const opened = await send({ cmd: "unlock", db, password });

  const entries = [];
  let offset = 0;
  while (offset < opened.count) {
    const page = await send({ cmd: "index", db: opened.db, offset });
    // Normalisér her, ét sted, i stedet for at strø `|| []` ud over søgning,
    // omnibox og popup: efter dette punkt HAR hver entry et urls-array.
    for (const entry of page.entries || []) {
      entries.push({ ...entry, urls: entry.urls || [] });
    }
    if (page.next <= offset) break; // Skal ikke ske, men lad os ikke loope.
    offset = page.next;
  }

  const cache = await readCache();
  cache[opened.db] = { entries, generation: opened.generation };
  await writeCache(cache);

  return { db: opened.db, count: entries.length };
}

async function lockAll() {
  try {
    await send({ cmd: "lock" });
  } finally {
    await browser.storage.session.remove(INDEX_KEY);
    // Et bevidst "Lock" slår auto-connect fra resten af sessionen. Uden det
    // ville næste popup-åbning låse op igen, og knappen var ren pynt.
    await writeAuto({ suppressed: true, failed: {} });
  }
}

// ------------------------------------------------------------ auto-connect

// Normalvejen kræver ikke brugeren: hosten henter selv masterpasswordet i
// OS-keyringen, så `unlock` beder ikke om noget. Derfor låser popup'en op af
// sig selv, i stedet for at parkere brugeren bag en knap der ikke gør andet.
//
// To ting må det ikke gøre:
//
//   * ophæve et bevidst "Lock" — så havde knappen ingen virkning,
//   * prøve igen ved hver eneste popup-åbning på en database der ikke KAN
//     låses op uden hjælp. Hvert forsøg koster en Argon2-kørsel, og svaret
//     bliver det samme.
//
// Begge dele bor i session storage sammen med indekset, ikke i en variabel
// her i filen: baggrundssiden er en event page, som browseren må lukke ned
// mellem to popup-åbninger. Og som indekset nulstilles tilstanden når
// Firefox lukkes — et "Lock" holder altså kun sessionen ud.

async function readAuto() {
  const stored = await browser.storage.session.get(AUTO_KEY);
  return stored[AUTO_KEY] || { suppressed: false, failed: {} };
}

async function writeAuto(state) {
  await browser.storage.session.set({ [AUTO_KEY]: state });
}

// pendingConnect deler ét forsøg mellem samtidige kaldere: popup'en kan lukkes
// og åbnes igen mens den første unlock stadig kører, og adresselinjen kan
// varme op oven i den. To Argon2-kørsler på samme database ville hverken være
// hurtigere eller pænere.
let pendingConnect = null;

function autoConnect() {
  if (!pendingConnect) {
    pendingConnect = runAutoConnect().finally(() => {
      pendingConnect = null;
    });
  }
  return pendingConnect;
}

async function runAutoConnect() {
  const state = await readAuto();
  if (state.suppressed) return { attempted: [] };

  const hostStatus = await send({ cmd: "status" });
  const cache = await readCache();
  const attempted = [];

  for (const db of hostStatus.databases || []) {
    if (cache[db.name] || state.failed[db.name]) continue;
    attempted.push(db.name);
    try {
      await unlockDatabase(db.name);
      delete state.failed[db.name];
    } catch (err) {
      // Fejlen gemmes, ikke bare logges. Popup'en tegner beskeden — og for en
      // database uden keyring-entry er password-feltet det rigtige svar, ikke
      // en knap der ville fejle på nøjagtig samme måde igen.
      state.failed[db.name] = {
        message: err.message || String(err),
        needPassword: Boolean(err.needPassword),
      };
    }
  }

  await writeAuto(state);
  return { attempted };
}

// manualUnlock er unlock med brugeren bag sig. Det ophæver både et tidligere
// "Lock" og et auto-forsøg der slog fejl — ellers ville en database man lige
// har låst op i hånden stadig være undtaget fra auto-connect bagefter.
async function manualUnlock(msg) {
  const result = await unlockDatabase(msg.db, msg.password);
  const state = await readAuto();
  state.suppressed = false;
  delete state.failed[result.db];
  await writeAuto(state);
  return result;
}

// --------------------------------------------------------------- navigation

async function openEntry(url, disposition = "currentTab") {
  if (!/^https?:\/\//i.test(url)) {
    throw new Error(`refusing to navigate to ${url}`);
  }
  if (disposition === "newForegroundTab") {
    await browser.tabs.create({ url, active: true });
    return;
  }
  if (disposition === "newBackgroundTab") {
    await browser.tabs.create({ url, active: false });
    return;
  }
  const [tab] = await browser.tabs.query({ active: true, currentWindow: true });
  if (tab) {
    await browser.tabs.update(tab.id, { url });
  } else {
    await browser.tabs.create({ url });
  }
}

// ------------------------------------------------------------------ omnibox

browser.omnibox.setDefaultSuggestion({
  description: "Search your KeePass entries and open the entry's site",
});

function escapeXml(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

function suggestionFor(entry, url) {
  const where = entry.group ? `${entry.db}/${entry.group}` : entry.db;
  return {
    content: url,
    description: `${escapeXml(entry.title)} <dim>${escapeXml(where)}</dim> <url>${escapeXml(url)}</url>`,
  };
}

browser.omnibox.onInputChanged.addListener(async (text, addSuggestions) => {
  if (!text.trim()) return;

  const entries = await allEntries();
  // Adresselinjen har ingen knap at trykke på: er intet låst op, er listen
  // tom, og brugeren kan ikke gøre noget ved det herfra. Så varmer vi
  // indekset op — uden at vente på det, for et forslag der først kommer efter
  // en Argon2-kørsel er alligevel for sent til dette tastetryk. Næste tegn
  // har det.
  if (entries.length === 0) autoConnect().catch(() => {});

  const hits = searchIndex(entries, text, 6).filter((h) => h.url);

  // Adresselinjen er en flad liste og kan ikke folde ud som popup'en. Skal en
  // entry's øvrige adresser kunne nås herfra, må de have hvert sit forslag.
  // De lægges bagest, så de aldrig fortrænger et andet hits hovedadresse.
  const primary = [];
  const extras = [];
  for (const hit of hits) {
    primary.push(suggestionFor(hit.entry, hit.url));
    for (const alt of rankedURLs(hit.entry, hit.urlScores)) {
      if (alt.url !== hit.url && alt.score > 0) {
        extras.push(suggestionFor(hit.entry, alt.url));
      }
    }
  }
  addSuggestions(primary.concat(extras).slice(0, 6));
});

browser.omnibox.onInputEntered.addListener(async (text, disposition) => {
  // Trykker brugeren Enter uden at vælge et forslag, får vi den rå tekst.
  // Så søger vi selv og tager det bedste hit.
  let url = text;
  if (!/^https?:\/\//i.test(url)) {
    const hits = searchIndex(await allEntries(), text, 1).filter((h) => h.url);
    if (hits.length === 0) return;
    url = hits[0].url;
  }
  await openEntry(url, disposition);
});

// -------------------------------------------------------- popup message API

// Alle svar til popup'en er en {ok, ...}-konvolut, aldrig et afvist promise.
//
// Grunden er at fejl ikke overlever turen mellem to extension-kontekster
// ordentligt: kun `message` på et rigtigt Error-objekt kommer med, og
// egenskaber som needPassword går tabt. Ved at pakke både succes og fejl i
// et almindeligt objekt kan popup'en altid se hvad der skete — og en fejl
// kan ikke længere forsvinde tavst.
browser.runtime.onMessage.addListener((msg) =>
  handlePopupMessage(msg).then(
    (data) => ({ ok: true, ...data }),
    (err) => {
      console.error(`${msg && msg.type} failed:`, err);
      return {
        ok: false,
        message: (err && err.message) || String(err),
        needPassword: Boolean(err && err.needPassword),
      };
    }
  )
);

async function handlePopupMessage(msg) {
  switch (msg.type) {
    case "status":
      return status();
    case "unlock":
      return manualUnlock(msg);
    case "connect":
      return autoConnect();
    case "lock":
      await lockAll();
      return {};
    case "search":
      return { hits: searchIndex(await allEntries(), msg.query, msg.limit || 25) };
    case "open":
      await openEntry(msg.url, msg.disposition);
      return {};
    case "diag":
      return diagnostics();
    default:
      throw new Error(`unknown message ${msg.type}`);
  }
}

// diagnostics svarer på "hvorfor sker der ingenting". Kør den i
// baggrundssidens konsol (about:debugging → Inspect) eller send
// {type:"diag"} fra popup'en.
async function diagnostics() {
  const cache = await readCache();
  const report = {
    searchAvailable: typeof searchIndex === "function",
    cachedDatabases: Object.keys(cache).map((db) => ({
      db,
      entries: Array.isArray(cache[db].entries) ? cache[db].entries.length : "MALFORMED",
    })),
    hostReachable: null,
    hostError: null,
  };
  try {
    const st = await send({ cmd: "status" });
    report.hostReachable = true;
    report.hostDatabases = st.databases;
  } catch (err) {
    report.hostReachable = false;
    report.hostError = err.message;
  }
  return report;
}

// status kombinerer hostens billede (hvilke databaser findes) med vores eget
// (hvilke har vi et indeks for), så popup'en kan tegne begge dele.
async function status() {
  const hostStatus = await send({ cmd: "status" });
  const cache = await readCache();
  const auto = await readAuto();

  const databases = (hostStatus.databases || []).map((db) => ({
    name: db.name,
    unlocked: Boolean(cache[db.name]),
    count: cache[db.name] ? cache[db.name].entries.length : 0,
    // Null når intet auto-forsøg er slået fejl. Ellers {message,
    // needPassword} fra det forsøg, så popup'en kan sige hvad der gik galt i
    // stedet for bare "is locked".
    failure: auto.failed[db.name] || null,
  }));

  return {
    version: hostStatus.version,
    databases,
    // Er der noget at hente ved at spørge? Reglerne for hvornår et automatisk
    // forsøg er i orden bor her, så popup'en ikke skal kende dem.
    canAutoConnect: !auto.suppressed && databases.some((db) => !db.unlocked && !db.failure),
  };
}
