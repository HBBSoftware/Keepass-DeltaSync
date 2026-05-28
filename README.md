# KeePass Delta-Sync

Synkroniseringssystem til KeePass-databaser (.kdbx) på entry-niveau, der genbruger KeePassXC's merge-logik til at undgå konflikter når flere klienter redigerer samme database samtidigt.

Filformatet (.kdbx) ændres ikke. Synkronisering sker via en let server der kun ser klient-krypterede blobs — masterpassword forlader aldrig klienten.

Se [`keepass-deltasync-spec.md`](keepass-deltasync-spec.md) for fuld specifikation og [`docs/`](docs/) for protokol- og threat-model-detaljer.

## Struktur

Dette monorepo indeholder fire komponenter, hver med sin egen licens:

| Mappe | Komponent | Licens | Sprog |
|-------|-----------|--------|-------|
| [`server/`](server/) | Sync-server | AGPL-3.0-or-later | PHP 8.2 + PostgreSQL |
| [`client/`](client/) | Desktop sync-agent | GPL-3.0-or-later | Go |
| [`android/`](android/) | Android-klient | GPL-3.0-or-later | Go (gomobile) + Kotlin |
| [`docs/`](docs/) | Fælles dokumentation | CC-BY-SA-4.0 | Markdown |

Klient og server kommunikerer kun over et veldefineret HTTP-API, så GPL/AGPL "smitter" ikke på tværs.

## Status

**v1 + v2 multi-bruger sharing er funktionelt komplet og bruges aktivt** (pr. 2026-05-28).

- **Server** (PHP/PostgreSQL): live på shared hosting. Endpoints for enrollment, entries med 3-versioners historik, restore, admin-CLI, audit-log.
- **Klient** (Go): `enroll`, `init`, `init-shared`, `push`, `pull`, `sync`, `daemon` (fsnotify + polling), `versions`, `restore`, `share`/`unshare`/`shares`. Krypto: Argon2id → HKDF → XChaCha20-Poly1305 for entries, X25519 sealed-box for sharing.
- **Android-klient**: ikke påbegyndt.

## Komme i gang

### Som administrator (server-side)

Se [`server/README.md`](server/README.md) for deployment. Skema-migrationer i [`server/schema/`](server/schema/). Migrationer 001–005 udgør v1; 006–007 er v2.

### Som første-gangs-bruger (klient-side)

```sh
# Bygg klient
cd client && go build -o keepass-deltasync ./cmd/keepass-deltasync

# Administrator har givet dig en enrollment-token
./keepass-deltasync enroll --server https://your-server.example.com <token>

# Registrér en lokal .kdbx
./keepass-deltasync init mypasswords ~/keepass/my.kdbx

# Synkronisér
./keepass-deltasync sync mypasswords

# Lad daemon synke automatisk (fsnotify + polling)
./keepass-deltasync daemon --store-keyring
```

### Som modtager af en delt database (v2)

```sh
# Alice har delt sin database med dig. Du ser den i din liste:
./keepass-deltasync databases
# adgangskoder  ...  role=member

# Bootstrap en lokal kopi:
./keepass-deltasync init-shared adgangskoder ~/keepass/shared.kdbx
# Prompt: vælg et nyt lokalt password for din kopi
```

Derefter er sharing transparent — `sync`/`daemon` virker som for owned databases.

### Som deler af en database (v2)

```sh
# Del din database med en anden bruger (de skal være enrollet og have brugt klienten mindst én gang)
./keepass-deltasync share adgangskoder bob

# Liste medlemmer
./keepass-deltasync shares adgangskoder

# Fjern et medlem (eller forlad selv en delt database)
./keepass-deltasync unshare adgangskoder bob
```

Se [`docs/v2-concurrent-write-semantics.md`](docs/v2-concurrent-write-semantics.md) for hvordan samtidige edits håndteres.

## Trust-model i korthed

- **Serveren ser:** krypterede entry-blobs, mtime, sletninger, bruger- og enhedsmetadata, audit-log med ip+useragent.
- **Serveren ser IKKE:** entry-indhold (titler, brugernavne, passwords), masterpasswords, database master_keys.
- **Multi-bruger sharing:** når Alice deler en database med Bob, krypteres database master_key med sealed-box til Bob's enheds public-key. Serveren ser det opaque sealed-box-blob; kun Bob's enheds private-key kan unwrappe.

Se [`docs/threat-model.md`](docs/threat-model.md) for fuld trust-model.

## Bidrag

- DCO sign-off påkrævet på commits (`git commit -s`).
- Sikkerhedshul rapporteres via privat kanal (security-mail / GitHub Security Advisory), ikke offentlige issues.
- Mindst to maintainere skal godkende ændringer i krypto eller auth.
