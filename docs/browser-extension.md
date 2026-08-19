# Firefox-udvidelse — søg og gå til URL

Status: **IMPLEMENTERET** (fase 1–5, 2026-08-19). Desktop-klienten har
`browser-host`, `install-browser-host` og `uninstall-browser-host`; udvidelsen
ligger i `extension/`. Mangler: signering/distribution (fase 6 og de åbne
spørgsmål nedenfor) samt en live-test mod en rigtig database i Firefox.

## Mål

Fritekstsøgning i entries fra Firefox' adresselinje eller en popup → naviger til
entry'ens URL. Intet andet.

**Udfyldning af credentials er eksplicit ikke-mål.** Når fanen er landet på den
rigtige side, overtager KeePassXC-Browser, som allerede gør det arbejde godt. Vi
konkurrerer ikke med den; vi leverer den manglende "hvor var det nu, den entry
lå"-navigation.

Konsekvensen er hele designets omdrejningspunkt: **udvidelsen behøver kun titel
og URL, aldrig hemmeligheder.** Det gør trusselsmodellen billig og tillader et
manifest uden en eneste host permission.

Ikke-mål i denne omgang: autofill, TOTP, oprettelse/redigering af entries,
visning af passwords, Chrome-support, Firefox for Android.

## Grundlag der genbruges

Næsten alt findes allerede i desktop-klienten:

- **`kdbx.CLI.Export`** (`client/internal/kdbx/cli.go`) — `keepassxc-cli export`
  til XML. Ingen ny kdbx-parser, ingen ny dependency; `keepassxc-cli` er i
  forvejen hård dependency for desktop.
- **`kdbx.ParseExport`** (`client/internal/kdbx/xml.go`) — entries + grupper ud
  af eksporten. Udelader allerede Root-gruppen og hele papirkurv-undertræet
  (`activeRecycleBinUUID`), så den filtrering er gratis og korrekt.
- **Entry-fragmentets `<String>`-felter** — de ekstra URL'er bor i
  custom-felter, så indekseringen skal læse dem. Se dog forbeholdet under
  "Indekset": `canonical.FromInnerXML` viste sig at være for streng til
  formålet.
- **`internal/keyring`** — masterpassword pr. `RemoteID` i OS' credential store.
  Afgørende for unlock-modellen nedenfor.
- **`config.Database`** — `Name`, `LocalPath`, `RemoteID`. Hosten indekserer på
  tværs af alle registrerede databaser.
- **fsnotify + debounce** — samme mekanik som `daemon`-kommandoen bruger til at
  opdage lokale kdbx-ændringer.

## Arkitektur

```
Firefox-udvidelse  ──native messaging (stdio)──>  keepass-deltasync browser-host
                                                            │
                                                     keepassxc-cli export
                                                            │
                                                        <db>.kdbx
```

Hosten er en **ny subkommando i den eksisterende binær**, ikke en separat
binær: `keepass-deltasync browser-host`. Samme config, samme keyring, samme
release-artefakter, ét installationstrin for brugeren.

### Unlock — passwordet rører aldrig browseren

Fordi keyringen allerede findes, kan browseren holdes helt uden for
hemmeligheden:

1. Brugeren klikker "Lås op" i popup'en → udvidelsen sender `unlock` **uden
   password**.
2. Hosten kalder `keyring.Get(RemoteID)`. På macOS og Linux kan OS'et vise sin
   egen godkendelsesdialog — den hører til OS'et, ikke til browseren.
3. Hosten kører `Export` → `ParseExport` → felt-udtræk, bygger indekset, og
   **zeroer XML-bufferen og passwordet med det samme**.
4. Hosten svarer med indekset og holder derefter intet hemmeligt overhovedet.

**Fallback** for databaser uden keyring-entry: popup'en beder om passwordet og
sender det over pipen. Hosten holder det i egen hukommelse så længe
forbindelsen lever, med idle-lock (forslag: 15 min), og zeroer ved `lock` eller
exit. Passwordet må aldrig røre `storage.local` og skal slettes fra udvidelsens
hukommelse straks efter afsendelse.

Bemærk hvad keyringen køber os: **re-indeksering koster ingen ny hemmelighed.**
Efter en sync kan hosten bare hente fra keyringen igen. Havde vi cachet nøglen i
stedet, skulle vi have holdt en langlivet hemmelighed alene for at holde
indekset friskt.

### Indekset

Pr. entry: `uuid`, `title`, `urls[]`, `group_path`, `db` (databasenavn).

URL-kilder — standard-`URL`-feltet er ikke nok:

