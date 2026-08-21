// SPDX-License-Identifier: GPL-3.0-or-later
//
// Popup'en tegner kun. Både native-hosten og søgningen ligger i
// baggrundssiden, så omnibox og popup deler ét indeks og én rangering.

"use strict";

const els = {
  query: document.getElementById("query"),
  results: document.getElementById("results"),
  empty: document.getElementById("empty"),
  unlock: document.getElementById("unlock"),
  unlockText: document.getElementById("unlock-text"),
  unlockForm: document.getElementById("unlock-form"),
  unlockButton: document.getElementById("unlock-button"),
  password: document.getElementById("password"),
  footerStatus: document.getElementById("footer-status"),
  lock: document.getElementById("lock"),
  setup: document.getElementById("setup"),
  setupText: document.getElementById("setup-text"),
  setupButton: document.getElementById("setup-button"),
};

// Vejledningen bor på nettet, ikke herinde. Den skal kunne rettes når en
// Firefox-indpakning flytter sine stier — og en signeret udvidelse kan ikke
// rettes uden en ny gennemgang hos AMO. Derfor er URL'erne det eneste faste.
// Ankrene svarer til de to blindgyder popup'en kan havne i.
const SETUP_URL = "https://deltasync.bjoerck-braun.dk/firefox.html";
const SETUP_HOST_URL = SETUP_URL + "#host";
const SETUP_DATABASE_URL = SETUP_URL + "#standalone";

let hits = [];
// Markeringen er en identitet, ikke et indeks i den viste liste: `hit` peger
// på entry'en, og `url` er -1 for selve entry-raekken eller nummeret på en
// udfoldet adresse. Havde vi brugt et listeindeks, ville det skride hver gang
// en udfoldning ændrede antallet af rækker.
let selected = { hit: 0, url: -1 };
let lockedDatabase = null;
// Er kun én database låst op, er dens navn det samme på hver eneste række —
// ren støj, og det står allerede i bundlinjen. Så viser vi kun gruppestien.
let showDatabaseName = false;

