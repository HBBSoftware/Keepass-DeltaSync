# v5 — Synkronisering mellem to servere

Status: **ANALYSE / forslag** (2026-09-06). Ingen kode-ændringer; dette
dokument beskriver hvad der skal til, og hvad det koster. Hører sammen med
[`v2-concurrent-write-semantics.md`](v2-concurrent-write-semantics.md), hvis
konfliktsemantik gælder uændret — bare med timer i stedet for millisekunder
mellem skribenterne.

## Mål

To DeltaSync-servere skal kunne udveksle blobs, i tre niveauer der kan vælges
pr. opsætning:

1. **Kopi** — envejs. Den ene server spejler den anden.
2. **Fuld synk** — begge veje.
3. **Filtreret** — kun blobs fra udvalgte enheder må sendes til / modtages af
   en given server.

Afgørende designvalg: **kun den aktive version synkroniseres.** Versions-
historikken må gerne være forskellig på hver server.

## Hvorfor det overhovedet kan lade sig gøre

**Serveren ser aldrig klartekst.** `entry_versions.blob` er klient-krypteret,
og owners udleder master_key fra Argon2id(password) lokalt —
`database_members.wrapped_master_key` er `NULL` for owners, så nøglen findes
ikke på serveren. En server nummer to kan holde nøjagtig samme ciphertext
**uden at nogen skal stole på den mere end på den første**. Replikering er et
transportproblem, ikke et kryptoproblem.

**Id'er er UUID'er** fra `gen_random_uuid()`, så `database_id`, `entry_uuid`,
`user_id` og `device_id` kolliderer ikke på tværs af servere.

**Sletninger er tombstones**, ikke fjernede rækker, så de replikerer som data.
En mekanisme der kun flyttede levende rækker ville genoplive slettede entries.

## Nøgleobservationen: `/changes` sender allerede kun den aktive version

`EntryController::changes()` filtrerer:

```sql
WHERE ev.database_id = :db
  AND ev.version_num = 3        -- kun nyeste
  AND ev.server_seq  > :since
```

Sync-stien har aldrig transporteret historik. Version 2 og 1 nås udelukkende
gennem `versions`- og `restore`-endpointsne, som klienten kun kalder på
opfordring.

Det har to konsekvenser, der begge trækker i samme retning:

- **"Kun aktiv version" er ikke en forringelse.** Det er en beskrivelse af
  hvordan protokollen allerede opfører sig. At to servere har hver sin
  historik indfører ingen ny inkonsistens, fordi historik allerede er en
  server-lokal artefakt ingen klient synkroniserer.

- **Server B kan blive sync-klient af server A på det uændrede endpoint.**
  `/changes?since=N` *er* en replikeringsfeed: nyeste version pr. objekt,
  tombstones inkluderet, ordnet efter `server_seq`.

En følge, der er værd at have skrevet ned, fordi den er kontraintuitiv: to
hurtige skrivninger på samme entry — også på én enkelt server i dag — ender
med at kun den sidste når klienterne. Den førstes række roterer til
`version_num = 2` og forsvinder dermed fra `/changes`. Last-writer-wins gælder
altså allerede på blob-niveau; det er ikke noget replikering introducerer.

## Arkitektur

Hver klient hører til én server. Serverne udveksler indbyrdes:

```
klient X ──▶ server A ◀──── replikering ────▶ server B ◀── klient Y
```

Server B husker **ét tal pr. peer**: "jeg har konsumeret A's strøm til seq N".
Den henter `/changes?since=N` og anvender resultatet som almindelige
skrivninger med sine **egne lokale** `server_seq`-numre.

Det betyder, at `server_seq` forbliver en ren lokal, monoton markør, præcis som
i dag. B's egne klienter ser én sammenhængende strøm indeholdende både lokale
og replikerede skrivninger. **Ingen versionsvektor er nødvendig** — et
vandmærke pr. peer er nok, netop fordi kun den aktive version udveksles.

Rotationstriggeren fra migration 005 går fra fælde til fordel: når B anvender en
replikeret nyeste version, skubber triggeren B's forrige ned på plads 2. B's
historik betyder dermed "de sidste tre tilstande *denne server* har observeret",
hvilket er en sammenhængende og forklarlig semantik.

## Det der skal løses

### Ekko

B anvender A's version som en lokal skrivning og giver den et nyt lokalt seq.
Ved næste runde vil B tilbyde den tilbage til A. Uden noget at genkende den på
kører de to i ring.

Løses **uden skemaændring**: blobs kopieres byte for byte, ikke krypteres om.
En sammenligning af `(entry_uuid, blob, modified_at)` mod modtagerens egen
nuværende nyeste er derfor en gyldig "den har jeg allerede"-test. Er de
identiske, springes rækken over.

Bivirkning: en klient der skriver præcis samme indhold igen propagerer ikke.
Ingen kan mærke det.

### Konfliktregel

