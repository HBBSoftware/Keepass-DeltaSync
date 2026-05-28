# Threat model

Hvilke aktører er i scope, hvad de kan se og påvirke, og hvilke garantier
vi giver mod hver. Modellen dækker både v1 (single-user) og v2
(multi-user sharing).

## Aktører

| Aktør | Beskrivelse |
|-------|-------------|
| **Bruger** | Lovlig ejer eller medlem af en database. Har masterpassword (owner) eller wrapped_master_key + device-private-key (member). |
| **Server-admin** | Har fuld adgang til hosting-miljø: DB-rows, log-filer, kørende processer. Kan ikke åbne klient-krypterede blobs uden klient-side keymateriale. |
| **Passiv netværks-observatør** | Ser TLS-krypteret trafik. Med valid certifikat-pinning umuligt at MitM uden CA-kompromittering. |
| **Aktiv angriber med RCE på server** | Kan læse alle DB-rows, ændre dem, observere kørende processer. Stadig blokeret fra klient-krypteret blob-indhold. |
| **Angriber med adgang til klient-enhed** | Har potentielt adgang til config.toml (device_token + device_private_key) og lokal .kdbx-fil. |
| **Anden enrolled bruger** | Har egen device-token, kan lookup andre brugere via username. Kan ikke se andre brugeres databaser uden delt adgang. |

## Hvad serveren ser

Selv en fuldt kompromitteret server kan ikke afsløre entry-indhold uden
en klient-side nøgle. Det den DOG kan se:

- **Krypterede blobs** (nonce ‖ ciphertext) — opaque
- **Bruger- og enhedsmetadata**: username, display_name, device name, enrolled_at, last_seen, public_keys
- **Database-metadata**: navn (Alice's valg ved init), oprettelses-tid, medlemskab (`database_members`)
- **Entry-metadata**: UUID, mtime, deleted-flag, server_seq, available_versions
- **Audit-log**: hvilke endpoints der er kaldt, fra hvilken IP, hvilken bruger, hvornår

Konsekvenser:
- En kompromitteret server kan slette eller manipulere blobs (DoS, men ikke
  forfalskning af content — klienten vil få MAC-fejl ved decryption)
- En kompromitteret server kan slå alle entry-mtimes op og se aktivitetsmønstre
- Database-navnet er valgt af brugeren og ofte beskrivende ("personal",
  "work") — _ikke_ behandlet som hemmelighed

## Hvad serveren ikke ser

- **Entry-indhold**: titles, usernames, passwords, notes, URLs, attachments
- **Master_keys**: hverken Alice's eller Bob's. Server ser kun `wrapped_master_key`
  (sealed-box) som kun Bob's device-private-key kan unwrappe.
- **Masterpasswords**: forlader aldrig klienten. Argon2id sker lokalt.
- **Device-private-keys**: lever kun i klientens config.toml.
- **KeePassXC's lokale kdbx-password**: hver klient har sit eget; aldrig
  sendt over wire.

## Krypto-garantier

- **Entry-encryption**: XChaCha20-Poly1305 med 24-byte random nonce pr.
  entry. 32-byte entry-key derived via HKDF-SHA256(master_key,
  salt=db_uuid_bytes, info="deltasync-entry-v1").
- **Master_key**: Argon2id(password, salt=db_uuid_bytes, t=3, m=64MiB, p=1)
  — kun klient-side, kun for owners. For members kommer master_key fra
  unwrap.
- **Sharing-wrapping**: NaCl sealed-box (X25519 + XSalsa20-Poly1305) med
  ephemeral keypair pr. wrap. Sender er anonym for serveren.
- **Nonce-genbrug**: random 24-byte nonces giver 2^192 nonce-rum — kollision
  er praktisk umulig (sikrere end CTR-style sequence-nonces).

## v2-specifikke trust-overvejelser

### Multi-bruger transitivt tillidsforhold

Når Alice deler en database med Bob:

1. Alice's klient deriverer master_key fra hendes masterpassword
2. Alice's klient wraps master_key til Bob's enheds public_key
3. Bob's klient unwraps med sin private_key og har nu samme master_key
4. Bob kan dekryptere ALLE entries i databasen, både historiske og fremtidige

**Konsekvens**: Alice giver Bob fuld read-write-adgang. Der findes ikke
"share kun denne entry"-granularitet. Hvis Alice senere ikke længere
stoler på Bob, må hun:

1. `unshare adgangskoder bob` → fjerner Bob's adgang fra serveren (fremtidige
   pull fejler 404)
