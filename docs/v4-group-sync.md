# v4 — Gruppe-synkronisering

Status: **DESIGN / forslag** (besluttet retning 2026-06-22). Spænder over server
(PHP), desktop-klient (Go) og Android (Kotlin).

## Mål

Synkronisér KDBX' **gruppestruktur** og **entry-gruppe-tilhørsforhold**, ikke kun
entries. Efter v4 skal det at flytte en entry til en anden gruppe, omdøbe en
gruppe, oprette/slette en gruppe og flytte en gruppe forplante sig til alle
enheder.

I dag (v3) er sync rent entry-baseret nøglet på UUID; grupper flades væk
(`collectEntries` i `client/internal/kdbx/xml.go` og Android'
`KotpassLocalStateAdapter`), og nye entries lander i en `deltasync`-gruppe
(desktop) / Root (Android). `canonical.Entry` har intet parent-group-felt.

## Grundlag der genbruges

- **Server** = nul-viden blob-store: `entries(database_id, entry_uuid)` +
  `entry_versions(blob, modified_at, deleted, server_seq, version_num)`. Blob er
  klient-krypteret og opaque. Monotonisk `server_seq` pr. database driver
  delta-pull. Max 3 versioner pr. objekt (rotation i migration 005).
- **Blob-format** (`canonical/envelope.go`): magic-byte `0x01` + JSON af
  `canonical.Entry`. `DetectFormat` peeker byte 0.
- **Merge**: Desktop delegerer til `keepassxc-cli merge`, som **allerede er
  gruppe-bevidst** (merger grupper by-UUID + `LocationChanged`). Android merger
  selv i `applyToDatabase`.

Den vigtige hævestang: **keepassxc-cli kan allerede gruppe-merge** — desktop
bliver derfor billig. Android er hovedarbejdet.

## Datamodel

### Ny `canonical.Group`
```
Group {
  v                int        // SchemaVersion
  uuid             string     // 8-4-4-4-12 hex
  name             string
  notes            string
  icon_id          int
  custom_icon_uuid string     // valgfri
  parent_group     string     // se "Root-sentinel"
  times {
    created, modified, location_changed  // UTC
  }
  // evt. senere: default_autotype_sequence, enable_searching, expiry
}
```

### Nyt felt på `canonical.Entry`
`parent_group string` — UUID på entry'ens gruppe (sentinel = Root).

### Root-sentinel (vigtigste faldgrube)
Hver enheds `.kdbx` har sin **egen** Root-gruppe-UUID — Device A's Root ≠ Device
B's Root. Derfor kodes "ligger i roden" som en **sentinel**: tom string `""`
(alternativt null-UUID `00000000-0000-0000-0000-000000000000`). Ved apply mapper
hver klient sentinel → sin egen lokale Root-UUID (`RootGroupUUID` findes allerede
i `xml.go`). Uden dette peger objekter på en gruppe-UUID, der ikke findes på
modtageren.

### Envelope / skemaversion
- **Entry-`SchemaVersion` bumpes IKKE.** `parent_group` er additivt (`omitempty`)
  — en v1-klient ignorerer bare det ukendte JSON-felt. Bumpede vi versionen,
  ville `DecodeCanonical`'s `e.V > SchemaVersion`-tjek få gamle klienter til at
  *afvise* nye entries. En tom `parent_group` udelades helt, så v1-blobs er
  byte-identiske med før.
- Grupper er et **selvstændigt object-kind** med egen envelope-magic-byte `0x02`
  (`formatByteGroup`) og egen `GroupSchemaVersion` (starter på 1). `DetectFormat`:
  `0x01`=entry-canonical, `0x02`=group-canonical, `<`=legacy-XML. Server-side
  filtrering (fase 1) sikrer at v1-klienter aldrig modtager `0x02`-blobs.

## Server

Grupper er bare endnu en slags krypteret blob. **Valgt: 2a — `object_kind`-kolonne**
(mindst kode, gratis seq-ordning på tværs af entries+grupper):

- Tilføj `object_kind SMALLINT NOT NULL DEFAULT 1` (1=entry, 2=group) til
  `entries` og `entry_versions` (ny migration 008).
- `server_seq` forbliver én strøm pr. database → en gruppe kan altid pull'es i
  rigtig rækkefølge ift. de entries, der peger på den.

### Forward-compat = server-side filtrering (BLOKERENDE, men ren)
Allerede udrullede v3-klienter må **ikke** modtage gruppe-blobs (de ville fejle i
`DetectFormat`). Løsning uden at røre gamle klienter:

- `GET /changes` returnerer **kun `object_kind=entry`** medmindre klienten
  eksplicit opt-in'er via fx `?include=groups` (eller en API-version-header).