- `URL` (standardfeltet)
- custom-felter `KP2A_URL_*` (KeePass2Android)
- custom-felter `Additional URL*` (KeePassXC's Additional URLs)

`urls` er **altid et array på wiren**, også når det er tomt. Det lyder som en
detalje, men en nil-slice i Go serialiseres til JSON-`null`, og udvidelsen
itererer over feltet — og godt hver fjerde entry i en rigtig database har ingen
navigerbar URL. En test på `len(urls) == 0` fanger det ikke, for den er sand
for både nil og en tom slice; det skal testes på serialiseringen.

Værdier der **skal afvises** før de kan ende i `tabs.update`: tom streng,
`{REF:...}`-referencer, `{S:felt}`-placeholders, `cmd://`, og alt der ikke
parser som `http`/`https`. En entry uden en eneste brugbar URL indekseres
stadig (den kan findes), men markeres som ikke-navigerbar i UI'et.

Filtrering af skjulte entries:

- **Papirkurv** — gratis, `ParseExport` udelader allerede undertræet.
- **Søge-deaktiverede grupper** (`EnableSearching=false`) — `xml.go` læste
  ikke feltet (kun `staging.go` skrev det, som `null`), så det blev tilføjet:
  `kdbx.Group.EnableSearching *bool`, hvor nil betyder "arv fra forælder".
  Uden det ville entries, brugeren bevidst har skjult, dukke op i søgningen —
  nøjagtig samme fælde som `kotpass`' `findEntries` gav på Android.

### Hvorfor ikke `canonical.FromInnerXML`

Den oplagte genbrug var canonical-parseren, men den er bygget til sync og
kræver derfor et komplet, gyldigt entry — blandt andet alle fire tidsstempler.
En entry med et manglende `CreationTime` ville dermed forsvinde lydløst fra
søgningen, selvom dens titel og URL er helt i orden. **Søgning skal være
lempelig, hvor sync skal være streng**, så indekseringen fik sin egen 20-liners
parser, der kun læser `<String>`-felter. Den holder samtidig
sikkerhedsgrænsen skarpere: vi materialiserer aldrig et fuldt `Entry` med
password og attachments i hukommelsen, kun de felter vi skal bruge.

### Protokol

Firefox' native messaging: 4-byte native-endian længdeprefix + UTF-8 JSON.

**Beskedgrænsen går den modsatte vej af hvad man skulle tro:** Firefox
tillader op til 4 GB fra udvidelsen til applikationen, men højst **1 MB fra
applikationen til udvidelsen**. Det er den retning indekset skal — så et
indeks kan ikke sendes i én besked. `unlock` returnerer derfor kun et antal,
og udvidelsen henter selve indekset side for side med `index`. Hosten holder
indekset i hukommelsen mellem siderne; det er metadata, ikke nøglemateriale.

| Retning | Besked | Svar |
|---|---|---|
| → | `{"cmd":"status"}` | registrerede databaser, og hvilke der er indekseret |
| → | `{"cmd":"unlock","db":"privat"}` | `{"ok":true,"count":412,"generation":17}` |
| → | `{"cmd":"unlock","db":"privat","password":"…"}` | fallback uden keyring |
| → | `{"cmd":"index","db":"privat","offset":0}` | `{"entries":[…],"next":180,"count":412}` |
| → | `{"cmd":"lock"}` | zeroer cachet password **og** indeks |
| ← | `{"event":"changed","db":"privat"}` | fsnotify på .kdbx, debounced 2 s |

Fejler `unlock` fordi der ikke er nogen keyring-entry, svarer hosten med
`need_password: true` frem for en generisk fejl. Det er signalet til
udvidelsen om at vise password-feltet — og dermed det eneste sted browseren
overhovedet kommer i berøring med masterpasswordet.

`changed`-eventen er gevinsten ved at høre sammen med resten af projektet:
efter en sync kan indekset friskes uden brugerhandling. KeePassXC-Browser kan
ikke det samme.

Søgningen selv kører i udvidelsen på det cachede indeks — det er instant og
holder hosten enkel. Host-side `search` tilføjes kun, hvis meget store
databaser viser sig at være et problem.

## Sikkerhedsgrænser

Normative krav, ikke anbefalinger:

1. **Hosten serialiserer aldrig andet end whitelisten** `{uuid, title, urls,
   group_path, db}`. Implementeres som en eksplicit allow-list i
   serialiseringen — ikke som "vi undlader at sætte password". En fremtidig bug
   i JS-koden må ikke kunne lække et felt, hosten aldrig sendte.
2. **Ingen host permissions i manifestet.** `tabs.create`/`tabs.update` kræver
   ingen. Nødvendige permissions: `nativeMessaging`, `storage`, `omnibox`,
   `commands`. Ingen `<all_urls>`, intet content script.
3. **Indekset ligger i `storage.session`** (kun RAM, ryddes ved browserluk).
   Aldrig `storage.local` — det ville skrive en klartekstliste over alle
   brugerens URL'er til disk.
4. **`allowed_extensions` i native-manifestet** begrænser hosten til vores
   extension-ID; andre udvidelser kan ikke tale med den.
5. Indekset **er** følsom metadata, selv uden passwords. En liste over alle ens
   URL'er siger meget. Derfor session-only, en eksplicit `lock`-kommando, og
   idle-lock i fallback-tilstanden.

`docs/threat-model.md` skal udvides med denne nye lokale angrebsflade.

## Installation

Firefox' native-manifest kan pege på ét program, men kan **ikke** sende
argumenter med — og vores host er en subkommando. `install-browser-host`
skriver derfor et lille launcher-script (`browser-host.bat` på Windows,
`browser-host.sh` ellers), som manifestet peger på. Det er samme mønster som
Mozillas egen dokumentation bruger.

`keepass-deltasync install-browser-host` skriver native-manifestet:

- Linux: `~/.mozilla/native-messaging-hosts/dk.hbb.keepass_deltasync.json`
- macOS: `~/Library/Application Support/Mozilla/NativeMessagingHosts/…`
- Windows: registry-værdi under
  `HKCU\Software\Mozilla\NativeMessagingHosts\dk.hbb.keepass_deltasync`, der
  peger på JSON-filen

Host-navnet skal matche `[a-z0-9_.]+`. `uninstall-browser-host` fjerner det
igen. Manifestet indeholder den absolutte sti til binæren — kommandoen skal
derfor køres igen, hvis binæren flyttes, og bør sige det.

## Ikon

`extension/icon.svg` er DeltaSync-mærket, med koordinater og farver kopieret
1:1 fra `android/app/src/main/res/drawable/ic_launcher_foreground.xml`, som
selv spejler `logo.svg` fra hjemmesiden. De tre skal følges ad — ændrer nogen
mærket ét sted, skal de andre med. Browserversionen tegner baggrunden med som
en afrundet flade, fordi en browser ikke maskerer ikonet, sådan som Androids
adaptive-icon gør.

## UX

- **omnibox-keyword `kp `** — skriv `kp gmail` i adresselinjen, se forslag,
  Enter. Bedste pasform i Firefox; én keyword pr. udvidelse.
- **Popup** med søgefelt for dem, der vil se en liste, plus en `commands`-genvej.
- **Rangering**: host-match > titel-prefix > titel-substring > gruppe/tag.
- Enter = samme fane, Ctrl+Enter eller midterklik = ny fane.
- **Flere URL'er pr. entry**: et søgeresultat er et `(entry, url)`-par, ikke en
  entry. Alle adresser er søgbare, og en entry optræder med en række pr.
  adresse der matchede — op til tre. Det var den oprindelige mangel: begge
  adresser blev fundet, men kun den højest rangerede kunne nås.

  Den primære adresse er altid med, også når det var en anden der matchede;
  den er entry'ens hovedindgang. Bar titlen matchet, ligger den primære
  øverst, for så peger søgningen på entry'en som helhed frem for på én
  bestemt adresse. Ramte en konkret adresse hårdere end titlen, er det den,
  brugeren ledte efter, og den vinder.

  Eksempel fra en rigtig database: entry'en `halmbox.localhost` har både
  `http://halmbox.localhost/login.php` og
  `https://office.halmbox.dk/login.php`. `halmbox` viser begge med
  localhost-adressen først (værtsnavnet starter med søgeordet); `office`
  vender rækkefølgen.

## Faser

1. ✅ `browser-host`-subkommando: `status`, `unlock`, `index`, `lock` +
   indeksbygning. `--probe <db>` er stdio-harnesset, der printer indekset som
   almindelig JSON uden browser i spillet.
2. ✅ Udvidelsen i `extension/`: popup + søgning i cachet indeks + navigation.
3. ✅ `install-browser-host` / `uninstall-browser-host` på Linux, macOS og
   Windows (registry-nøgle under HKCU på sidstnævnte).
4. ✅ omnibox-keyword `kp` + Alt+Shift+K.
5. ✅ `changed`-push via fsnotify, debounced.
6. Udestår: signering og distribution, Chromium-manifest, Firefox for Android.

Alt undtagen fase 6 er bygget. Live-test mod en rigtig database i Firefox
mangler stadig.

## Åbne spørgsmål

1. **Distribution** — AMO-signering, eller selvhostet signeret XPI som på
   Obtainium-sporet for Android? AMO giver auto-opdatering gratis, men
   review-kø; extension-ID'et skal ligge fast før fase 3, fordi det står i
   native-manifestet.
2. **Flere databaser** — ét fælles søgeresultat med db-badge, eller vælg én ad
   gangen? Forslag: ét fælles.
3. **Idle-lock i fallback-tilstand** — 15 min, eller følge en eksisterende
   indstilling?
4. ~~**`EnableSearching`**~~ — **afgjort under implementeringen.**
   `kdbx.Group` fik et `EnableSearching *bool`-felt (nil = arv), som
   `ParseExport` udfylder, men *ikke* handler på. Politikken ligger i
   browser-host's indeksering alene, så sync-adfærden er uændret.
