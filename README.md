# KeePass Delta-Sync

Synkroniseringssystem til KeePass-databaser (.kdbx) på entry-niveau, der genbruger KeePassXC's merge-logik til at undgå konflikter når flere klienter redigerer samme database samtidigt.

Filformatet (.kdbx) ændres ikke. Synkronisering sker via en let server der kun ser klient-krypterede blobs — masterpassword forlader aldrig klienten.

Se [`keepass-deltasync-spec.md`](keepass-deltasync-spec.md) for fuld specifikation.

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

Skelet. Kun server-komponenten er påbegyndt — schema, package-manifest og PHP-bootstrap er på plads. Endpoints er listet i `server/src/Router.php` men har endnu ingen implementering. Se [Milestone 1 i spec'en](keepass-deltasync-spec.md#milestone-1--server-mvp) for udestående punkter.

## Bidrag

- DCO sign-off påkrævet på commits (`git commit -s`).
- Sikkerhedshul rapporteres via privat kanal (security-mail / GitHub Security Advisory), ikke offentlige issues.
- Mindst to maintainere skal godkende ændringer i krypto eller auth.
