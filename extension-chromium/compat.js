// SPDX-License-Identifier: GPL-3.0-or-later
//
// Chromium-skallen om `browser`-navnerummet.
//
// Al logik i denne udvidelse deles ordret med Firefox-udgaven i ../extension.
// Den kalder `browser.*`, som Chrome og Edge ikke definerer — de har kun
// `chrome.*`. Denne fil bygger `browser` af de kald der faktisk bruges, og
// retter undervejs den ene forskel der ikke er kosmetisk: Firefox lader en
// onMessage-lytter svare ved at returnere et løfte, Chromium gør ikke.
//
// Hvorfor en eksplicit liste og ikke `globalThis.browser = chrome`: aliasset
// alene virker for alt undtagen onMessage, og dét ville fejle stille — popup'en
// ville få `undefined` tilbage i stedet for et svar og vise "no answer from the
// extension background page", uden noget at forbinde det med. Retter man i
// stedet `chrome.runtime.onMessage.addListener` på plads, skriver man i en
// andens objekt. Listen her fejler højlydt i stedet: bruger den delte kode et
// kald jeg ikke har taget med, er det `undefined is not a function` første gang
// koden køres, ikke en mystisk tavshed i produktion.
//
// webextension-polyfill ville gøre det samme, men den er 30 kB tredjepartskode
// i en pakke der ellers kun indeholder vores egen — og butikkernes anmeldere
// læser al kildekode i pakken.

"use strict";

(() => {
  // Firefox definerer selv `browser`. Filen er med i Chromium-pakken alene,
  // men tjekket koster intet og gør den sikker at indlæse begge steder.
  if (typeof globalThis.browser !== "undefined") return;

  const c = globalThis.chrome;

  globalThis.browser = {
    runtime: {
      connectNative: (name) => c.runtime.connectNative(name),
      sendMessage: (message) => c.runtime.sendMessage(message),
      onMessage: {
        // Broen mellem de to svarmodeller. Returnerer lytteren et løfte,
        // holder vi kanalen åben med `return true` og sender svaret når
        // løftet indfries. Baggrundssiden pakker selv både succes og fejl i
        // en {ok, ...}-konvolut, så afvisningsgrenen er et sikkerhedsnet:
        // uden den ville en fejl her blive til tavshed, som er præcis det
        // konvolutten blev indført for at undgå.
        addListener(fn) {
          c.runtime.onMessage.addListener((message, sender, sendResponse) => {
            const out = fn(message, sender);
            if (!out || typeof out.then !== "function") return out;
            out.then(sendResponse, (err) =>
              sendResponse({
                ok: false,
                message: (err && err.message) || String(err),
              })
            );
            return true;
          });
        },
      },
    },

    storage: {
      // storage.session findes i Chromium fra 102 og holder, som i Firefox,
      // kun så længe browseren kører. Indekset må ikke overleve på disken.
      session: {
        get: (keys) => c.storage.session.get(keys),
        set: (items) => c.storage.session.set(items),
        remove: (keys) => c.storage.session.remove(keys),
      },
    },

    tabs: {
      create: (props) => c.tabs.create(props),
      query: (info) => c.tabs.query(info),
      update: (tabId, props) => c.tabs.update(tabId, props),
    },

    // omnibox har samme form begge steder, inklusive at suggest-funktionen
    // gerne må kaldes efter at lytteren er vendt tilbage.
    omnibox: c.omnibox,
  };
})();
