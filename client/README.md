# keepass-deltasync-client

Desktop-klienten: sync-agent, daemon, terminalmenu og den native host bag
Firefox-udvidelsen. Kører på **Linux, Windows og macOS**.

- **Sprog:** Go (se `go.mod` for versionen)
- **Licens:** GPL-3.0-or-later (matcher KeePassXC's licensfamilie)
- **Status:** I drift. Al kryptering, al servertrafik og hele sync-cyklussen
  ligger her — GUI'en (`../gui`) og terminalmenuen er skaller oven på de samme
  kommandoer, og Android-appen bruger `mobile/` gennem gomobile.

Klienten rører aldrig `.kdbx`-formatet selv. Den kalder `keepassxc-cli` til at
eksportere, importere og merge, og laver krypteringen på det der kommer ud.
Serveren ser kun blobs; masterpasswordet forlader aldrig maskinen.

## Forudsætninger

| Hvad | Hvorfor |
|------|---------|
| **KeePassXC** | `keepassxc-cli` åbner og skriver databasen. Findes automatisk i `PATH`, ellers via `$KEEPASSXC_CLI` eller `--keepassxc-cli` |
| **En Delta-Sync-server** | Kun til sync. Søgning fra Firefox virker uden — se `add-local` nedenfor |

## Byg

```sh
cd client
go build ./cmd/keepass-deltasync
go test ./...
```

Ingen cgo, så krydskompilering er ren `GOOS`/`GOARCH`. CI bygger
`linux/amd64`, `darwin/amd64`, `darwin/arm64` og `windows/amd64` på et
`client/vX.Y.Z`-tag og hænger dem på Releases-siden.

## Kommandoer

```
enroll <enrollment-token>   Tilmeld denne enhed til serveren
init <navn> <local.kdbx>    Registrér en lokal .kdbx til sync
add-local <navn> <sti>      Registrér en .kdbx til søgning alene (ingen server)
init-shared <remote> <sti>  Opret en lokal .kdbx for en database delt med dig
forget <navn>               Fjern en lokal binding (server og fil røres ikke)
delete-database <navn|uuid> Slet en database på serveren permanent (kun ejer)

push <navn>                 Upload alle entries fra en lokal .kdbx
pull <navn>                 Hent server-entries og merge ind i den lokale .kdbx
sync <navn>                 Pull, derefter push af det ændrede siden sidst
daemon                      Sync-løkke i forgrunden: fsnotify + polling

versions <navn> <uuid>      Vis serverens gemte versioner af en entry
restore <navn> <uuid> <n>   Rul en entry tilbage til version n (1, 2 eller 3)

share <navn> <bruger>       Del en ejet database med en anden bruger
unshare <navn> <bruger>     Fjern et medlem (eller dig selv) fra en database
shares <navn>               Vis medlemmer af en database (kun ejer)

status                      Vis enrollment og sidst-set
devices                     Vis tilmeldte enheder (`devices remove <id>` spærrer én)
databases                   Vis registrerede databaser (lokale + server)
log                         Vis kontoens seneste audit-log

browser-host                Native messaging-host til Firefox-udvidelsen
install-browser-host        Registrér hosten hos hver Firefox på maskinen
uninstall-browser-host      Fjern registreringen igen

tui                         Fuldskærmsmenu over kommandoerne ovenfor
admin <subkommando>         Brugeradministration (kræver admin-token)
```

`keepass-deltasync <kommando> -h` viser flagene for den enkelte kommando.

## Kom i gang

**Med server:**

```sh
keepass-deltasync enroll --server https://din-server.dk <token>
keepass-deltasync init privat ~/keepass/privat.kdbx
keepass-deltasync sync privat
```

**Uden server** — kun søgning fra Firefox:

```sh
keepass-deltasync add-local privat ~/keepass/privat.kdbx --save-password
keepass-deltasync install-browser-host
```

En sådan binding får intet `remote_id`, og alt der taler med serveren afviser
den frem for at sende en tom UUID afsted. Hele opsætningen står i
[`docs/install-browser.md`](../docs/install-browser.md); `tui` har den samme
under **Firefox-søgning**, også før man er tilmeldt.

## Config og hemmeligheder

`config.toml` ligger i OS'ets konventionelle config-mappe under
`keepass-deltasync/` — `%AppData%` på Windows, `~/.config` på Linux,
`~/Library/Application Support` på macOS. `$KEEPASS_DELTASYNC_CONFIG` overrider
hele stien, hvilket er vejen til at køre en test-opsætning ved siden af sin
rigtige.

Filen indeholder server-URL, device-token, enhedens private X25519-nøgle til
v2-deling, og én blok pr. registreret database. **Masterpasswords står der
ikke** — de ligger i OS'ets nøglering (Credential Manager, Keychain, Secret
Service), nøglet på databasens id, og hentes derfra af daemonen og af
browser-hosten.

## Undermapper

- `cmd/keepass-deltasync/` — kommandoerne, én fil pr. stykke
- `internal/api` — HTTP-klient mod serveren
- `internal/crypto` — Argon2id-nøgleafledning, HKDF pr. entry, sealed-box-deling
- `internal/kdbx` — `keepassxc-cli`-kald og det kanoniske entry/gruppe-format
- `internal/config`, `internal/keyring`, `internal/passwd` — config, nøglering, promptning
- `mobile/` — gomobile-bindinger; Android-appens krypto er den samme kode

Designet ligger i [spec'en](../keepass-deltasync-spec.md#klient-komponent--sync-agent)
og i `docs/` — se især `v3-canonical-entry-format.md`, `v4-group-sync.md` og
`browser-extension.md`.
