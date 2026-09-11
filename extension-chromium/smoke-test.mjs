// SPDX-License-Identifier: GPL-3.0-or-later
//
// Kører den byggede pakkes baggrundskode mod et opdigtet `chrome` og tjekker
// at skallen i compat.js dækker det, den delte kode faktisk bruger.
//
// Hvorfor: logikken er Firefox-udgavens, ordret. Den kalder `browser.*`, og
// mangler ét af de kald i skallen, opdager man det først når popup'en står
// tom i en rigtig browser — uden noget i konsollen der peger på hvad. Her
// fejler det i stedet med det samme, med navnet på kaldet der manglede.
//
// Testen er ikke en erstatning for at prøve udvidelsen i Chrome. Den fanger
// den ene fejlklasse der ellers er dyrest at finde, og den kan køre uden en
// browser installeret.
//
//   node smoke-test.mjs        # efter ./package.sh
//
// Kræver kun node. Ingen pakker.

import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import vm from "node:vm";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const UNPACKED = path.join(HERE, "build", "unpacked");

if (!fs.existsSync(UNPACKED)) {
  console.error("build/unpacked is missing — run ./package.sh first");
  process.exit(1);
}

// ---------------------------------------------------------------- fake host
//
// Svarer som browser-host'en gør: ét svar per request, med samme id tilbage.
// Kun de kommandoer baggrundskoden sender under testen er med.
function fakeNativePort() {
  const port = {
    messageListeners: [],
    disconnectListeners: [],
    onMessage: { addListener: (fn) => port.messageListeners.push(fn) },
    onDisconnect: { addListener: (fn) => port.disconnectListeners.push(fn) },
    postMessage(msg) {
      const reply = { id: msg.id, ok: true };
      if (msg.cmd === "status") {
        reply.databases = [{ name: "Personal", unlocked: false, count: 0 }];
      }
      // Asynkront, som en rigtig port.
      queueMicrotask(() => port.messageListeners.forEach((fn) => fn(reply)));
    },
    disconnect() {},
  };
  return port;
}

// ------------------------------------------------------------- fake chrome
const store = new Map();
const messageListeners = [];
const omniboxListeners = { changed: [], entered: [] };
let defaultSuggestion = null;
const openedTabs = [];

const chrome = {
  runtime: {
    connectNative: () => fakeNativePort(),
    sendMessage: async () => {
      throw new Error("the background page must not send messages to itself");
    },
    onMessage: { addListener: (fn) => messageListeners.push(fn) },
  },
  storage: {
    session: {
      get: async (keys) => {
        const wanted = typeof keys === "string" ? [keys] : keys;
        const out = {};
        for (const k of wanted) if (store.has(k)) out[k] = store.get(k);
        return out;
      },
      set: async (items) => {
        for (const [k, v] of Object.entries(items)) store.set(k, v);
      },
      remove: async (keys) => {
        for (const k of typeof keys === "string" ? [keys] : keys) store.delete(k);
      },
    },
  },
  tabs: {
    create: async (props) => openedTabs.push(props),
    query: async () => [{ id: 1 }],
    update: async (id, props) => openedTabs.push({ id, ...props }),
  },
  omnibox: {
    setDefaultSuggestion: (s) => {
      defaultSuggestion = s;
    },
    onInputChanged: { addListener: (fn) => omniboxListeners.changed.push(fn) },
    onInputEntered: { addListener: (fn) => omniboxListeners.entered.push(fn) },
  },
};

// ------------------------------------------------------------------ indlæs
//
// Samme rækkefølge og samme delte globale scope som sw.js giver dem.
const context = vm.createContext({ chrome, console, setTimeout, clearTimeout, queueMicrotask, URL });
for (const file of ["compat.js", "search.js", "background.js"]) {
  const code = fs.readFileSync(path.join(UNPACKED, file), "utf8");
  vm.runInContext(code, context, { filename: file });
}

// ------------------------------------------------------------------- tjek
assert.equal(messageListeners.length, 1, "background.js must register exactly one message listener");
assert.ok(defaultSuggestion, "the omnibox needs a default suggestion, or the keyword looks broken");
assert.equal(omniboxListeners.changed.length, 1, "omnibox input listener missing");
assert.equal(omniboxListeners.entered.length, 1, "omnibox enter listener missing");

// Sådan kalder Chrome en lytter: med en sendResponse-funktion, og svaret
// kommer kun frem hvis lytteren returnerer true. Det er netop dét compat.js
// er til for — Firefox svarer ved at returnere et løfte.
function ask(message) {
  return new Promise((resolve, reject) => {
    const kept = messageListeners[0](message, {}, resolve);
    if (kept !== true) {
      reject(new Error(`listener returned ${kept}, so Chrome would close the channel before the answer`));
    }
  });
}

const status = await ask({ type: "status" });
assert.equal(status.ok, true, `status failed: ${status.message}`);
assert.deepEqual(
  status.databases.map((db) => db.name),
  ["Personal"],
  "the host's database list did not survive the round trip"
);

// En ukendt besked skal komme tilbage som en fejl-konvolut, ikke som tavshed.
const bogus = await ask({ type: "no-such-thing" });
assert.equal(bogus.ok, false);
assert.match(bogus.message, /unknown message/);

console.log("compat.js covers the shared code: status round trip and error envelope both work");