Har begge servere skrevet på samme entry siden sidste udveksling, skal én
vinde. Forslag: `modified_at` (klientens ur, som i dag), med `origin_server_id`
som deterministisk tiebreak så begge servere når frem til samme resultat uden
at tale sammen om det.

Tombstone-genopstandelsen beskrevet i v2-dokumentet — Alice sletter, Bob
redigerer uden at have pullet — går fra sjælden race til noget der sker jævnligt,
fordi vinduet nu er så længe serverne er ude af kontakt. Semantikken er uændret,
sandsynligheden er ikke.

### Attribution (forudsætning for niveau 3)

`entry_versions` har **ingen `device_id`**. Feltet optræder i enrollment,
auth-konteksten, enhedsadministration og audit-loggen — men ingen steder
registreres hvilken enhed der producerede en blob. Audit-loggen har det pr.
hændelse, men slettes efter `AUDIT_RETENTION_DAYS` (30 som standard), så
proveniensen fordamper.

Krævede kolonner på `entry_versions`:

- `device_id` — sættes ved PUT fra `AuthContext`, som allerede bærer `deviceId`
- `origin_server_id` — hvilken server rækken stammer fra

Begge er additive og bagudkompatible. **Men de kan ikke laves med tilbagevirkende
kraft** — eksisterende rækker får aldrig proveniens. Det er argumentet for at
tilføje dem nu, uanset hvilket niveau der bygges hvornår.

Et tillidsforbehold: attribution registreret af A er kun så troværdig som A.
Skal B kunne filtrere uafhængigt af A's velvilje, skal blobs signeres af
enheden. Enhederne har X25519-nøglepar, men de er til sealed-box key agreement,
ikke signaturer — det ville kræve Ed25519 ved siden af.

## De tre niveauer

### Niveau 1 — kopi

Målet er read-only for klienter og udleder sin egen lokale seq. Replikeres:
`databases`, `database_members`, `entries`, `entry_versions` (kun
`version_num = 3`), `users`, `devices`. Replikeres **ikke**: `admin_tokens`,
`enrollment_tokens`, `admin_account`, `admin_sessions`, `auth_attempts`.

Vær ærlig om hvad det er: en varm standby. **Skal der ikke vælges enkelte
databaser ud, løser Postgres' egen streaming-replikering eller `pg_dump` det
bedre og billigere.** Selektiv per-database-spejling er den eneste grund til at
bygge det selv.

### Niveau 2 — begge veje

Samme mekanik i begge retninger, plus ekko-testen og konfliktreglen ovenfor.
Med nyeste-kun kræver det hverken protokolbrud eller versionsvektor.

### Niveau 3 — filtreret

To forskellige ting gemmer sig i "kun udvalgte enheder":

- **Push-filter**: hvilke af mine blobs forlader denne server
- **Accept-filter**: hvilke indkommende blobs gemmer jeg

Kun accept-filteret giver en reel sikkerhedsegenskab, fordi modtageren
håndhæver det uanset hvad afsenderen gør. Begge bør findes; accept-filteret er
kontrollen.

Nyeste-kun gør niveau 3 renere: afviser B en blob fra en ikke-godkendt enhed,
står B's nyeste blot på en ældre værdi, indtil en godkendt enhed skriver igen.
Der hober sig ingen huller op.

**Prisen skal stå et sted en bruger læser:** afviser B en enheds skrivning, og
skriver ingen godkendt enhed derefter, viser A og B forskellige værdier for
samme entry på ubestemt tid, uden fejlmeddelelse. Det er ikke en fejl — det er
hvad et accept-filter betyder.

## Det klienterne kan mærke

- `available_versions` i `/changes`-svaret tælles per server. En klient på B kan
  se `1`, hvor en klient på A ser `3`.
- `restore <db> <uuid> <n>` vælger fra en server-lokal historik og giver derfor
  forskellige resultater afhængigt af hvilken server der spørges. Selve restore
  laver en ny nyeste version med ny mtime, så *effekten* replikerer normalt —
  det er kun udvalget, der er lokalt.

Det er den samlede pris ved divergerende historik.

## Rækkefølge

1. **Attribution først** (`device_id`, `origin_server_id`). Additivt, billigt,
   og det eneste der ikke kan gøres bagudrettet.
2. Niveau 1 kun hvis selektiv spejling er kravet — ellers brug Postgres.
3. Niveau 2 oven på ekko-test og konfliktregel.
4. Niveau 3 som accept-filter, når attribution findes.

## Forhold til v4

[v4 gruppe-sync](v4-group-sync.md) er stadig på designstadiet og lægger grupper
ind i den samme `server_seq`-strøm med `object_kind = 2`. Alt ovenstående gælder
derfor grupper uændret — én mekanisme dækker begge. Men rækkefølgen betyder
noget: låses v4's protokol fast uden at replikering er tænkt med, skal den
brydes to gange.