// displayURL er kun til øjet; det er hit.url der navigeres til. Skemaet er
// ens på alle rækker og bærer derfor ingen information, men koster den plads
// der ellers ville vise stien — som er dét der adskiller to adresser på
// samme vært.
function displayURL(raw) {
  return raw.replace(/^https?:\/\//, "").replace(/\/$/, "");
}

// send pakker baggrundssidens {ok, ...}-konvolut ud. Fejler noget, kaster vi
// en rigtig Error her — så er der ét sted der skal fanges, og en fejl kan
// ikke ende som en tavs unhandled rejection.
async function send(message) {
  const res = await browser.runtime.sendMessage(message);
  if (!res) throw new Error("no answer from the extension background page");
  if (res.ok) return res;
  const err = new Error(res.message || "unknown error");
  err.needPassword = Boolean(res.needPassword);
  throw err;
}

function showHint(text, isError = false) {
  els.empty.textContent = text;
  els.empty.hidden = !text;
  els.empty.classList.toggle("error", isError);
}

// showSetup er den ene tilstand hvor popup'en ikke kan hjælpe med noget som
// helst: enten mangler hosten, eller også er der ingen database at søge i.
// Begge dele løses uden for browseren, så det eneste nyttige vi kan gøre er
// at pege på vejledningen.
function showSetup(text, label, url) {
  els.setupText.textContent = text;
  els.setupButton.textContent = label;
  els.setupButton.dataset.url = url;
  els.setup.hidden = false;
  els.query.disabled = true;
  els.lock.hidden = true;
  els.unlock.hidden = true;
}

els.setupButton.addEventListener("click", () => {
  browser.tabs.create({ url: els.setupButton.dataset.url || SETUP_URL });
  window.close();
});

async function refresh() {
  let status;
  try {
    status = await send({ type: "status" });
  } catch (err) {
    // Den typiske årsag er at hosten ikke er installeret endnu. Fejlteksten
    // alene efterlader brugeren uden noget at gøre ved det, så den får
    // vejledningen med.
    showHint(`${err.message || err}.`, true);
    showSetup(
      "Firefox cannot reach the keepass-deltasync host. It has to be installed and registered once, outside the browser.",
      "How to set it up",
      SETUP_HOST_URL
    );
    return;
  }

  els.setup.hidden = true;

  const unlocked = status.databases.filter((db) => db.unlocked);
  const locked = status.databases.filter((db) => !db.unlocked);

  if (status.databases.length === 0) {
    // Hosten svarer, så installationen er i orden — der er bare ikke peget
    // på en .kdbx endnu. Det er en anden opgave og en anden knap.
    showHint("");
    showSetup(
      "The host is running, but no database is registered yet. Point it at your .kdbx with \"keepass-deltasync add-local\".",
      "How to add a database",
      SETUP_DATABASE_URL
    );
    return;
  }

  els.query.disabled = unlocked.length === 0;
  els.lock.hidden = unlocked.length === 0;
  showDatabaseName = unlocked.length > 1;
  els.footerStatus.textContent = unlocked.length
    ? `${unlocked.reduce((n, db) => n + db.count, 0)} entries in ${unlocked.map((db) => db.name).join(", ")}`
    : "";

  lockedDatabase = locked.length ? locked[0].name : null;
  els.unlock.hidden = locked.length === 0;
  if (lockedDatabase) {
    els.unlockText.textContent = `${lockedDatabase} is locked.`;
    els.password.hidden = true;
    els.unlockButton.textContent = `Unlock ${lockedDatabase}`;
  }

  if (unlocked.length) {
    els.query.focus();
    if (!els.query.value) showHint("Type to search.");
  }
}

async function runSearch() {
  const query = els.query.value.trim();
  if (!query) {
    hits = [];
    render();
    showHint("Type to search.");
    return;
  }
  try {
    const res = await send({ type: "search", query, limit: 25 });
    hits = clusterByGroup(res.hits || []);
  } catch (err) {
    hits = [];
    render();
    showHint(`Search failed: ${err.message}`, true);
    return;
  }
  selected = { hit: 0, url: -1 };
  render();
  showHint(hits.length ? "" : `No entry matches “${query}”.`);
}

// expandable siger hvor mange adresser en entry folder ud. Kun entries med
// mere end én — en enkelt adresse står allerede på entry-rækken.
function expandable(hitIndex) {
  const hit = hits[hitIndex];
  const n = hit ? hit.entry.urls.length : 0;
  return n > 1 ? n : 0;
}

// whereText er entry'ens plads i træet — overskriften over dens blok.
// Databasenavnet kommer kun med når der er mere end én database at forveksle
// den med; er der kun én, står den allerede i bundlinjen.
function whereText(entry) {
  if (!showDatabaseName) return entry.group || entry.db;
  return entry.group ? `${entry.db}/${entry.group}` : entry.db;
}

// groupKey identificerer gruppen entydigt. IKKE whereText: den udelader
// databasenavnet når kun én er låst op, og to databaser med en gruppe af
// samme navn ville så smelte sammen til én blok.
function groupKey(entry) {
  return `${entry.db}\u0000${entry.group || ""}`;
}

// clusterByGroup samler træffere fra samme gruppe uden at flytte den bedste.
// Grupperne kommer i den rækkefølge deres bedste medlem havde, og inden for
// en gruppe beholder træfferne deres indbyrdes orden — så hits[0] er stadig
// det bedste match, og Enter uden at røre piletasterne gør som før.
//
// Omordningen sker på selve `hits`, ikke kun i visningen: markeringen og
// nextSelection/previousSelection vandrer gennem arrayet, så visuel orden og
// array-orden SKAL være den samme, ellers hopper piletasterne rundt.
function clusterByGroup(list) {
  const order = [];
  const buckets = new Map();
  for (const hit of list) {
    const key = groupKey(hit.entry);
    if (!buckets.has(key)) {
      buckets.set(key, []);
      order.push(key);
    }
    buckets.get(key).push(hit);
  }
  return order.flatMap((key) => buckets.get(key));
}

function groupRow(label) {
  const li = document.createElement("li");
  li.className = "group";
  // Ingen data-hit: hændelseslytterne matcher på li[data-hit], så en
  // overskrift hverken markeres, åbnes eller fanger musen.
  li.textContent = label;
  return li;
}

function entryRow(hit, i, expanded) {
  const li = document.createElement("li");
  li.setAttribute("role", "option");
  li.setAttribute("aria-selected", String(i === selected.hit && selected.url === -1));
  li.dataset.hit = String(i);
  li.dataset.url = "-1";
  li.dataset.navigable = String(Boolean(hit.url));

  const title = document.createElement("span");
  title.className = "title";
  title.textContent = hit.entry.title || "(untitled)";

  if (hit.entry.urls.length > 1) {
    const badge = document.createElement("span");
    badge.className = "badge";
    badge.textContent = `${hit.entry.urls.length} URLs`;
    title.append(" ", badge);
  }
  li.append(title);

  // Er entry'en foldet ud, står adresserne i underrækkerne — så ville det
  // bare være støj at gentage en af dem her. Gruppen står i overskriften
  // over blokken og gentages ikke på hver række.
  const text = expanded ? "" : hit.url ? displayURL(hit.url) : "no usable URL";
  if (text) {
    const url = document.createElement("span");
    url.className = "url";
    url.textContent = text;
    li.append(url);
  }
  return li;
}

function urlRow(hitIndex, k, url) {
  const li = document.createElement("li");
  li.className = "suburl";
  li.setAttribute("role", "option");
  li.setAttribute("aria-selected", String(hitIndex === selected.hit && k === selected.url));
  li.dataset.hit = String(hitIndex);
  li.dataset.url = String(k);
  li.dataset.navigable = "true";
  li.textContent = displayURL(url);
  return li;
}

function render() {
  els.results.textContent = "";
  let openGroup = null;
  hits.forEach((hit, i) => {
    const key = groupKey(hit.entry);
    if (key !== openGroup) {
      openGroup = key;
      els.results.append(groupRow(whereText(hit.entry)));
    }
    const expanded = i === selected.hit && expandable(i) > 0;
    els.results.append(entryRow(hit, i, expanded));
    if (!expanded) return;
    // Bedst matchende adresse først, så den øverste underrække altid er
    // den, Enter på entry-rækken ville have åbnet.
    rankedURLs(hit.entry, hit.urlScores).forEach((ranked, k) => {
      els.results.append(urlRow(i, k, ranked.url));
    });
  });

  const active = els.results.querySelector('[aria-selected="true"]');
  if (active) active.scrollIntoView({ block: "nearest" });
}

// urlAt oversætter en markering til den adresse der skal åbnes.
function urlAt({ hit, url }) {
  const target = hits[hit];
  if (!target) return null;
  if (url < 0) return target.url;
  const ranked = rankedURLs(target.entry, target.urlScores);
  return ranked[url] ? ranked[url].url : null;
}

function move(delta) {
  if (hits.length === 0) return;
  selected = delta > 0 ? nextSelection(selected) : previousSelection(selected);
  render();
}

function nextSelection({ hit, url }) {
  if (url + 1 < expandable(hit)) return { hit, url: url + 1 };
  return { hit: hit + 1 < hits.length ? hit + 1 : 0, url: -1 };
}

function previousSelection({ hit, url }) {
  if (url >= 0) return { hit, url: url - 1 };
  const prev = hit - 1 >= 0 ? hit - 1 : hits.length - 1;
  // expandable() er 0 når entry'en ikke folder ud, og så lander vi på -1,
  // altså dens egen række.
  return { hit: prev, url: expandable(prev) - 1 };
}

async function open(target, event) {
  const url = urlAt(target);
  if (!url) return;

  // Ctrl/Cmd-klik og midterklik åbner i ny fane, som alle andre steder i
  // browseren. Almindeligt klik genbruger den aktive fane, så
  // KeePassXC-Browser kan overtage og udfylde med det samme.
  const newTab = event && (event.ctrlKey || event.metaKey || event.button === 1);
  try {
    await send({
      type: "open",
      url,
      disposition: newTab ? "newForegroundTab" : "currentTab",
    });
  } catch (err) {
    showHint(`Could not open the page: ${err.message}`, true);
    return;
  }
  window.close();
}

els.query.addEventListener("input", runSearch);

els.query.addEventListener("keydown", (event) => {
  switch (event.key) {
    case "ArrowDown":
      event.preventDefault();
      move(1);
      break;
    case "ArrowUp":
      event.preventDefault();
      move(-1);
      break;
    case "Enter":
      event.preventDefault();
      open(selected, event);
      break;
    case "Escape":
      window.close();
      break;
  }
});

function selectionOf(li) {
  return { hit: Number(li.dataset.hit), url: Number(li.dataset.url) };
}

els.results.addEventListener("click", (event) => {
  const li = event.target.closest("li[data-hit]");
  if (li) open(selectionOf(li), event);
});

els.results.addEventListener("auxclick", (event) => {
  if (event.button !== 1) return;
  const li = event.target.closest("li[data-hit]");
  if (li) open(selectionOf(li), event);
});

// Musen skal kunne nå de udfoldede adresser, og udfoldningen følger
// markeringen — så peger man på en række, bliver den markeret.
// Underrækkerne dukker op UNDER den række markøren står på, så den
// flytter sig ikke væk under fingeren.
els.results.addEventListener("mouseover", (event) => {
  const li = event.target.closest("li[data-hit]");
  if (!li) return;
  const next = selectionOf(li);
  if (next.hit === selected.hit && next.url === selected.url) return;
  selected = next;
  render();
});

els.unlockForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  if (!lockedDatabase) return;

  // refresh() nulstiller lockedDatabase, så navnet skal gemmes først — ellers
  // står der "Unlock null" på knappen bagefter.
  const name = lockedDatabase;
  els.unlockButton.disabled = true;
  els.unlockButton.textContent = "Unlocking…";
  try {
    await send({
      type: "unlock",
      db: name,
      password: els.password.hidden ? undefined : els.password.value,
    });
    els.password.value = "";
    els.unlockText.classList.remove("error");
    await refresh();
  } catch (err) {
    if (err.needPassword) {
      // Ingen keyring-entry for databasen. Først her beder vi om
      // masterpasswordet — normalvejen henter hosten det selv.
      els.password.hidden = false;
      els.password.focus();
      els.unlockText.textContent = `${name} has no entry in the OS keyring.`;
    } else {
      els.unlockText.textContent = err.message;
      els.unlockText.classList.add("error");
    }
  } finally {
    els.unlockButton.disabled = false;
    els.unlockButton.textContent = `Unlock ${name}`;
  }
});

els.lock.addEventListener("click", async () => {
  try {
    await send({ type: "lock" });
  } catch (err) {
    showHint(`Lock failed: ${err.message}`, true);
    return;
  }
  hits = [];
  els.query.value = "";
  selected = { hit: 0, url: -1 };
  render();
  await refresh();
});

refresh();
