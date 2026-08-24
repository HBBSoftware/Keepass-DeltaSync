# Browser-host på Linux — testplan for snap, flatpak og pakket Firefox

Status pr. 2026-08-21. Skrevet til en Claude Code-session på en Linux-maskine.

`install-browser-host` blev 2026-08-21 lavet om fra **én** manifest-sti til
**én pr. Firefox-variant** (`hostTargets(exe)` i
`client/cmd/keepass-deltasync/browser_install_unix.go`). Baggrunden: kommandoen
skrev kun `~/.mozilla/native-messaging-hosts` og meldte succes uanset, så en
snap- eller flatpak-bruger fik "Installed browser host for Firefox" og en
udvidelse der stadig sagde *cannot start the native host* — uden noget at
forbinde de to med.

**Det der ER verificeret** (WSL Ubuntu-22.04, isoleret `HOME`, 2026-08-21):
detektion af de tre varianter, at manifest og launcher lander de rigtige
steder, at flatpak-launcheren går gennem `flatpak-spawn --host`, at gentagen
kørsel er idempotent, og at `uninstall-browser-host` fjerner alle tre sporløst.
Enhedstestene i `browser_install_unix_test.go` dækker det samme.

**Det der IKKE er verificeret — og som er hele grunden til denne plan:** om en
*indespærret* Firefox rent faktisk får lov at starte hosten. Både snap og
flatpak kører Firefox under en sikkerhedsprofil, og profilen kan spærre for at
eksekvere et program uden for sandkassen. WSL har ingen af delene.

---

## Før du starter

**1. Koden skal være pushet.** Arbejdet lå uforpligtet i arbejdstræet på
Windows-maskinen da denne plan blev skrevet. Er `git log` på Linux-maskinen
ikke forbi `c5cea73`, mangler der en `git push` fra Windows.

**2. Rør ikke en kørende daemon med en ældre binær.** Kører maskinen
`keepass-deltasync daemon` mod serveren, så har den binær formentlig ikke
`LocalOnly()`-spærren. En `add-local`-database i den *rigtige* `config.toml`
ville få den til at forsøge at synkronisere en database uden `remote_id` næste
gang den starter. To udveje — vælg én:

- opdatér daemon-binæren til denne kode først, **eller**
- hold hele testen i en separat config:

  ```sh
  export KEEPASS_DELTASYNC_CONFIG=/tmp/kpds-test/config.toml
  ```

  Firefox arver ikke din shells miljø, så launcher-scriptet skal have samme
  linje for at hosten ser den samme config. Se trin 3.

**3. Du skal bruge:** Go 1.26, KeePassXC (`keepassxc-cli` skal kunne findes —
en flatpak/snap-KeePassXC lægger den ikke i `PATH`), en `.kdbx` med mindst én
entry der har en URL, og mindst én af de tre Firefox-varianter.

---

## Varianterne og det åbne spørgsmål

| Variant | Manifest-mappe | Det uafklarede |
|---------|----------------|----------------|
| Pakket (distro/tarball) | `~/.mozilla/native-messaging-hosts/` | Intet — forventes at virke som hidtil |
| **snap** | `~/snap/firefox/common/.mozilla/native-messaging-hosts/` | Må AppArmor-profilen eksekvere launcheren, og må den derfra starte binæren og `keepassxc-cli`? |
| **flatpak** | `~/.var/app/org.mozilla.firefox/.mozilla/native-messaging-hosts/` | Virker `flatpak-spawn --host` med den `--talk-name`-override vi beder om? |

Snap er den mest sandsynlige at fejle: `home`-interfacet giver ikke adgang til
punktum-mapper i `$HOME`, og at eksekvere et vilkårligt program på værten er
ikke noget en strengt indespærret snap normalt må.

---

## Trin 0 — byg

```sh
cd client
go build -o keepass-deltasync ./cmd/keepass-deltasync
go test ./...            # skal være grøn, inkl. browser_install_unix_test.go
```

Læg binæren et **blivende** sted — stien bages ind i manifestet. Til
snap-testen skal den ligge uden for en punktum-mappe:

```sh
mkdir -p ~/bin && cp keepass-deltasync ~/bin/
```

## Trin 1 — registrér

```sh
~/bin/keepass-deltasync install-browser-host --dry-run   # se hvad der ville ske
~/bin/keepass-deltasync install-browser-host
```

Forventet: én blok pr. variant den fandt, hver med `launcher:` og `manifest:`,
og for snap/flatpak en `note:` med den variant-specifikke hage. Til sidst
`Installed browser host for N Firefox variant(s)`.

