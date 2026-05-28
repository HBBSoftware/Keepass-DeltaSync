# keepass-deltasync-docs

Fælles dokumentation — protokol-specifikation, betjeningsguides, threat model.

- **Licens:** CC-BY-SA-4.0 (passer bedre til tekst end GPL/AGPL).
- **Status:** Ikke påbegyndt. Hovedspecifikation findes pt. som [`../keepass-deltasync-spec.md`](../keepass-deltasync-spec.md) — på sigt brydes den op i emnespecifikke dokumenter her.

## Tilgængelige dokumenter

- `threat-model.md` — hvad serveren ser, hvad den ikke ser, multi-bruger trust-overvejelser
- `v2-concurrent-write-semantics.md` — race-condition-analyse for v2's multi-bruger sharing
- `v2-test-plan.md` — live-test-procedure for M3+M4 (share-flow + accept-flow)

## Planlagte dokumenter

- `protocol.md` — fuldt HTTP-API + JSON-skemaer
- `crypto.md` — XChaCha20-Poly1305 + HKDF-detaljer, nøgleafledning
- `operations.md` — deployment, backup, migration, log-håndtering
- `admin-guide.md` — admin-CLI og admin-API
