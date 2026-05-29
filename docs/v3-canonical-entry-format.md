# v3 canonical entry format

Forberedelse til M4 (Android). Erstatter "rå keepassxc-cli InnerXML fragment som
on-wire payload" med et eksplicit, platform-uafhængigt canonical schema.

Designet er drevet af to ting:

1. Kotpass (Kotlin KeePass-bibliotek vi vil bruge på Android) eksponerer
   *typed* `Entry`-objekter, ikke XML. Hvis Android skal kunne producere blobs
   som desktop kan dekryptere og merge, skal wiren tale en form som begge
   platforme kan emittere og parse uafhængigt af `keepassxc-cli`.
2. KeePassXC-cli kører ikke på Android. Vi skal selv implementere
   entry-level merge — og det er nemmere på en velkendt struktureret
   repræsentation end på rå XML-fragmenter.

## On-wire envelope

Blob-strukturen ændres ikke: stadig `nonce(24) ‖ ciphertext ‖ MAC(16)` via
XChaCha20-Poly1305. Det er *plaintext*'en der får en version-byte prefix:

```
byte 0     = format version
byte 1..N  = payload
```

Format-versioner:

| Byte | Format | Notes |
|------|--------|-------|
| `0x3C` (`<`) | Legacy v1 (raw InnerXML) | Eksisterende blobs — auto-detekteres |
| `0x01` | Canonical v1 (JSON) | Nye blobs |

Detection er kollisionsfri fordi gyldig XML altid starter med `<` og vores
canonical-magic er en byte vi vælger. Begge klienter implementerer
*dual-read* under migration; kun *write-canonical* fra dag ét.

## Canonical schema (v1, JSON)

JSON valgt fremfor CBOR for debugability — attachments er base64 alligevel,
så størrelsesgevinsten ved CBOR er beskeden.

```json
{
  "v": 1,
  "uuid": "01234567-89ab-cdef-0123-456789abcdef",
  "times": {
    "created":          "2026-05-29T10:00:00Z",
    "modified":         "2026-05-29T10:00:00Z",
    "accessed":         "2026-05-29T10:00:00Z",
    "expires_at":       null,
    "expires":          false,
    "usage_count":      0,
    "location_changed": "2026-05-29T10:00:00Z"
  },
  "fields": {
    "Title":    {"v": "GitLab",       "protected": false},
    "UserName": {"v": "hans",         "protected": false},
    "Password": {"v": "...",          "protected": true},
    "URL":      {"v": "https://...",  "protected": false},
    "Notes":    {"v": "...",          "protected": false}
  },
  "custom_fields": {
    "API-Token": {"v": "...", "protected": true}
  },
  "binaries": [
    {"name": "key.pem", "data": "base64..."}
  ],
  "tags":             ["work", "important"],
  "icon_id":          0,
  "custom_icon_uuid": null,
  "foreground_color": null,
  "background_color": null,
  "override_url":     null,
  "autotype": {
    "enabled":          true,
    "obfuscation":      0,
    "default_sequence": "{USERNAME}{TAB}{PASSWORD}{ENTER}",
    "associations": [
      {"window": "...", "sequence": "..."}
    ]
  },
  "quality_check": true,
  "custom_data": {
    "KPXC_BROWSER_xxx": {"v": "...", "modified": "..."}
  },
  "history": [
    { "... shallower entry (uden indlejret history) ..." }
  ]
}
```

Mapping er én-til-én med KDBX4 entry-schema. `protected` bevares per felt så
re-import via keepassxc-cli kan markere felter til memory-protection ved
næste kdbx-save.

## Migration

Produktionen har én bruger (`hans`) pr. 2026-05-29:

1. Begge klient-codepaths (desktop nu, Android senere) implementerer
   *dual-read*: detekter format på byte 0, parse derefter.
2. *Write-canonical* fra dag ét — alle nye PUT-blobs er v1 canonical.
3. Hans kører `keepass-deltasync push --force` én gang efter upgrade.
   Alle eksisterende blobs erstattes af canonical-versioner.
4. Server's 3-version-rotation pr. entry betyder at gamle XML-blobs ryger
   ud naturligt efter et par push'er.
5. Når metrics viser at ingen byte-0-`<` blobs forekommer i nogen aktiv
   database, kan dual-read-stien fjernes fra klienterne.

Server kræver ingen ændringer — blobs er stadig opake bytes for den.

## Pipeline changes

### Desktop push

```
kdbx → cli.Export → InnerXML fragments per entry
                      ↓
             canonical.FromInnerXML(fragment)   ← NY
                      ↓
                  json.Marshal
                      ↓
                  prefix 0x01
                      ↓
             crypto.EncryptBlob
                      ↓
                client.PutEntry
```

### Desktop pull

```
client.GetChanges
       ↓
crypto.DecryptBlob → plaintext
       ↓
   peek byte 0
       ├── 0x3C (legacy)  → fragment som hidtil
       └── 0x01 (canonical)
              ↓
          json.Unmarshal
              ↓
       canonical.ToInnerXML(c)   ← NY
              ↓
          fragment
                      ↓
              ...
       kdbx.BuildStagingXML → cli.Import → cli.Merge   (uændret)
```