✅ **Tjek:** at N svarer til de Firefox'er du faktisk har. `snap list firefox`
og `flatpak list | grep -i firefox` afgør det. Er en variant installeret men
ikke nævnt, er detektionen forkert — det er et fund i sig selv.

Har du kun én variant, men vil teste de andres filplacering, tvinger `--all`
alle tre igennem.

## Trin 2 — giv hosten en database

```sh
~/bin/keepass-deltasync add-local test ~/sti/til/din.kdbx --save-password
~/bin/keepass-deltasync browser-host --probe test
```

`--probe` printer præcis det JSON udvidelsen ville få. Virker det ikke her, er
det ikke Firefox' skyld, og resten af planen er spildt.

`--save-password` lægger masterpasswordet i keyringen (Secret Service over
D-Bus). Er der ingen keyring på maskinen, så drop flaget — popup'en spørger i
stedet.

## Trin 3 — sæt config-stien ind i launcheren (kun hvis du bruger en testconfig)

Firefox' host arver ikke din shell. Kører du med
`KEEPASS_DELTASYNC_CONFIG`, så indsæt linjen i **hver** launcher
`install-browser-host` skrev, lige før `exec`:

```sh
#!/bin/sh
KEEPASS_DELTASYNC_CONFIG=/tmp/kpds-test/config.toml
export KEEPASS_DELTASYNC_CONFIG
exec "/home/dig/bin/keepass-deltasync" browser-host "$@"
```

En ny `install-browser-host` overskriver filen og fjerner linjen igen.

## Trin 4 — indlæs udvidelsen