2. Roter masterpassword OG genskab databasen — Bob har stadig master_key i
   memory/disk fra perioden hvor han havde adgang. Tidligere kopier af
   blobs (gemt af Bob før unshare) kan stadig dekrypteres.

**Forensisk genåbning er fundamentalt uundgåelig** i ende-til-ende-
krypteret deling — Bob HAR data'en så længe han var medlem.

### Privacy lookup

`GET /users/lookup?username=X` lader enhver enrolled bruger finde andre
brugere ved username. Dette er nødvendigt for share-flowet, men:

- Enumeration: en angriber med en enrollment-token kan probe usernames
- Mitigation i v2.0: ingen — kontrolleret deployment med begrænset bruger-
  population
- Senere: opt-in "findable"-flag eller invite-token-baseret sharing

### Re-share rotation kan ikke "ulære" master_key

Hvis Bob's enhed kompromitteres, og Alice re-share'r til Bob's nye enhed:
- master_key er stadig det samme (vi roterer ikke server-side ved re-share)
- Den kompromitterede gamle enhed har stadig master_key i sin lokale state
- For at virkeligt rotere master_key skal alle entries re-encrypteres og
  alle medlemmer re-wraps — v2.1-feature

## Klient-enhed-kompromittering

Hvis en angriber får adgang til en klient-enhed:

| Tilgået | Konsekvens |
|---------|-----------|
| `config.toml` | device_token (auth som denne enhed), device_private_key (kan unwrap shared keys) |
| `~/.../keyring` (`--store-keyring` brugt) | masterpasswords for owned databases |
| Lokal `.kdbx`-fil | KDBX' egen kryptering med brugerens password — ikke ekstra beskyttet af os |

Mitigations:
- Klient zeroer master_key + entry_key + password efter brug (`passwd.Zero`).
  Reducerer cold-boot/memory-dump-vinduet men eliminerer det ikke.
- OS keyring lagrer kun masterpasswords (kun for owned databases). Member-
  databases lagrer ikke master_key — den genskabes fra wrapped_master_key
  + device-private-key ved hver sync.
- Anbefaling til brugere: brug fuld-disk-encryption på enheden;
  device_private_key er i config.toml på klar tekst (TOML-fil, file-perm
  0600).

## Audit og recovery

Server bevarer 3 versioner pr. entry. Tombstone-DELETEs tæller som versioner.
`restore <db> <uuid> <n>` ruller en gammel version tilbage. Det giver
modstandskraft mod:

- Bruger der fortryder en ændring
- Ransomware-style "alle entries krypteret med ondsindet payload" (rul
  tilbage til version før angrebet)
- En member der saboterer med dårlige edits (restore før dem)

Audit-log dækker 90 dage default. Indeholder: timestamp, level, event_type,
user_id, device_id, database_id, entry_uuid, IP, user_agent, success-flag.
Bruger kan se sit eget log via `log`-kommandoen; admin kan se hele log via
admin-API.

## Ikke-mål (out of scope)

- **Forward secrecy**: efter en master_key-kompromittering kan tidligere
  blobs dekrypteres af angriber. Vi har ingen key-rotation-mekanisme i
  v2.0 udover at genoprette databasen fra bunden.
- **Anonymitet**: server kender hvem der hører til hvem (alle brugere er
  identificeret ved username).
- **Hardening mod side-channels på server**: vi bruger constant-time
  password-hashing (Argon2id) i klienten, men server's PHP-runtime giver
  ingen særlige garantier mod timing-angreb.
- **End-to-end deniability**: alle krypterings-operationer er deterministiske
  ift. plaintext+key (givet nonce); en kompromitteret klient-enhed kan
  rekonstruere hele beslutnings-historikken.