### Android push/pull (M4)

Android har ingen `cli.Export`/`cli.Merge`. Pipeline'en bliver:

```
push: kotpass.Entry → canonical (Kotlin mapper) → JSON → 0x01 → encrypt → PUT
pull: GET → decrypt → JSON → canonical → mergeInto(kotpass.Entry) → kotpass.save
```

`mergeInto` er vores egen entry-level last-writer-wins (se næste afsnit).

## Merge algorithm (v1)

Entry-level, last-writer-wins via `times.modified`. Pseudo:

```
for each incoming canonical entry C:
  if local has no entry with C.uuid:
    insert C
  elif local[C.uuid].times.modified < C.times.modified:
    replace local[C.uuid] with C
  else:
    drop C (local nyere — vil blive pushet næste push)

for each tombstone T:
  if local has entry T.uuid:
    delete local[T.uuid]
  insert T in local tombstones liste
```

Det er svagere end keepassxc-cli's field-level merge — Alice ændrer Password,
Bob ændrer URL, sidste push wins begge ændringer mens den anden taber begge.
Acceptabelt for v1; per-field mtime tilføjes som v2-enhancement *uden*
ændring af canonical schema (vi tilføjer kun læsning af nye `modified`-felter
per `String`/`custom_field`).

På desktop bevares keepassxc-cli's merge for legacy-blobs i overgangs-
perioden. For canonical-blobs udfører vi vores egen merge før vi bygger
staging-XML — eller staging-XML'en indeholder kun "winners" og keepassxc-cli
merger dem ind. Detalje pinned i implementation-phase.

## Implementations-faser

**Phase A — schema + lossless conversion (Go):**

- A1: `client/internal/kdbx/canonical/` package med `type Entry struct`
- A2: `FromInnerXML([]byte) (*Entry, error)` — parse keepassxc-cli fragment
- A3: `ToInnerXML(*Entry) ([]byte, error)` — emit keepassxc-cli-kompatibel
- A4: Round-trip-tests: parse fragment → canonical → emit → parse → samme struct.
      Differential test mod ægte kdbx-eksporter for at fange tabte felter.

**Phase B — wire integration desktop:**

- B1: Version-byte prefix i `EncryptBlob` / detection i `DecryptBlob`
- B2: Push: A2 → JSON → 0x01 → encrypt
- B3: Pull: decrypt → detect → A3 → eksisterende staging-pipeline
- B4: `push --force` migration på hans' enheder

**Phase C — Android scaffold:**

- C1: `mobile/` Go-package eksponerer canonical-types + crypto + HTTP via
      gomobile-friendly API (ingen channels, ingen funcs som parametre, kun
      simple types + []byte + string)
- C2: `gradle init` Kotlin app skeleton, kotpass-dep, lokalt `.aar`-import
- C3: kotpass `Entry` ↔ canonical Kotlin-mapper (kan generes fra A1-schema)
- C4: Foreground service + WorkManager-trigger
- C5: F-Droid metadata + reproducible-build setup

**Phase D — cross-platform validering:**

- D1: Desktop ↔ desktop sync regresses ikke
- D2: Push fra Android → pull på desktop → entry vises korrekt i KeePassXC
- D3: Push fra desktop → pull på Android → entry vises korrekt via kotpass

**Phase E — clean-up:**

- E1: Fjern legacy-XML-stien fra både klienter efter grace-periode
- E2: Per-field mtime merge (v2-enhancement af schema)

## Åbne spørgsmål

1. **Memory-protection re-encryption**: keepassxc-cli's import sætter
   automatisk Salsa20-stream-cipher på `Protected="True"` felter. Verificer
   at vores `ToInnerXML` med `protected=true` resulterer i samme behavior
   som original kdbx havde. Hvis ikke: post-import-kald til keepassxc-cli
   for at re-protect.

2. **Binary pool**: KDBX'er binary attachments som dedupede pool-entries.
   Vores canonical sender inline base64. Verificer at keepassxc-cli's
   import allokerer pool-entries korrekt. Hvis ikke: små attachments
   (under threshold) inlines, store afvises eller chunkes.

3. **History depth**: KDBX'es `History` per entry kan være vilkårligt
   stort. Bevarer vi det fuldt? Foreslår ja — keepassxc-cli's GUI har
   selv en `MaxHistoryItems`/`MaxHistorySize` der trimmer på save.

4. **Server schema_version i metadata**: Skal serveren tracke pr.-entry
   "canonical or legacy" for at give klienter et hint om hvad de vil få?
   Foreslår nej — det adder server-state og kollidere med "server ser kun
   opake bytes"-principperne. Detection ved byte 0 efter dekryptering er
   tilstrækkeligt.

5. **Sharing-wrap-format**: `wrapped_master_key` (v2) påvirkes ikke — den
   wrapper master_key, ikke entry-blobs. Canonical-skift er ortogonalt.