Planen tester bygget i `extension/`, ikke det signerede fra AMO, så:
`about:debugging#/runtime/this-firefox` → *Load Temporary Add-on…* →
`extension/manifest.json`. ID'et ligger fast (`keepass-deltasync@hb-b.dk`), så
en allerede indlæst udgave — også en installeret fra
[addons.mozilla.org](https://addons.mozilla.org/firefox/addon/deltasync-keepass-search-go/)
— skal fjernes først. Gentag i hver Firefox-variant du tester.

## Trin 5 — det afgørende

Åbn popup'en (værktøjslinjen eller `Alt+Shift+K`) i hver variant.

| Du ser | Betydning |
|--------|-----------|
| Søgefeltet er aktivt og finder din entry | ✅ Varianten virker |
| *"The host is running, but no database is registered yet"* + knap | Hosten kører — trin 2 eller 3 ramte forbi |
| *"Firefox cannot reach the keepass-deltasync host"* + knap | ❌ Sandkassen spærrer, eller manifestet ligger forkert. Videre nedenfor |

---

## Fejlfinding, når hosten ikke starter

**Først: er det sandkassen eller os?** Kør launcheren i hånden — den er
ukonfineret dér, så lykkes det, ligger fejlen i indespærringen:

```sh
echo | ~/snap/firefox/common/keepass-deltasync/browser-host.sh
```

Forventet: en byte-suppe (længdeprefixet JSON), ikke `Permission denied`.

**Browser Console** (`Ctrl+Shift+J`) skiller de to fejltyper ad:

- *"No such native application dk.hbb.keepass_deltasync"* → Firefox fandt ikke
  manifestet. Forkert mappe for netop denne variant — sammenlign med tabellen
  ovenfor og med hvad trin 1 skrev.
- *"Error: An unexpected error occurred"* / hosten dør straks → manifestet blev
  fundet, men programmet kunne ikke køre. Det er sandkassen.

### snap

```sh
sudo journalctl -k --since "5 min ago" | grep -i -e apparmor -e denied
```

En `DENIED`-linje med `firefox` og stien til launcheren er svaret. Prøv i
rækkefølge:

1. Ligger binæren i en punktum-mappe (`~/.local/bin`)? Flyt til `~/bin` og kør
   `install-browser-host` igen.
2. Kom hosten i gang, men fejler den på `keepassxc-cli`? Så er det næste led
   der spærres — noter det, for så hjælper det ikke at flytte binæren.
3. Kør en kommando inde i selve indespærringen for at være sikker:

   ```sh
   snap run --shell firefox -c '~/snap/firefox/common/keepass-deltasync/browser-host.sh </dev/null'
   ```

### flatpak

```sh
flatpak override --user --show org.mozilla.firefox
```

`--talk-name=org.freedesktop.Flatpak` skal stå der. Mangler den:

```sh
flatpak override --user --talk-name=org.freedesktop.Flatpak org.mozilla.firefox
```

Genstart Firefox bagefter — en override slår ikke igennem på en kørende app.
Isolér portalen fra vores kode med:

```sh
flatpak run --command=sh org.mozilla.firefox -c 'flatpak-spawn --host echo ok'
```

Skriver den `ok`, virker vejen ud af sandkassen, og fejlen ligger hos os.

---

## Rapportér tilbage

Udfyld og læg svaret i denne fil (eller giv det videre):

| Variant | Distro + Firefox-version | Registreret? | Popup virker? | Hvad skulle der til |
|---------|--------------------------|--------------|---------------|---------------------|
| pakket  | Manjaro (6.18.42), Firefox 153.0.3 | ja | **ja** — søgning fandt "Github hbbsoft" | intet, virkede i første forsøg |
| snap    | — | — | — | snapd findes ikke på maskinen; ikke testet |
| flatpak | — | **ja — men der er ingen flatpak-Firefox** | ikke testbart | se fundet nedenfor |

`keepassxc-cli` blev fundet af sig selv (`/usr/bin/keepassxc-cli`). Keyringen
virkede: `ksecretd` leverer `org.freedesktop.secrets` på KDE, `--save-password`
gik igennem uden fejl, og hverken `--probe` eller `unlock` over launcheren
spurgte om noget.

### Kørsel 2026-08-22 (Manjaro, KDE)

`go test ./...` grøn. Ud over popup-testen blev launcheren kørt i hånden med
rigtige native-messaging-rammer, altså gennem præcis den sti Firefox bruger:

- `status` → 3 databaser
- `unlock` **uden** password-felt → `ok`, keyringen leverede det
- `index` → 579 entries

Så alt på vores side af Firefox er verificeret uafhængigt af browseren.
Gentagen `install-browser-host` var idempotent (uændrede checksums).

### Fund: flatpak-detektionen giver falsk positiv

`hostTargets` afgør flatpak på `dirExists(~/.var/app/org.mozilla.firefox)`
(`browser_install_unix.go:84`). Den mappe beviser ikke at Firefox er
installeret:

- `flatpak uninstall` sletter ikke brugerdata uden `--delete-data`, så mappen
  overlever afinstallation
- på KDE lægger `plasma-browser-integration` uopfordret sin egen
  native-messaging-host i den mappe

På denne maskine er `flatpak list --app` **helt tom**, mens `~/.var/app/` har 30
efterladte mapper — og `install-browser-host` meldte alligevel
`Installed browser host for 2 Firefox variant(s)` og bad brugeren køre en
`flatpak override` for en app de ikke har. Det er den samme klasse af
tavs-uoverensstemmelse som planen selv blev skrevet for at komme til livs, blot
med modsat fortegn.

Snap-grenen (`dirExists(~/snap/firefox)`, linje 66) har samme form: `snap
remove` efterlader `~/snap/<navn>`.

Et pålideligt signal er deploy-mappen frem for datamappen — verificeret på
denne maskine, hvor residue-mappen findes og alle deploy-stier mangler:

| | flatpak | snap |
|---|---|---|
| I dag (residue) | `~/.var/app/org.mozilla.firefox` | `~/snap/firefox` |
| Foreslået | `/var/lib/flatpak/app/org.mozilla.firefox` eller `~/.local/share/flatpak/app/org.mozilla.firefox` | `/snap/firefox` |

Manifestet skal fortsat *skrives* i datamappen — det er kun detektionen der
skal se et andet sted hen. `--all` beholder muligheden for at tvinge alle tre
igennem.

## Hvis en variant ikke kan lade sig gøre

Så er det et dokumentationsspørgsmål, ikke et kodespørgsmål — og det skal siges
ligeud frem for at brugeren selv opdager det:

1. `docs/install-browser.md` → afsnittet **"Linux: there is more than one
   Firefox"** skal sige det rent ud, og pege på en pakket Firefox som udvejen.
2. `hostTarget.Hint` for varianten (i `browser_install_unix.go`) skal sige det
   samme, så `install-browser-host` selv advarer i stedet for at lade som om.
3. Overvej om varianten helt skal droppes fra `hostTargets` — et manifest der
   aldrig kan virke, er værre end intet, fordi det giver Firefox noget at
   fejle på.
4. `CHANGELOG.md` under *Unreleased* og hjemmesidens `firefox.html` skal
   følge med.

Virker alle tre, så stryg forbeholdet: `docs/browser-extension.md` punkt 8
siger i dag at flatpak/snap-vejen er skrevet efter dokumentationen, ikke
afprøvet.

## Ryd op

```sh
~/bin/keepass-deltasync forget test              # fjerner også keyring-posten
~/bin/keepass-deltasync uninstall-browser-host   # rydder alle varianter
```

Ingen af delene rører din `.kdbx`.
