# keepass-deltasync-docs

Fælles dokumentation — protokol-specifikation, betjeningsguides, threat model.

- **Licens:** CC-BY-SA-4.0 (passer bedre til tekst end GPL/AGPL).
- **Status:** I brug, men ikke komplet. Hovedspecifikationen ligger stadig
  samlet i [`../keepass-deltasync-spec.md`](../keepass-deltasync-spec.md) — på
  sigt brydes den op i emnespecifikke dokumenter her.

## Betjening

- `install-browser.md` — opsætning af Firefox-søgning, med og uden server.
  Den side udvidelsens opsætningsknapper peger på
- `self-hosting-docker.md` — kør din egen server med Docker Compose
- `deployment.md` — tre konkrete server-deployment-recipes (root, sub-path,
  subdomæne)
- `dockerhub-overview.md` — teksten der ligger på Docker Hub-siden

## Design

- `threat-model.md` — hvad serveren ser, hvad den ikke ser, multi-bruger
  trust-overvejelser
- `browser-extension.md` — Firefox-udvidelsens design og sikkerhedsgrænser
- `v2-concurrent-write-semantics.md` — race-condition-analyse for v2's
  multi-bruger sharing
- `v3-canonical-entry-format.md` — platform-uafhængigt entry-format
- `v4-group-sync.md` — gruppesynkronisering: struktur og entry-tilhørsforhold

## Testplaner

- `v2-test-plan.md` — live-test-procedure for M3+M4 (share-flow + accept-flow)
- `v4-test-plan.md` — lagdelt teststrategi for gruppe-sync
- `browser-host-linux-test-plan.md` — snap, flatpak og pakket Firefox

`diagrams/` rummer SVG'erne der bruges i rod-README'en.

## Planlagte dokumenter

- `protocol.md` — fuldt HTTP-API + JSON-skemaer
- `crypto.md` — XChaCha20-Poly1305 + HKDF-detaljer, nøgleafledning
- `operations.md` — backup, migration, log-håndtering
- `admin-guide.md` — admin-CLI og admin-API
