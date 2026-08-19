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
let selected = 0;
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
  selected = 0;
  render();
  showHint(hits.length ? "" : `No entry matches “${query}”.`);
}

function render() {
  els.results.textContent = "";
  hits.forEach((hit, i) => {
    const { entry } = hit;
    const li = document.createElement("li");
    li.setAttribute("role", "option");
    li.setAttribute("aria-selected", String(i === selected));
    li.dataset.index = String(i);
    li.dataset.navigable = String(Boolean(hit.url));

    const title = document.createElement("span");
    title.className = "title";
    title.textContent = entry.title || "(untitled)";

    // Har entry'en flere adresser, siger badgen hvor mange — og om det var en
    // af de øvrige, søgningen ramte. Ellers ville det se forkert ud at åbne
    // noget andet end entry'ens primære URL.
    if (entry.urls.length > 1) {
      const badge = document.createElement("span");
      badge.className = "badge";
      badge.textContent = hit.matchedURL && entry.urls[0] !== hit.url
        ? `matched 1 of ${entry.urls.length} URLs`
        : `${entry.urls.length} URLs`;
      title.append(" ", badge);
    }
    li.append(title);

    const meta = document.createElement("span");
    meta.className = "meta";

    const url = document.createElement("span");
    url.className = "url";
    url.textContent = hit.url || "no usable URL";
    meta.append(url);

    const where = document.createElement("span");
    where.className = "where";
    where.textContent = entry.group ? `${entry.db}/${entry.group}` : entry.db;
    meta.append(where);

    li.append(meta);
    els.results.append(li);
  });

  const active = els.results.querySelector('[aria-selected="true"]');
  if (active) active.scrollIntoView({ block: "nearest" });
}

function move(delta) {
  if (hits.length === 0) return;
  selected = (selected + delta + hits.length) % hits.length;
  render();
}

async function open(index, event) {
  const hit = hits[index];
  if (!hit || !hit.url) return;

  // Ctrl/Cmd-klik og midterklik åbner i ny fane, som alle andre steder i
  // browseren. Almindeligt klik genbruger den aktive fane, så
  // KeePassXC-Browser kan overtage og udfylde med det samme.
  const newTab = event && (event.ctrlKey || event.metaKey || event.button === 1);
  try {
    await send({
      type: "open",
      url: hit.url,
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

els.results.addEventListener("click", (event) => {
  const li = event.target.closest("li");
  if (li) open(Number(li.dataset.index), event);
});

els.results.addEventListener("auxclick", (event) => {
  if (event.button !== 1) return;
  const li = event.target.closest("li");
  if (li) open(Number(li.dataset.index), event);
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
  render();
  await refresh();
});

refresh();
