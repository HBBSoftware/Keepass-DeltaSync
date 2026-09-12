# Enhedens livscyklus: sletning, re-enroll og nøglen der følger med

Status: **DESIGN / forslag** (2026-09-13). Ingen kode endnu. Parkeret sammen
med [backup og restore](backup-restore.md); TrueNAS-katalog-appen
(truenas/apps#5766) står foran begge.

Anledningen var konkret: Android-appen blev slettet under test, og dermed
forsvandt enhedens private nøgle. Serveren har stadig rækken i `devices`.
Spørgsmålet var, om en eksisterende terminal kan udstede et nyt enrollment, og
om web-panelet kan rydde den døde enhed væk.

## Hvad der virker i dag

Den forældede række spærrer ingenting. En bruger må have vilkårligt mange
enheder, så vejen frem er admin-panelet, fanen Users, knappen Enroll token.
Den gamle enhed kan blive liggende imens.

To brikker findes allerede:

| Rute | Auth | Findes |
|---|---|---|
| `DELETE /api/v1/devices/{id}` | `device` | ja, afgrænset til samme bruger |
| `POST /api/v1/admin/users/{id}/enrollment` | `admin` | ja |
| `GET /api/v1/admin/devices` | `admin` | ja, kun læsning |
| sletning fra admin | `admin` | **nej** |
| enrollment udstedt af en enhed | `device` | **nej** |

Terminal-klienten har allerede den første som `keepass-deltasync devices
remove <id>`, så oprydning kan ske i dag. Bare ikke fra web.

## Forslag 1: sletning af enhed i admin-panelet

En `DELETE /api/v1/admin/devices/{id}` og en knap i fanen Devices.

Det er en lille tilføjelse og **ingen ny magt**. Admin kan i forvejen slette
hele brugeren med kaskade over enheder, databaser og entries, så sletning af
én enhed er strengt mindre destruktiv end det der allerede kan lade sig gøre.
Det er kun finere granularitet.

Det ene der kræver omtanke er revisionssporet. `DeviceController::destroy`
logger i dag `revoked_by_device_id` og `self_revoke`, hvilket ikke giver
mening når afsenderen er en admin-session. Admin-varianten skal navngive
admin, ikke en enhed, ellers bliver loggen misvisende præcis når den skal
bruges.

## Forslag 2: en enhed må forny sin egen bruger

En rute hvor et enheds-token kan udstede et enrollment-token til sin **egen**
bruger.

Sikkerhedsmæssigt flytter det mindre end det lyder. En kompromitteret enhed
har allerede fuld adgang til brugerens data og kan allerede slette brugerens
øvrige enheder. Den vinder altså ikke nye rettigheder, den sparer et opkald
til en administrator. Det svarer til at man kan tilføje en enhed til sin egen
konto uden at ringe til nogen, hvilket er normalt for den slags tjenester.

Det der bør lægges på: kort levetid, engangsbrug, og en post i loggen om
hvilken enhed der udstedte tokenet. Uden det sidste kan en tilføjet enhed ikke
spores tilbage til den enhed der bad om den.

Bemærk at dette **ikke** løser den slettede app i sig selv. Var telefonen den
eneste enhed, er der ingen tilbage til at udstede noget, og så er admin den
eneste vej. Forslaget hjælper den der har både telefon og computer, hvilket er
det almindelige tilfælde.

## Det egentlige problem: nøglen hænger på enheden, ikke på brugeren

`database_members` har én `wrapped_master_key` per database og **bruger**. Men
indpakningen sker til en enkelt **enheds** X25519-nøgle:
`ShareController::lookupUser` vælger modtagerens nyeste enhed med en
`public_key` og returnerer den som mål.

De to ting passer ikke sammen, og det er her det gør ondt:

| Database | Efter re-enroll med nyt nøglepar |
|---|---|
| Dem brugeren selv ejer | virker, master-kodeordet indtastes af brugeren |
| Dem andre har delt med brugeren | kan ikke åbnes, ejeren skal dele igen |

Det rammer også **uden** sletning. Enroller man en ny enhed, bliver den den
nyeste med en offentlig nøgle, og fremtidige delinger peger på den. Den gamle
enhed kan stadig åbne det den allerede har fået, men de to enheder får ikke
automatisk fælles adgang til nye delinger. To enheder under samme bruger er
altså ikke ligestillede, selvom brugerfladen siger at en brugers enheder ser
de samme databaser.

Tre veje ud, fra billigst til dyrest:

1. **Wrap til alle brugerens enheder.** `database_members` får en række per
   enhed i stedet for per bruger, eller en sideordnet tabel gør det. Ejeren
   pakker ind én gang per modtager-enhed. Ulempen er at en ny enhed stadig
   kræver at ejeren gør noget, for ejeren er den eneste der kan pakke ind.
2. **En nøgle per bruger, ikke per enhed.** Brugeren får ét nøglepar, og hver
   enhed får en kopi af den private nøgle ved enrollment. Så overlever
   delinger et enhedsskift af sig selv. Prisen er at den private nøgle nu
   rejser mellem enheder, og at enrollment-tokenet dermed bærer noget
   hemmeligt. Det ændrer trusselsmodellen og skal holdes op mod
   [threat-model.md](threat-model.md).
3. **Genindpakning på forespørgsel.** Den nye enhed beder om adgang, og
   ejerens klient pakker ind næste gang den er online. Mest korrekt, men
   kræver en tilstand at vente i og en brugerflade til at godkende.

Valget hører sammen med backup og restore, for det er samme spørgsmål stillet
to gange: hvad ejer en nøgle, enheden eller brugeren. Svares der forskelligt
de to steder, bliver en gendannet server ikke den samme server.

## Rækkefølge

Forslag 1 og 2 er små og uafhængige af nøglespørgsmålet. De kan laves når som
helst. Nøglemodellen bør derimod besluttes sammen med backup-planen, ikke før.
