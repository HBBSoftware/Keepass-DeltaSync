# Samlet Windows-installer

`build.ps1` bygger **én** `setup.exe` der installerer både GUI'en og
kommandolinje-klienten (`keepass-deltasync.exe`) i **samme mappe**. Det er dét
der binder dem sammen: GUI'en er en tynd wrapper der finder CLI'en ved siden af
sig selv (`locateCLI` i `cli.go`) — ingen PATH-opsætning nødvendig.

## Byg

Til en **release** behøver du ikke gøre noget: `gui/vX.Y.Z`-taggen får CI til at
bygge installeren (`build:installer-stage` + `build:installer` i
`.gitlab-ci.yml`) og hæfte den på GitLab-releasen. `build.ps1` nedenfor er den
lokale vej — til at prøve en ændring af selve installeren af, eller til at lave
en installer uden for et tag.

Vil du arbejde på CI-jobbene i stedet, så skub til en `ci/gui-*`-branch; de tre
GUI-jobs har den escape hatch, præcis som Android- og extension-jobbene, så du
ikke skal flytte et offentliggjort tag for at afprøve en pipeline.

```powershell
cd gui\installer
pwsh -File build.ps1
# → out\KeePass-Delta-Sync-Setup-<version>.exe
```

Forudsætninger på byggemaskinen:

- **Go** + **`fyne`**-værktøjet (`go install fyne.io/tools/cmd/fyne@latest`)
- en **gcc (mingw-w64)** til CGO (Fyne kræver CGO) — stien sættes i `build.ps1`
- **Inno Setup 6** (`ISCC.exe`)

CLI'en tages fra `client/` i samme checkout — der er ikke længere noget
søster-repo at klone ved siden af.

Versionen tages automatisk fra `FyneApp.toml` (GUI) og seneste `client/v*`-tag
(CLI).

## Hvad installeren gør

- **Komponent "Program"** (påkrævet): `keepass-deltasync-gui.exe` +
  `keepass-deltasync.exe` + `LICENSE.txt` i installationsmappen.
- **Komponent "Kildekode"** (valgfri, fravalgt som standard): et zip-arkiv af
  hvert projekt under `source\`.
- Start-menu-genvej (+ valgfri skrivebordsgenvej), afinstallation, per-bruger
  uden administrator.

`keepass-deltasync.iss` er selve Inno Setup-scriptet; det tager sti/version ind
via `/D`-defines fra `build.ps1`.
