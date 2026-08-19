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
  // Genindekser lydløst. Har databasen kun et fallback-password, og har
  // idle-låsen taget det, fejler det her; så bliver det gamle indeks stående,
  // hvilket er bedre end at prompte brugeren uopfordret.
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
  }
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

browser.omnibox.onInputChanged.addListener(async (text, addSuggestions) => {
  if (!text.trim()) return;
  const hits = searchIndex(await allEntries(), text, 6).filter((h) => h.url);
  addSuggestions(
    hits.map((hit) => {
      const { entry } = hit;
      const where = entry.group ? `${entry.db}/${entry.group}` : entry.db;
      // hit.url — ikke entry.urls[0]. Har entry'en flere adresser, og var det
      // den anden der matchede, er det den vi skal foreslå.
      return {
        content: hit.url,
        description: `${escapeXml(entry.title)} <dim>${escapeXml(where)}</dim> <url>${escapeXml(hit.url)}</url>`,
      };
    })
  );
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
      return unlockDatabase(msg.db, msg.password);
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
  return {
    version: hostStatus.version,
    databases: (hostStatus.databases || []).map((db) => ({
      name: db.name,
      unlocked: Boolean(cache[db.name]),
      count: cache[db.name] ? cache[db.name].entries.length : 0,
    })),
  };
}
