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
};

let hits = [];
// Markeringen er en identitet, ikke et indeks i den viste liste: `hit` peger
// på entry'en, og `url` er -1 for selve entry-raekken eller nummeret på en
// udfoldet adresse. Havde vi brugt et listeindeks, ville det skride hver gang
// en udfoldning ændrede antallet af rækker.
let selected = { hit: 0, url: -1 };
let lockedDatabase = null;

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

async function refresh() {
  let status;
  try {
    status = await send({ type: "status" });
  } catch (err) {
    // Den typiske årsag er at hosten ikke er installeret endnu — så er en
    // konkret kommando mere brugbar end fejlteksten alene.
    showHint(
      `${err.message || err}. Run "keepass-deltasync install-browser-host" and restart Firefox.`,
      true
    );
    return;
  }

  const unlocked = status.databases.filter((db) => db.unlocked);
  const locked = status.databases.filter((db) => !db.unlocked);

  if (status.databases.length === 0) {
    showHint("No databases are registered. Run \"keepass-deltasync init\" first.", true);
    return;
  }

  els.query.disabled = unlocked.length === 0;
  els.lock.hidden = unlocked.length === 0;
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
    hits = res.hits || [];
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

  const meta = document.createElement("span");
  meta.className = "meta";

  // Er entry'en foldet ud, står adresserne i underrækkerne — så ville det
  // bare være støj at gentage en af dem her.
  const url = document.createElement("span");
  url.className = "url";
  url.textContent = expanded ? "" : hit.url || "no usable URL";
  meta.append(url);

  const where = document.createElement("span");
  where.className = "where";
  where.textContent = hit.entry.group ? `${hit.entry.db}/${hit.entry.group}` : hit.entry.db;
  meta.append(where);

  li.append(meta);
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
  li.textContent = url;
  return li;
}

function render() {
  els.results.textContent = "";
  hits.forEach((hit, i) => {
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
  const li = event.target.closest("li");
  if (li) open(selectionOf(li), event);
});

els.results.addEventListener("auxclick", (event) => {
  if (event.button !== 1) return;
  const li = event.target.closest("li");
  if (li) open(selectionOf(li), event);
});

// Musen skal kunne nå de udfoldede adresser, og udfoldningen følger
// markeringen — så peger man på en række, bliver den markeret.
// Underrækkerne dukker op UNDER den række markøren står på, så den
// flytter sig ikke væk under fingeren.
els.results.addEventListener("mouseover", (event) => {
  const li = event.target.closest("li");
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
