# Backup og restore

Status: **DESIGN / forslag** (2026-09-09). Ingen kode endnu, og ikke det næste
der bygges: F-Droid-indsendelsen (fdroiddata!41661) og TrueNAS-katalog-appen
(truenas/apps#5766) skal i mål først.

## Hvad en backup skal indeholde

Databasen har 14 tabeller. De falder i tre grupper.

**Skal med**, ellers er resultatet ikke den samme server:

`users`, `devices`, `databases`, `database_members`, `entries`,
`entry_versions`, `database_seq`, `admin_account`, `admin_tokens`.

**Bør udelades:**

- `admin_sessions` — ville genoplive browsersessioner, der burde være døde
  sammen med den gamle server.
- `auth_attempts` — rate-limit-tællere, der kun betyder noget i det minut de
  blev skrevet.

**Valgfri:** `audit_log`. Den er stor, den slettes alligevel efter
`AUDIT_RETENTION_DAYS`, og den rummer det mest følsomme klartekst i hele
dumpet — IP-adresser og user agents. Bør være et tilvalg, ikke en selvfølge.

`enrollment_tokens` og `system_state` kan tages med eller ej; de er
kortlivede og harmløse begge veje.

## To felter der lydløst ødelægger alt hvis de tabes

**`devices.token_hash`.** Uden den skal hver enkelt enhed enrolles på ny.

**`database_seq.next_seq`.** Klienterne husker hvor langt de er nået og spørger
`/changes?since=N`. Restaureres der uden tælleren — eller nulstilles den —
står hver klients markør højere end serverens, og `/changes` returnerer
**tomt for evigt**. Ingen fejl, ingen advarsel, bare en synkronisering der er
holdt op med at virke.

Det er hovedargumentet for at bruge `pg_dump` frem for en håndskrevet eksport:
den håndterer sekvenser korrekt. Og den ligger allerede i imaget —
`postgresql-client` installeres i forvejen til migrationerne.

## Hvorfor en backup her er mindre farlig end normalt

Blobs er klient-krypteret ciphertext. `wrapped_master_key` er forseglet til en
enheds X25519-nøgle. Device- og admin-tokens lagres som SHA-256, som ikke kan
vendes om. Admin-kodeordet er Argon2id.

Lækker filen, får angriberen brugernavne, enhedsnavne og tidsstempler — ikke
indholdet af nogens KeePass-database. Det er et argument for at gøre backup
let tilgængelig frem for at gemme den bag advarsler.

Undtagelsen er `audit_log` med sine IP-adresser. Endnu en grund til at den
skal være et tilvalg.

## Restore som en del af opsætningen

Den første tanke var at holde restore ude af browseren helt, fordi den er
destruktiv på en kørende database, fordi uploaden er hele serverens tilstand
gennem én request, og især fordi handlingen **logger sig selv ud undervejs**:
`admin_account` ligger i backuppen, så i det øjeblik den er indlæst, er
legitimationen måske en anden end den, sessionen blev oprettet med.

En bedre model fjerner de to første helt: **tilbyd kun restore på en server,
der aldrig har været taget i brug.** Er der data, findes muligheden ikke. Det
er en enkeltrettet dør frem for en advarselsdialog, og det placerer restore
dér hvor den hører hjemme — i opsætningen, ikke blandt de administrative
handlinger.

To ting i opstartsforløbet skal der designes omkring:

**Databasen er aldrig helt tom.** Sættes `ADMIN_USERNAME` og `ADMIN_PASSWORD`,
opretter entrypointet admin-kontoen før nogen åbner browseren. Porten kan
derfor ikke være "ingen rækker nogen steder". Den skal være **ingen brugere og
ingen databaser** — migreret klar, men aldrig brugt.

**`admin:ensure` kører ved hver opstart.** Indeholder restoren `admin_account`,
overskrives den med env-værdierne ved næste genstart. Det ligner en restore,
der gik tabt, men er bare opstarten, der gør sit arbejde igen.

Det peger på en afklaring, som gør modellen renere: **admin-identiteten hører
til installationen, ikke til dataene.** `admin_account` udelades derfor af
restoren, og den nye server beholder sit eget admin-login fra env.
`admin_tokens` er en anden sag — de bruges af maskiner, og at bevare dem gør
migreringen usynlig for CLI-klienter.

## Forslaget

**Eksport i admin-panelet.** En knap der streamer `pg_dump` som fil, med et
flueben til at tage audit-loggen med. Det rammer TrueNAS-brugere, der ikke har
shell-vaner.

**Restore i opsætningen.** Kun synlig når der hverken er brugere eller
databaser. Efter en vellykket restore forsvinder muligheden, fordi
forudsætningen ikke længere er opfyldt.

## Det backuppen ikke løser

Flyttes serveren til en ny adresse, skal **hver klient pege et nyt sted hen**.
Device-tokenet overlever, men server-URL'en står i klientens egen config, og
der findes ingen mekanisme til at flytte den.

Det er argumentet for et **DNS-navn frem for en IP** fra starten. Med et navn
er en serverflytning en DNS-ændring; med `192.168.2.60:30486` er det en tur
rundt til alle enheder.

Men det er ikke en blindgyde. Har man brugt en IP, kan enhederne **enrolles på
ny** mod den nye adresse og køre videre — databaserne ligger på serveren og
hentes ned igen. To ting er værd at vide om den vej:

- Enheden får et nyt device-token og et nyt X25519-nøglepar. Den gamle række
  bliver stående, indtil den fjernes i panelet.
- For **egne** databaser er det uproblematisk: masternøglen udledes lokalt fra
  adgangskoden, så en ny enhed udleder den samme. For **delte** databaser er
  `wrapped_master_key` forseglet til den gamle enheds nøgle, så ejeren skal
  dele på ny.

Med andre ord koster en IP-flytning en runde enrollments, ikke tabte data.

## Forhold til v5

[`v5-multi-server-sync.md`](v5-multi-server-sync.md) stiller mange af de samme
spørgsmål fra en anden vinkel: hvad udgør en server, og hvor meget identitet er
bundet til den enkelte installation. Konklusionen her — at admin-identiteten
hører til installationen og dataene ikke gør — er en brik, som også hører
hjemme dér.
