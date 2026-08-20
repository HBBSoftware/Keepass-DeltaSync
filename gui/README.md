# KeePass Delta-Sync — GUI

En lille **grafisk skal** oven på `keepass-deltasync`-kommandolinjeprogrammet.
Den hjælper en ny bruger i gang via en **guide** (tilmeld enhed → tilføj
database) og giver derefter et simpelt **dashboard** til at synkronisere
KeePass-databaser.

Programmet kører på **Windows, Linux og macOS** og er skrevet i Go med
[Fyne](https://fyne.io). Det er bevidst et **selvstændigt projekt** — det rører
hverken krypto, server eller config selv, men kalder CLI'en som en subproces,
præcis som projektets eksisterende terminal-menu (`keepass-deltasync tui`).

```
┌─────────────────────┐        os/exec        ┌──────────────────────────┐
│  keepass-deltasync   │ ───────────────────▶ │   keepass-deltasync(.exe) │
│        -gui          │   enroll / init /     │   (CLI — al krypto,       │
│  (dette projekt)     │   sync / databases …  │    server, config)        │
└─────────────────────┘                       └──────────────────────────┘
```

Fordelen ved denne opdeling: kryptokoden findes kun ét sted, GUI'en og CLI'en
kan udvikles og udgives uafhængigt, og licenserne blandes ikke sammen.

---

## 1. Forudsætninger

| Hvad | Hvorfor | Note |
|------|---------|------|
| **Go 1.23+** | Bygge GUI'en | `go version` |
| **En C-compiler** | Fyne kræver CGO | Se nedenfor per OS |
| **`keepass-deltasync`-CLI'en** | GUI'en kalder den | Byg fra `../Keepass-deltasync/client`, eller hent en release-binær |
| **`keepassxc-cli`** | Bruges af CLI'en til selve sync-merge | Følger med [KeePassXC](https://keepassxc.org) |

### C-compiler per OS

- **Windows:** en mingw-w64 gcc. Letteste vej:
  ```powershell
  winget install BrechtSanders.WinLibs.POSIX.UCRT
  ```
  (eller [w64devkit](https://github.com/skeeto/w64devkit) / MSYS2). Sørg for at
  `gcc` er i PATH, eller sæt `CC` til den fulde sti.
- **Linux:** `sudo apt install gcc pkg-config libgl1-mesa-dev xorg-dev`
  (Debian/Ubuntu) — tilsvarende `gl`- og `X11`-dev-pakker på andre distroer.
- **macOS:** `xcode-select --install` (Xcode Command Line Tools).

---

## 2. Byg

```sh
# I denne mappe (keepass-deltasync-gui)
go mod tidy
go build -o keepass-deltasync-gui .       # .exe på Windows
```

Eller med den medfølgende Makefile:

```sh
make build      # bygger til det aktuelle OS
make run        # bygger og kører
```

> **Windows-tip:** hvis `gcc` ikke er i PATH, så sæt den for byggekommandoen:
> ```powershell
> $env:CGO_ENABLED=1
> $env:CC="C:\sti\til\mingw64\bin\gcc.exe"
> go build -o keepass-deltasync-gui.exe .
> ```

### Pæne installerbare pakker (til release)

Brug Fyne's eget værktøj — det laver ikon, metadata og en rigtig `.app`/`.exe`.
Ikon, navn, id og version læses fra `FyneApp.toml`:

```sh
go install fyne.io/tools/cmd/fyne@latest
fyne package -os windows    # på Windows
fyne package -os darwin     # på macOS    → KeePass Delta-Sync.app
fyne package -os linux      # på Linux    → .tar.xz
```

Fyne navngiver Windows-filen efter visningsnavnet ("KeePass Delta-Sync.exe").
`make package-windows` omdøber den bagefter til **`keepass-deltasync-gui.exe`**
(så den matcher CLI'ens navn) uden at ændre det pæne navn i titellinje og
fil-egenskaber.

Ikonet (`icon.png`) er projektets fælles ikon — samme guldnøgle-med-sync-pile
som Android-klienten. De to lag (baggrund + motiv) ligger i
`tools/genicon/layers/`, og `go run ./tools/genicon` komponerer dem til
`icon.png` med afrundede hjørner.

Cross-compilering mellem OS'er er besværligt med CGO. Byg **nativt på hvert OS**,
eller brug [`fyne-cross`](https://github.com/fyne-io/fyne-cross) (Docker) til at
bygge til alle tre fra én maskine.

---

## 3. Hvordan GUI'en finder CLI'en

Ved opstart leder GUI'en efter `keepass-deltasync(.exe)` i denne rækkefølge:

1. En sti du selv har valgt (gemt i `gui.json`).
2. **Ved siden af GUI-programmet** — dette er den anbefalede måde at udgive på:
   læg de to binærer i samme mappe.
3. I systemets `PATH`.

Finder den ingenting, beder den dig pege på programmet. Du kan altid ændre stien
under **Indstillinger**.

GUI'ens egne præferencer (CLI-sti + sprog) ligger i:

- Windows: `%AppData%\keepass-deltasync-gui\gui.json`
- Linux: `~/.config/keepass-deltasync-gui/gui.json`
- macOS: `~/Library/Application Support/keepass-deltasync-gui/gui.json`

Klientens egentlige konfiguration (server-token, nøgler, database-bindinger)
ejes som altid af CLI'en og ligger i dens egen `config.toml`.

---

## 4. Sådan kommer en bruger i gang (guiden)

1. **Start programmet.** Er enheden ikke tilmeldt endnu, åbner guiden automatisk.
2. **Trin 1 — Tilmeld enhed:** indtast server-adressen og det enrollment-token
   du har fået af din administrator. (Svarer til `keepass-deltasync enroll`.)
3. **Trin 2 — Tilføj database:** giv din lokale `.kdbx` et kort navn og vælg
   filen. (Svarer til `keepass-deltasync init`.)
4. **Færdig.** Dashboardet viser dine databaser. Klik **Synkronisér** på en
   database, indtast dit masterpassword, og ændringer sendes/hentes.
   (Svarer til `keepass-deltasync sync <navn> --password-stdin` — passwordet
   sendes via stdin og optræder aldrig på kommandolinjen.)

Sproget kan skiftes mellem **dansk** og **engelsk** under Indstillinger.

---

## 5. Hvad GUI'en understøtter i dag

| Funktion | CLI-kommando bag |
|----------|------------------|
| Tilmeld enhed (guide) | `enroll` |
| Tilføj lokal database | `init` |
| Vis databaser + status | `databases`, `status` |
| Synkronisér én / alle | `sync --password-stdin` |
| Aktivitetslog (CLI-output) | — |

Endnu **ikke** i GUI'en (men findes i CLI'en — gode næste skridt):
`daemon` (automatisk baggrunds-sync), `share` / `unshare` / `shares`,
`init-shared`, `versions` / `restore`, `init --bind` for server-databaser der
kun vises som "kun på server".

---

## 6. Projektstruktur

| Fil | Ansvar |
|-----|--------|
| `main.go` | App-opstart, vindue, routing (guide vs. dashboard) |
| `cli.go` | Find + kald CLI'en, parse `status` / `databases` |
| `wizard.go` | Onboarding-guiden (tilmeld → tilføj database) |
| `dashboard.go` | Faneblade: databaser, aktivitet, indstillinger |
| `settings.go` | GUI'ens egne præferencer (`gui.json`) |
| `i18n.go` | Dansk/engelsk strenge |
| `helpers.go` | Async-kald på UI-tråd, log, småting |

## Licens

GPL-3.0-or-later — samme som `keepass-deltasync`-klienten, som den kalder.
