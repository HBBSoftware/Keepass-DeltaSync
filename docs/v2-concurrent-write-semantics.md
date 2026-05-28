# v2 concurrent-write semantics

Hvad sker når Alice og Bob (begge medlemmer af samme delte database) skriver
samtidigt? Dette dokument beskriver de faktiske garantier, hvor race
conditions er udelukket, og hvor brugeren skal forvente eventual
consistency frem for stærk konsistens.

Genereret som del af M5 — analysis-only, ingen kode-ændringer. Den her hører
sammen med [`v2-test-plan.md`](v2-test-plan.md).

## Server-side ordering

Hver `entry_versions`-insert serialiseres pr. database af `database_seq`-row.
En PUT/DELETE-handler:

1. `UPDATE database_seq SET next_seq = next_seq + 1 WHERE database_id = ? RETURNING ...`
   tager en row-lock — kun én PUT/DELETE ad gangen pr. database kan passere
   forbi denne linje.
2. `INSERT entry_versions ... VALUES (..., version_num=3)`. Triggeren fra
   migration 005 roterer eksisterende rækker 3→2, 2→1, sletter en evt.
   tidligere 1.

Konsekvens: **race-fri** server-side. To samtidige PUTs på samme entry får
distinkte `server_seq`-værdier; den med højeste server_seq er den der vises
af `GET /changes` indtil næste PUT.

## Last-writer-wins via klient-mtime

Klientens `pull` merger via `keepassxc-cli merge`, som sammenligner entries'
interne `<LastModificationTime>` (mtime). Ved konflikt på samme UUID:

- Klient med højere mtime vinder feltets værdier
- KeePassXC's merge er pr. felt — Alice kan have ændret password, Bob
  username; merge tager hver fra den nyeste version
- `pullChanges` i klienten skriver server's `modified_at` ind i fragmentet
  før merge (`rewriteLastModificationTime`) — det sikrer at restore-flowet
  (server kopierer gammel blob med ny mtime) vinder over lokale ældre
  versioner

## Garanti vi har

For en delt database er det garanteret at:

1. Alle med adgang ser **samme nyeste version** pr. entry efter alle har
   pulled (eventual consistency)
2. Server bevarer **3 versioners historik** pr. entry; intet skrev tabes
   permanent inden for vinduet
3. Sletninger er **tombstones**, ikke fysiske rækker — kan tilbagerulles via
   `restore <db> <uuid> <n>` af enhver bruger med adgang
4. Klienten retransmitterer **kun entries hvis lokale mtime > sidst sete
   mtime** (`EntryStates`-map) — ingen polling-storm efter en stille sync

## Edge cases der ikke er stærkt konsistente

### Tombstone vs. resurrection

Alice deletes entry X (push DELETE, server_seq=N).
Bob — uden at have pullet endnu — editer entry X lokalt (mtime T_b).
Bob's `sync` pusher entry X som PUT (server_seq=N+1).

Resultat: entry X er **live igen** på serveren med Bob's blob. Alice's
næste pull merger Bob's version ind i hendes lokale kdbx — entry kommer
tilbage. Det er last-writer-wins, men kan overraske Alice der "slettede den".

Mitigation: hvis dette er et reelt problem i praksis, kan vi i M6 tilføje en
warning i klienten når den ser en entry resurrected (deleted=true i forrige
version, deleted=false i nyeste). Indtil videre dokumenteret men ikke
implementeret.

### Konkurrent edit på samme felt

Alice editer entry X's password til "alice123" på sin enhed.
Bob editer entry X's password til "bob456" på sin enhed.
Begge sync.

Resultatet er den **klient der gemmer sidst** vinder for det password-felt.
KeePassXC's merge er felt-niveau, så hvis Alice ændrer password og Bob
ændrer username, beholder begge ændringer. Men hvis begge ændrer samme
felt, taber den ene.

Mitigation: ingen — KDBX-formatet har ingen merge-conflict-markering.
Brugerne må koordinere out-of-band.

### Stale wrapped_master_key i daemon

Daemon henter `role` + `wrapped_master_key` én gang ved start. Hvis Alice
re-share'r mens Bob's daemon kører (rotation pga. ny enhed), vil daemon
bruge den gamle wrapped indtil den genstartes.

For Bob's nuværende enhed gør det ikke en forskel — `master_key` er det
samme før og efter rotation. Det er kun hvis Bob har en ANDEN enhed der
trådte i kraft som "target device" at rotation kunne ramme. Daemon-restart
løser det.

### Manuel `sync` parallelt med daemon

`syncMu` serialiserer alle syncs i daemon-processen. To processer (manuel
`sync`-kommando + kørende daemon) har INGEN fælles mutex. Hvis brugeren
kører `sync` mens daemon kører:

- Begge læser config.toml
- Begge laver pull/push
- Begge skriver config.toml — den sidste vinder

Den faktiske risiko er begrænset til at `last_seq` kan blive sat tilbage,
hvilket fører til en ekstra "no-op" pull næste cycle. Ingen data tab.
Eksplicit "user error" i v1; ikke håndteret i v2.

## Sammenfatning

For typiske bruger-scenarier (sjælden samtidige edits, lange perioder af
stille i daemon-mode, manuel sync når nødvendigt) er semantikken passende:
last-writer-wins, eventual consistency, ingen silent corruption. Server-
serialization eliminerer den vigtigste race-class. Klient-mtime håndterer
resten.
