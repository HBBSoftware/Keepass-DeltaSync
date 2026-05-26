# keepass-deltasync-client

Desktop sync-agent (CLI/daemon) til Linux, Windows og macOS.

- **Sprog:** Go
- **Licens:** GPL-3.0-or-later (matcher KeePassXC's licensfamilie)
- **Status:** Ikke påbegyndt. Implementeres efter server-MVP'en er klar — se [Milestone 2 i spec'en](../keepass-deltasync-spec.md#milestone-2--linux-klient-go).

## Forventet CLI

```
keepass-deltasync enroll <enrollment-token>
keepass-deltasync init <name> <local.kdbx>
keepass-deltasync sync [name]
keepass-deltasync status
keepass-deltasync daemon
keepass-deltasync devices
keepass-deltasync versions <name> <entry-uuid>
keepass-deltasync restore <name> <entry-uuid> <version-num>
keepass-deltasync log [--since=24h] [--limit=50]
```

Læs spec'ens [`Klient-komponent`-afsnit](../keepass-deltasync-spec.md#klient-komponent--sync-agent) for designdetaljer (krypterings-lag, sync-cyklus, konfigurationsformat).
