// SPDX-License-Identifier: GPL-3.0-or-later
//
// Service worker-indgangen. Firefox kører baggrundskoden som en event page og
// får filerne listet i manifestets `scripts`; Chromium kræver én service
// worker og henter resten herfra.
//
// importScripts kører synkront, så alle lyttere i background.js er registreret
// inden denne fil er færdig. Det er et krav i Chromium: en lytter der bliver
// tilføjet senere — i et løfte, efter et await — ser aldrig den hændelse der
// vækkede workeren, og udvidelsen svarer så kun hver anden gang.
//
// Rækkefølgen er den samme som manifestets i ../extension: skallen først, så
// søgningen, som background.js kalder ind i.

"use strict";

importScripts("compat.js", "search.js", "background.js");