- Gamle klienter sender ikke flaget → får kun entries → bryder aldrig. De
  avancerer stadig deres cursor til `current_seq` (high-water mark), så de
  re-fetcher ikke de oversprungne gruppe-seqs. Korrekt.
- Nye klienter sender flaget → får entries + grupper.

Dette betyder, at **server-ændringen kan udrulles uafhængigt og først**, uden en
koordineret klient-opdatering.

## Klient-merge

### Desktop (Go) — billigt
- `collectEntries` (`xml.go`) skal nu også emittere **grupper** (navn, parent,
  tider), ikke kun entries. Ny `Group`-walk.
- Push: send gruppe-objekter som `object_kind=group`-blobs (envelope `0x02`).
- Pull/merge (`syncop.go` `pullChanges` + `staging.go` `BuildStagingXML`):
  genopbyg det **rigtige gruppetræ** i staging-kdbx'en fra de synkede grupper, og
  placér entries i deres `parent_group`. Så laver `keepassxc-cli merge`
  gruppe-merge **for os** (by-UUID + `LocationChanged`). Erstatter den flade
  `deltasync`/Root-staging.

### Android (Kotlin) — hovedarbejdet
`applyToDatabase` (`KotpassLocalStateAdapter`) skal håndtere grupper eksplicit:
1. Opret manglende grupper (`modifyParentGroup`), respektér parent-kæden
   (topologisk: parent før barn).
2. Omdøb/flyt eksisterende grupper (LWW på `modified` / `location_changed`).
3. Slet gruppe-tombstones (se regler nedenfor).
4. Placér hver entry i dens `parent_group` (map sentinel → lokal Root).
kotpass har byggeklodserne (`Group`, `modifyParentGroup`), men selve
gruppe-merge-algoritmen skrives i hånden. `collectEntries` på Android skal
tilsvarende også emittere grupper.

## Konflikt-semantik (skal bekræftes)

- **Omdøb/flyt af gruppe:** last-writer-wins på tidsstempel (som entries).
- **Sletning af gruppe (tombstone via `deleted`-flag):** Entries/undergrupper i en
  slettet gruppe → **flyttes til papirkurv** (KeePass-norm). *(Åbent: alternativ =
  flyt til Root, eller blokér sletning af ikke-tom gruppe.)*
- **Cyklisk parent** (A→B→A pga. samtidige flytninger): detektér og bryd ved
  apply (fald tilbage til Root for den brydende gruppe) — ellers uendelig løkke.
- **Papirkurv:** recycle-bin er allerede speciel (`activeRecycleBinUuid`).
  Gruppe-sync må ikke synkronisere papirkurvs-strukturen forkert.
- **Root:** synkroniseres aldrig som objekt; kun via sentinel.

## Bagudkompatibilitet / mixed-version flåde

- Server-filtrering (ovenfor) beskytter gamle klienter mod gruppe-blobs.
- **Kendt begrænsning:** parent_group ligger i entry-blob'en. En *gammel* klient,
  der redigerer + pusher en entry, kender ikke feltet → det forsvinder på
  round-trip → entry'en falder tilbage til Root på andre enheder, til en ny
  klient re-filer den. Acceptabelt for lille flåde (pt. ~1 bruger); doc'er det.
  Anbefaling: opgradér alle klienter før man stoler på gruppe-sync.

## Faser

1. **Forward-compat: server-side filtrering** af `GET /changes` (entries-only as
   default). Blokerende — men ufarlig at udrulle alene.
2. **Canonical-model:** `canonical.Group`, `Entry.parent_group`, root-sentinel,
   envelope `0x02`, `SchemaVersion`→2. + tests.
3. **Server:** migration 008 (`object_kind`), PUT/GET for grupper, `?include=groups`.
4. **Desktop:** emit grupper i `collectEntries`, push, genopbyg gruppetræ i staging.
5. **Android:** gruppe-bevidst `applyToDatabase` + `collectEntries`.
6. **Konflikt-regler + tests:** cykler, sletning, root-mapping, mixed-fleet.

Indsats: fase 5 + 6 er ~70%. Fase 1 først (uafhængig). Desktop (fase 4) næsten
gratis pga. keepassxc-cli.

## Åbne spørgsmål (kræver beslutning)

1. Entries i en **slettet gruppe**: papirkurv (anbefalet) vs. Root vs. blokér?
2. **Mixed-fleet-politik**: acceptér transient Root-fald (anbefalet for lille
   flåde) vs. udskyd til alle klienter er opgraderet?
3. Root-sentinel: tom string (anbefalet) vs. null-UUID?
