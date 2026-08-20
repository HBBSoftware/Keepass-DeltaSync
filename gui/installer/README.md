# Samlet Windows-installer

`build.ps1` bygger **én** `setup.exe` der installerer både GUI'en og
kommandolinje-klienten (`keepass-deltasync.exe`) i **samme mappe**. Det er dét
der binder dem sammen: GUI'en er en tynd wrapper der finder CLI'en ved siden af
sig selv (`locateCLI` i `cli.go`) — ingen PATH-opsætning nødvendig.

## Byg

```powershell
cd installer
pwsh -File build.ps1
# → out\KeePass-Delta-Sync-Setup-<version>.exe
```

Forudsætninger på byggemaskinen:

- **Go** + **`fyne`**-værktøjet (`go install fyne.io/tools/cmd/fyne@latest`)
- en **gcc (mingw-w64)** til CGO (Fyne kræver CGO) — stien sættes i `build.ps1`
- **Inno Setup 6** (`ISCC.exe`)
- søster-repo'et `Keepass-deltasync` klonet ved siden af dette (til CLI'en +
  dens kildekode-zip)

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
