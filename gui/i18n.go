// SPDX-License-Identifier: GPL-3.0-or-later

package main

// i18n holder en meget enkel to-sprogs-ordbog (dansk/engelsk). GUI'en er
// bevidst en tynd skal oven på keepass-deltasync-CLI'en, så vi har kun brug
// for et lille sæt strenge. Default er dansk; brugeren kan skifte til engelsk
// i Indstillinger, og valget gemmes i gui.json sammen med stien til CLI'en.

type lang string

const (
	langDA lang = "da"
	langEN lang = "en"
)

// L er den aktive ordbog. Sættes ved opstart fra gemt indstilling og opdateres
// når brugeren skifter sprog.
var L = dicts[langDA]

func setLang(l lang) {
	if d, ok := dicts[l]; ok {
		L = d
	}
}

// dict er alle de strenge UI'en bruger. Felterne er navngivet efter hvor de
// optræder, så det er nemt at finde dem i koden.
type dict struct {
	AppTitle string

	// Generelt
	OK, Cancel, Close, Save, Back, Next, Finish, Browse string
	Working, Done, Error                                string
	Copy, DBInfoTitle                                   string

	// CLI-lokalisering
	CLINotFound     string
	CLILocate       string
	CLILocateHint   string
	CLIPathLabel    string
	CLISelectBinary string

	// Wizard
	WizardWelcomeTitle string
	WizardWelcomeBody  string
	WizardStart        string
	WizardStepEnroll   string
	WizardStepDB       string
	ServerURL          string
	ServerURLHint      string
	EnrollToken        string
	EnrollTokenHint    string
	DeviceName         string
	DeviceNameHint     string
	EnrollButton       string
	EnrollOK           string
	WizardAddDB        string
	WizardAddDBBody    string
	DBName             string
	DBNameHint         string
	KdbxFile           string
	KdbxFileHint       string
	CreateDBButton     string
	SkipForNow         string
	WizardDone         string
	WizardDoneBody     string

	// Avanceret tilmelding (administrator udsteder token + enroller PC'en)
	WizardAdvanced     string
	WizardStepAdvanced string
	AdvancedIntro      string
	AdvUserMode        string
	AdvExistingUser    string
	AdvNewUser         string
	AdvEnrollButton    string
	AdvIssuingToken    string
	AdvEnrolling       string
	AdvNoTokenErr      string

	// Dashboard
	TabDatabases string
	TabDevices   string
	TabActivity  string
	TabLog       string
	TabAdmin     string
	TabSettings  string
	ActivityHint string
	Clear        string

	// Log-fane (server-audit-log)
	LogHint         string
	LogCount        string
	LogEmpty        string
	LogPeriodLabel  string
	LogPeriod24h    string
	LogPeriod7d     string
	LogPeriod30d    string
	LogPeriodAll    string
	LogDetailsTitle string
	LogColTime      string
	LogColEvent     string
	LogColLevel     string
	LogColIP        string
	LogOK           string
	LogFail         string

	// Enheder-fane
	ColEnrolled     string
	ColLastSeen     string
	ThisDevice      string
	DevCount        string // "%d"
	DeviceInfoTitle string

	AddDevice           string
	AddDeviceCreate     string
	RemoveDevice        string
	Username            string
	AdminToken          string
	EnrollTokenCreated  string
	SelectDeviceFirst   string
	CannotRemoveCurrent string
	ConfirmRemoveDevice string // "%s" = enhedsnavn
	StatusBox           string
	NotEnrolled         string
	Refresh             string
	Sync                string
	SyncSelected        string
	SyncAll             string
	SelectFirst         string
	AddDatabase         string
	ForgetDatabase      string
	ConfirmForget       string // "%s" = db-navn
	MoreActions         string
	PushNow             string
	PullNow             string
	DeleteOnServer      string
	ConfirmDeleteServer string // "%q" = db-navn
	NoDatabases         string
	DBCount             string
	ColName             string
	ColStatus           string
	ColID               string
	ColCreated          string
	ColPath             string
	BoundLocally        string
	OnServerOnly        string

	// Detalje-panel: medlemmer/enheder koblet til den valgte database.
	MembersTitle       string
	MembersOf          string // "%s" = db-navn
	SelectToSeeMembers string
	MemberCount        string // "%d"
	MembersNeedBound   string
	MembersUnavailable string
	NoMembers          string
	ColRole            string
	ColUser            string
	ColDisplay         string
	ColAdded           string

	// Deling (share / unshare / init-shared)
	ShareDatabase     string
	ShareTitle        string // "%s" = db-navn
	ShareWith         string
	RemoveMember      string
	SelectMemberFirst string
	CannotRemoveOwner string
	ConfirmUnshare    string // "%s" = bruger, "%q" = db
	SetupShared       string
	SetupSharedTitle  string // "%s" = remote-navn
	AlreadyLocal      string
	NewLocalPassword  string
	BindExisting      string
	BoundNowSync      string
	MasterPwd         string
	MasterPwdFor      string
	Language          string
	ThemeLabel        string
	ThemeSystem       string
	ThemeLight        string
	ThemeDark         string
	HelpPanelLabel    string
	HelpPanelDesc     string
	HelpTitle         string
	HelpDatabases     string
	HelpDevices       string
	HelpActivity      string
	HelpLog           string
	HelpAdmin         string
	HelpSettings      string
	ResetEnroll       string

	// Autostart (Indstillinger): OS-opsætning der kører `daemon` ved login.
	AutostartTitle       string
	AutostartDesc        string
	AutostartEnable      string
	AutostartDisable     string
	AutostartOn          string
	AutostartOff         string
	AutostartUnsupported string
	AutostartNoCLI       string
	AutostartEnabled     string
	AutostartDisabled    string

	// Administration-fane
	AdminHint         string
	AdminNeedToken    string
	AdminLoadUsers    string
	AdminCreateUser   string
	AdminTokenHelp    string
	AdminUserCount    string // "%d"
	AdminNewEnroll    string
	AdminEnable       string
	AdminDisable      string
	AdminDeleteUser   string
	AdminDisplayName  string
	ConfirmDeleteUser string // "%q" = brugernavn

	// Versioner / gendan
	VersionsMenu    string
	VersionsTitle   string // "%s" = db-navn
	VersionsHint    string
	EntryUUID       string
	EntryUUIDHint   string
	VersionsShow    string
	VersionsCount   string // "%d"
	VersionsNone    string
	VersionsRestore string
	ConfirmRestore  string // "%s" = version, "%q" = db-navn
	RestoreDoneSync string
}

var dicts = map[lang]*dict{
	langDA: {
		AppTitle: "KeePass Delta-Sync",

		OK: "OK", Cancel: "Annullér", Close: "Luk", Save: "Gem",
		Back: "Tilbage", Next: "Næste", Finish: "Færdig", Browse: "Vælg…",
		Working: "Arbejder…", Done: "Færdig", Error: "Fejl",
		Copy: "Kopiér", DBInfoTitle: "Database-info",

		CLINotFound:     "Kan ikke finde programmet 'keepass-deltasync'.",
		CLILocate:       "Find keepass-deltasync",
		CLILocateHint:   "Peg på keepass-deltasync-programmet (CLI'en). Det leveres normalt ved siden af denne app.",
		CLIPathLabel:    "Sti til CLI",
		CLISelectBinary: "Vælg keepass-deltasync-programmet",

		WizardWelcomeTitle: "Velkommen",
		WizardWelcomeBody: "Denne guide hjælper dig i gang med at synkronisere din KeePass-database " +
			"mellem dine enheder.\n\nDu skal bruge:\n  • Server-adressen fra din administrator\n  • Et enrollment-token (engangskode)\n  • Din .kdbx-fil",
		WizardStart:      "Start guide",
		WizardStepEnroll: "Trin 1 af 2 — Tilmeld enhed",
		WizardStepDB:     "Trin 2 af 2 — Tilføj database",
		ServerURL:        "Server-adresse",
		ServerURLHint:    "F.eks. https://deltasync.example.dk",
		EnrollToken:      "Enrollment-token",
		EnrollTokenHint:  "Engangskoden du fik af din administrator",
		DeviceName:       "Enhedsnavn (valgfrit)",
		DeviceNameHint:   "Vises i administrationen. Tomt = computerens navn.",
		EnrollButton:     "Tilmeld denne enhed",
		EnrollOK:         "Enheden er tilmeldt!",
		WizardAddDB:      "Tilføj din første database",
		WizardAddDBBody:  "Registrér en lokal .kdbx-fil til synkronisering. Du kan altid tilføje flere senere.",
		DBName:           "Navn",
		DBNameHint:       "Et kort navn, f.eks. 'privat' eller 'arbejde'",
		KdbxFile:         "KeePass-fil (.kdbx)",
		KdbxFileHint:     "Stien til din lokale database",
		CreateDBButton:   "Opret database",
		SkipForNow:       "Spring over",
		WizardDone:       "Alt klar!",
		WizardDoneBody:   "Du er klar til at synkronisere. Brug knappen 'Synkronisér' på en database for at sende og hente ændringer.",

		WizardAdvanced:     "Avanceret (administrator)…",
		WizardStepAdvanced: "Avanceret tilmelding — administrator",
		AdvancedIntro: "Har du et admin-token, kan du udstede et enrollment-token og tilmelde denne PC i ét hug — " +
			"du behøver ikke et token på forhånd. Vælg en eksisterende bruger, eller opret en ny.",
		AdvUserMode:     "Bruger",
		AdvExistingUser: "Eksisterende bruger",
		AdvNewUser:      "Opret ny bruger",
		AdvEnrollButton: "Udsted token og tilmeld denne PC",
		AdvIssuingToken: "Udsteder enrollment-token…",
		AdvEnrolling:    "Tilmelder denne PC…",
		AdvNoTokenErr:   "Kunne ikke udlæse enrollment-tokenet fra serverens svar. Se loggen for detaljer.",

		TabDatabases: "Databaser",
		TabDevices:   "Enheder",
		TabActivity:  "Aktivitet",
		TabLog:       "Log",
		TabAdmin:     "Administration",
		TabSettings:  "Indstillinger",
		ActivityHint: "Output fra CLI-kald (sync, tilføj/fjern, fejl …) vises her.",
		Clear:        "Ryd",

		LogHint:         "Server-aktivitetslog (audit) — historik på tværs af enheder, gemt i op til 30 dage.",
		LogCount:        "%d log-poster",
		LogEmpty:        "(ingen log-poster i perioden)",
		LogPeriodLabel:  "Periode:",
		LogPeriod24h:    "Seneste 24 timer",
		LogPeriod7d:     "Seneste 7 dage",
		LogPeriod30d:    "Seneste 30 dage",
		LogPeriodAll:    "Alle",
		LogDetailsTitle: "Log-detaljer",
		LogColTime:      "Tidspunkt",
		LogColEvent:     "Hændelse",
		LogColLevel:     "Niveau",
		LogColIP:        "IP-adresse",
		LogOK:           "OK",
		LogFail:         "Fejlede",

		ColEnrolled:     "Tilmeldt",
		ColLastSeen:     "Sidst set",
		ThisDevice:      "● denne enhed",
		DevCount:        "%d enhed(er) på kontoen",
		DeviceInfoTitle: "Enheds-info",

		AddDevice:           "Tilføj enhed",
		AddDeviceCreate:     "Generér token",
		RemoveDevice:        "Fjern enhed",
		Username:            "Brugernavn",
		AdminToken:          "Admin-token",
		EnrollTokenCreated:  "Enrollment-token til ny enhed",
		SelectDeviceFirst:   "Vælg først en enhed i listen.",
		CannotRemoveCurrent: "Du kan ikke fjerne den enhed du bruger lige nu herfra.",
		ConfirmRemoveDevice: "Fjern (tilbagekald) enheden %q? Dens token bliver ugyldig.",
		StatusBox:           "Status",
		NotEnrolled:         "Ikke tilmeldt endnu.",
		Refresh:             "Opdatér",
		Sync:                "Synkronisér",
		SyncSelected:        "Synkronisér valgte",
		SyncAll:             "Synkronisér alle",
		SelectFirst:         "Vælg først en database i listen.",
		AddDatabase:         "Tilføj database",
		ForgetDatabase:      "Glem database",
		ConfirmForget:       "Glem den lokale binding for %q? Selve .kdbx-filen og databasen på serveren røres IKKE — kun koblingen i denne klient fjernes.",
		MoreActions:         "Flere handlinger",
		PushNow:             "Push nu (kun upload)",
		PullNow:             "Pull nu (kun download)",
		DeleteOnServer:      "Slet på server",
		ConfirmDeleteServer: "Slet databasen %q PERMANENT på serveren?\n\nDette fjerner ALLE entries, versioner, delinger og historik — for alle brugere. Handlingen kan IKKE fortrydes. Din lokale .kdbx-fil røres ikke.",
		NoDatabases:         "Ingen databaser endnu. Klik 'Tilføj database' for at komme i gang.",
		DBCount:             "%d database(r) tilkoblet",
		ColName:             "Navn",
		ColStatus:           "Status",
		ColID:               "ID",
		ColCreated:          "Oprettet",
		ColPath:             "Lokal sti",
		BoundLocally:        "● klar",
		OnServerOnly:        "○ kun på server",

		MembersTitle:       "Koblet til databasen",
		MembersOf:          "Koblet til %q",
		SelectToSeeMembers: "Vælg en database ovenfor for at se hvem der er koblet til den.",
		MemberCount:        "%d medlem(mer)",
		MembersNeedBound:   "Databasen skal være sat op lokalt for at vise medlemmer.",
		MembersUnavailable: "Kan ikke hente medlemmer (kun ejeren kan se dette):",
		NoMembers:          "Kun dig — ikke delt med andre endnu.",
		ColRole:            "Rolle",
		ColUser:            "Brugernavn",
		ColDisplay:         "Visningsnavn",
		ColAdded:           "Tilføjet",

		ShareDatabase:     "Del database",
		ShareTitle:        "Del %q med en bruger",
		ShareWith:         "Del med (brugernavn)",
		RemoveMember:      "Fjern medlem",
		SelectMemberFirst: "Vælg først et medlem i listen.",
		CannotRemoveOwner: "Ejeren kan ikke fjernes.",
		ConfirmUnshare:    "Fjern %s fra %q?",
		SetupShared:       "Sæt op lokalt",
		SetupSharedTitle:  "Sæt den delte database %q op lokalt",
		AlreadyLocal:      "Databasen er allerede sat op lokalt.",
		NewLocalPassword:  "Nyt lokalt password",
		BindExisting:      "Forbind eksisterende .kdbx (egen database, ny enhed)",
		BoundNowSync:      "Forbundet! Klik på ⟳ (Synkronisér) for at hente entries.",
		MasterPwd:         "Masterpassword",
		MasterPwdFor:      "Masterpassword for",
		Language:          "Sprog",
		ThemeLabel:        "Tema",
		ThemeSystem:       "System (følg styresystem)",
		ThemeLight:        "Lyst",
		ThemeDark:         "Mørkt",
		HelpPanelLabel:    "Vis hjælpe-panel",
		HelpPanelDesc:     "Når slået til, vises et felt i bunden af vinduet med en beskrivelse af den fane du er på — hvad siden gør, og hvad knapperne svarer til i keepass-deltasync-programmet.",
		HelpTitle:         "Om denne side",
		HelpDatabases: "## Databaser\n\n" +
			"Dine databaser og deres delinger. **● (fuld cirkel)** = bundet til en lokal `.kdbx`-fil og klar til synkronisering. **○ (åben cirkel)** = findes kun på serveren.\n\n" +
			"**Handlinger pr. database:**\n\n" +
			"- **Synkronisér** — send og hent ændringer (`sync`).\n" +
			"- **Del** — giv en anden bruger adgang (`share`).\n" +
			"- **Glem** — fjern kun den lokale binding; rører ikke serveren eller filen (`forget`).\n" +
			"- **⋮ Flere** — Push (kun upload), Pull (kun download), Versioner/gendan, og **Slet på server** (`delete-database`, permanent for alle).\n\n" +
			"Er en database kun på serveren: **Forbind** din egen `.kdbx` (`init --bind`) eller **Opsæt delt** kopi (`init-shared`).\n\n" +
			"Øverst: **Tilføj database** (`init`) og **Synkronisér alle**.\n\n" +
			"**CLI-kommandoer:**\n\n" +
			"- `keepass-deltasync databases`\n" +
			"- `keepass-deltasync init <navn> <sti>`\n" +
			"- `keepass-deltasync sync --password-stdin <db>`\n" +
			"- `keepass-deltasync push --password-stdin <db>` / `pull --password-stdin <db>`\n" +
			"- `keepass-deltasync share --password-stdin <db> <bruger>` / `unshare <db> <bruger>` / `shares <db>`\n" +
			"- `keepass-deltasync forget <db>` / `delete-database <db|uuid>`\n" +
			"- `keepass-deltasync init --bind <uuid> <db> <sti>` / `init-shared --password-stdin <remote> <sti>`\n" +
			"- `keepass-deltasync versions <db> <entry-uuid>` / `restore <db> <entry-uuid> <version>`",
		HelpDevices: "## Enheder\n\n" +
			"De enheder der er tilmeldt din konto (`devices`). Den **● markerede** er den du bruger nu.\n\n" +
			"- **Tilføj enhed** — generér en enrollment-token som en ny enhed bruger til at tilmelde sig (`admin user-enrollment`).\n" +
			"- **Fjern** — tilbagekald en enheds adgang (`devices remove`). Du kan ikke fjerne den enhed du sidder ved.\n" +
			"- **Info** — se enhedens ID, samt tilmeldt- og sidst set-tidspunkt.\n\n" +
			"**CLI-kommandoer:**\n\n" +
			"- `keepass-deltasync devices`\n" +
			"- `keepass-deltasync devices remove <id>`\n" +
			"- `keepass-deltasync admin user-enrollment <bruger>`",
		HelpActivity: "## Aktivitet\n\n" +
			"Rå output fra de CLI-kald GUI'en laver i **denne session** (sync, tilføj/fjern, fejl …). Det nulstilles når appen lukkes.\n\n" +
			"Vil du se historik på tværs af sessioner, så brug **Log**-fanen, der henter serverens audit-log.\n\n" +
			"*Ingen egen CLI-kommando — fanen viser blot output fra de øvrige kommandoer.*",
		HelpLog: "## Log\n\n" +
			"Serverens **audit-log** (`log`) — en vedvarende historik over hændelser på din konto (login, sync, ændringer …), på tværs af alle dine enheder, gemt i op til 30 dage.\n\n" +
			"- **Periode** styrer hvor langt tilbage der hentes (`--since`).\n" +
			"- Hver linje viser tid, hændelse, OK/fejl, niveau og IP. Klik **ℹ** for fulde detaljer.\n\n" +
			"**CLI-kommando:**\n\n" +
			"- `keepass-deltasync log --since <varighed> --limit <n>`",
		HelpAdmin: "## Administration\n\n" +
			"Brugeradministration. Kræver en **admin-token**, der kun holdes i hukommelsen og aldrig gemmes på disk.\n\n" +
			"- **Hent brugere** — liste over alle brugere (`admin user-list`).\n" +
			"- **Opret bruger** — opret + få en enrollment-token (`admin user-create`).\n" +
			"- Pr. bruger: **ny enrollment-token** (`user-enrollment`), **aktivér/deaktivér** (`user-enable` / `user-disable`), **slet** (`user-delete`, CASCADE).\n" +
			"- **Hent admin-token (SQL)** — SQL til at oprette en frisk admin-token (`admin token-sql`); kør den i DBeaver.\n\n" +
			"**CLI-kommandoer:**\n\n" +
			"- `keepass-deltasync admin user-list`\n" +
			"- `keepass-deltasync admin user-create <bruger> --display-name <navn>`\n" +
			"- `keepass-deltasync admin user-enrollment <bruger>`\n" +
			"- `keepass-deltasync admin user-enable <bruger>` / `user-disable <bruger>`\n" +
			"- `keepass-deltasync admin user-delete <bruger> --yes`\n" +
			"- `keepass-deltasync admin token-sql`",
		HelpSettings: "## Indstillinger\n\n" +
			"- **Sprog** — dansk eller engelsk.\n" +
			"- **Tema** — System (følg styresystemet), Lyst eller Mørkt.\n" +
			"- **Sti til CLI** — hvor `keepass-deltasync`-programmet ligger. GUI'en kalder det til alt arbejde.\n" +
			"- **Vis hjælpe-panel** — dette felt.\n" +
			"- **Autostart** — få styresystemet til at køre `keepass-deltasync daemon` ved login, så baggrunds-synkroniseringen kører uden at GUI'en er åben. Opsættes i dit eget brugerområde (en \"ved logon\"-opgave i Opgaveplanlægning på Windows, launchd på macOS, systemd --user på Linux) — ingen administrator nødvendig. På Windows venter opgaven kort på netværket og genstarter daemonen hvis den dør. GUI'en kører aldrig selv daemonen.\n\n" +
			"GUI'en er en skal oven på `keepass-deltasync`-kommandolinjen; al krypto, server-kald og config ligger i CLI'en.\n\n" +
			"**Nyttige CLI-kommandoer:**\n\n" +
			"- `keepass-deltasync status`\n" +
			"- `keepass-deltasync enroll --server <url> <token>`",
		ResetEnroll: "Nulstil tilmelding",

		AutostartTitle: "Autostart",
		AutostartDesc: "Kør baggrunds-synkroniseringen (`keepass-deltasync daemon`) automatisk ved login, " +
			"uden at denne app skal være åben. Opsættes i dit eget brugerområde — ingen administrator nødvendig.",
		AutostartEnable:      "Aktivér autostart",
		AutostartDisable:     "Deaktivér autostart",
		AutostartOn:          "Autostart er aktiv — daemonen starter ved login.",
		AutostartOff:         "Autostart er ikke aktiv.",
		AutostartUnsupported: "Autostart understøttes ikke på dette styresystem.",
		AutostartNoCLI:       "Find først keepass-deltasync-programmet (Sti til CLI ovenfor).",
		AutostartEnabled:     "Autostart aktiveret.",
		AutostartDisabled:    "Autostart deaktiveret.",

		AdminHint:         "Brugeradministration. Kræver en admin-token (gemmes ikke på disk).",
		AdminNeedToken:    "Indtast en admin-token og klik 'Hent brugere'.",
		AdminLoadUsers:    "Hent brugere",
		AdminCreateUser:   "Opret bruger",
		AdminTokenHelp:    "Hent admin-token (SQL)",
		AdminUserCount:    "%d bruger(e)",
		AdminNewEnroll:    "Ny enrollment-token",
		AdminEnable:       "Aktivér bruger",
		AdminDisable:      "Deaktivér bruger",
		AdminDeleteUser:   "Slet bruger",
		AdminDisplayName:  "Visningsnavn (valgfrit)",
		ConfirmDeleteUser: "Slet brugeren %q PERMANENT?\n\nCASCADE: alle brugerens enheder, databaser og entries fjernes. Handlingen kan IKKE fortrydes.",

		VersionsMenu:    "Versioner / gendan…",
		VersionsTitle:   "Versioner — %s",
		VersionsHint:    "Indsæt en entry-UUID for at se dens bevarede versioner (op til 3).",
		EntryUUID:       "Entry-UUID",
		EntryUUIDHint:   "UUID på den entry du vil se versioner for",
		VersionsShow:    "Vis versioner",
		VersionsCount:   "%d version(er)",
		VersionsNone:    "Ingen versioner — entry ikke fundet.",
		VersionsRestore: "Gendan",
		ConfirmRestore:  "Gendan version %s som ny nyeste version i %q?\n\nServeren opretter en ny version ud fra den valgte. Kør 'Synkronisér' (eller Pull) bagefter for at hente ændringen ned i din lokale fil.",
		RestoreDoneSync: "Versionen er gendannet på serveren. Kør 'Synkronisér' (eller Pull) på databasen for at hente ændringen ned.",
	},
	langEN: {
		AppTitle: "KeePass Delta-Sync",

		OK: "OK", Cancel: "Cancel", Close: "Close", Save: "Save",
		Back: "Back", Next: "Next", Finish: "Finish", Browse: "Browse…",
		Working: "Working…", Done: "Done", Error: "Error",
		Copy: "Copy", DBInfoTitle: "Database info",

		CLINotFound:     "Could not find the 'keepass-deltasync' program.",
		CLILocate:       "Locate keepass-deltasync",
		CLILocateHint:   "Point to the keepass-deltasync program (the CLI). It usually ships next to this app.",
		CLIPathLabel:    "Path to CLI",
		CLISelectBinary: "Select the keepass-deltasync program",

		WizardWelcomeTitle: "Welcome",
		WizardWelcomeBody: "This guide helps you start syncing your KeePass database " +
			"across your devices.\n\nYou will need:\n  • The server address from your administrator\n  • An enrollment token (one-time code)\n  • Your .kdbx file",
		WizardStart:      "Start guide",
		WizardStepEnroll: "Step 1 of 2 — Enroll device",
		WizardStepDB:     "Step 2 of 2 — Add database",
		ServerURL:        "Server address",
		ServerURLHint:    "e.g. https://deltasync.example.dk",
		EnrollToken:      "Enrollment token",
		EnrollTokenHint:  "The one-time code from your administrator",
		DeviceName:       "Device name (optional)",
		DeviceNameHint:   "Shown in admin listings. Empty = computer name.",
		EnrollButton:     "Enroll this device",
		EnrollOK:         "Device enrolled!",
		WizardAddDB:      "Add your first database",
		WizardAddDBBody:  "Register a local .kdbx file for syncing. You can always add more later.",
		DBName:           "Name",
		DBNameHint:       "A short name, e.g. 'personal' or 'work'",
		KdbxFile:         "KeePass file (.kdbx)",
		KdbxFileHint:     "Path to your local database",
		CreateDBButton:   "Create database",
		SkipForNow:       "Skip for now",
		WizardDone:       "All set!",
		WizardDoneBody:   "You're ready to sync. Use the 'Sync' button on a database to send and fetch changes.",

		WizardAdvanced:     "Advanced (administrator)…",
		WizardStepAdvanced: "Advanced enrollment — administrator",
		AdvancedIntro: "If you have an admin token, you can issue an enrollment token and enroll this PC in one step — " +
			"no token needed up front. Pick an existing user, or create a new one.",
		AdvUserMode:     "User",
		AdvExistingUser: "Existing user",
		AdvNewUser:      "Create new user",
		AdvEnrollButton: "Issue token & enroll this PC",
		AdvIssuingToken: "Issuing enrollment token…",
		AdvEnrolling:    "Enrolling this PC…",
		AdvNoTokenErr:   "Could not read the enrollment token from the server's response. See the log for details.",

		TabDatabases: "Databases",
		TabDevices:   "Devices",
		TabActivity:  "Activity",
		TabLog:       "Log",
		TabAdmin:     "Administration",
		TabSettings:  "Settings",
		ActivityHint: "Output from CLI calls (sync, add/remove, errors …) shows here.",
		Clear:        "Clear",

		LogHint:         "Server activity log (audit) — history across devices, kept for up to 30 days.",
		LogCount:        "%d log entries",
		LogEmpty:        "(no log entries in this period)",
		LogPeriodLabel:  "Period:",
		LogPeriod24h:    "Last 24 hours",
		LogPeriod7d:     "Last 7 days",
		LogPeriod30d:    "Last 30 days",
		LogPeriodAll:    "All",
		LogDetailsTitle: "Log details",
		LogColTime:      "Time",
		LogColEvent:     "Event",
		LogColLevel:     "Level",
		LogColIP:        "IP address",
		LogOK:           "OK",
		LogFail:         "Failed",

		ColEnrolled:     "Enrolled",
		ColLastSeen:     "Last seen",
		ThisDevice:      "● this device",
		DevCount:        "%d device(s) on the account",
		DeviceInfoTitle: "Device info",

		AddDevice:           "Add device",
		AddDeviceCreate:     "Generate token",
		RemoveDevice:        "Remove device",
		Username:            "Username",
		AdminToken:          "Admin token",
		EnrollTokenCreated:  "Enrollment token for new device",
		SelectDeviceFirst:   "Select a device in the list first.",
		CannotRemoveCurrent: "You can't remove the device you're currently using from here.",
		ConfirmRemoveDevice: "Remove (revoke) device %q? Its token will become invalid.",
		StatusBox:           "Status",
		NotEnrolled:         "Not enrolled yet.",
		Refresh:             "Refresh",
		Sync:                "Sync",
		SyncSelected:        "Sync selected",
		SyncAll:             "Sync all",
		SelectFirst:         "Select a database in the list first.",
		AddDatabase:         "Add database",
		ForgetDatabase:      "Forget database",
		ConfirmForget:       "Forget the local binding for %q? The .kdbx file and the database on the server are NOT touched — only the binding in this client is removed.",
		MoreActions:         "More actions",
		PushNow:             "Push now (upload only)",
		PullNow:             "Pull now (download only)",
		DeleteOnServer:      "Delete on server",
		ConfirmDeleteServer: "Delete the database %q PERMANENTLY on the server?\n\nThis removes ALL entries, versions, shares and history — for every user. This action CANNOT be undone. Your local .kdbx file is left untouched.",
		NoDatabases:         "No databases yet. Click 'Add database' to get started.",
		DBCount:             "%d database(s) connected",
		ColName:             "Name",
		ColStatus:           "Status",
		ColID:               "ID",
		ColCreated:          "Created",
		ColPath:             "Local path",
		BoundLocally:        "● ready",
		OnServerOnly:        "○ server only",

		MembersTitle:       "Connected to the database",
		MembersOf:          "Connected to %q",
		SelectToSeeMembers: "Select a database above to see who is connected to it.",
		MemberCount:        "%d member(s)",
		MembersNeedBound:   "The database must be set up locally to show members.",
		MembersUnavailable: "Cannot fetch members (only the owner can see this):",
		NoMembers:          "Just you — not shared with anyone yet.",
		ColRole:            "Role",
		ColUser:            "Username",
		ColDisplay:         "Display name",
		ColAdded:           "Added",

		ShareDatabase:     "Share database",
		ShareTitle:        "Share %q with a user",
		ShareWith:         "Share with (username)",
		RemoveMember:      "Remove member",
		SelectMemberFirst: "Select a member in the list first.",
		CannotRemoveOwner: "The owner can't be removed.",
		ConfirmUnshare:    "Remove %s from %q?",
		SetupShared:       "Set up locally",
		SetupSharedTitle:  "Set up the shared database %q locally",
		AlreadyLocal:      "The database is already set up locally.",
		NewLocalPassword:  "New local password",
		BindExisting:      "Connect existing .kdbx (your own database, new device)",
		BoundNowSync:      "Connected! Click ⟳ (Sync) to fetch entries.",
		MasterPwd:         "Master password",
		MasterPwdFor:      "Master password for",
		Language:          "Language",
		ThemeLabel:        "Theme",
		ThemeSystem:       "System (follow OS)",
		ThemeLight:        "Light",
		ThemeDark:         "Dark",
		HelpPanelLabel:    "Show help panel",
		HelpPanelDesc:     "When enabled, a panel at the bottom of the window describes the tab you are on — what the page does, and what the buttons map to in the keepass-deltasync program.",
		HelpTitle:         "About this page",
		HelpDatabases: "## Databases\n\n" +
			"Your databases and their shares. **● (filled circle)** = bound to a local `.kdbx` file and ready to sync. **○ (open circle)** = exists on the server only.\n\n" +
			"**Per-database actions:**\n\n" +
			"- **Sync** — send and fetch changes (`sync`).\n" +
			"- **Share** — give another user access (`share`).\n" +
			"- **Forget** — remove the local binding only; leaves the server and file untouched (`forget`).\n" +
			"- **⋮ More** — Push (upload only), Pull (download only), Versions/restore, and **Delete on server** (`delete-database`, permanent for everyone).\n\n" +
			"If a database is server-only: **Bind** your own `.kdbx` (`init --bind`) or **Set up shared** copy (`init-shared`).\n\n" +
			"Top: **Add database** (`init`) and **Sync all**.\n\n" +
			"**CLI commands:**\n\n" +
			"- `keepass-deltasync databases`\n" +
			"- `keepass-deltasync init <name> <path>`\n" +
			"- `keepass-deltasync sync --password-stdin <db>`\n" +
			"- `keepass-deltasync push --password-stdin <db>` / `pull --password-stdin <db>`\n" +
			"- `keepass-deltasync share --password-stdin <db> <user>` / `unshare <db> <user>` / `shares <db>`\n" +
			"- `keepass-deltasync forget <db>` / `delete-database <db|uuid>`\n" +
			"- `keepass-deltasync init --bind <uuid> <db> <path>` / `init-shared --password-stdin <remote> <path>`\n" +
			"- `keepass-deltasync versions <db> <entry-uuid>` / `restore <db> <entry-uuid> <version>`",
		HelpDevices: "## Devices\n\n" +
			"The devices enrolled on your account (`devices`). The **● marked** one is the device you are using now.\n\n" +
			"- **Add device** — generate an enrollment token a new device uses to enroll (`admin user-enrollment`).\n" +
			"- **Remove** — revoke a device's access (`devices remove`). You cannot remove the device you are on.\n" +
			"- **Info** — see the device ID and its enrolled / last-seen times.\n\n" +
			"**CLI commands:**\n\n" +
			"- `keepass-deltasync devices`\n" +
			"- `keepass-deltasync devices remove <id>`\n" +
			"- `keepass-deltasync admin user-enrollment <user>`",
		HelpActivity: "## Activity\n\n" +
			"Raw output from the CLI calls the GUI makes in **this session** (sync, add/remove, errors …). It resets when the app closes.\n\n" +
			"For history across sessions, use the **Log** tab, which fetches the server's audit log.\n\n" +
			"*No CLI command of its own — the tab just shows output from the other commands.*",
		HelpLog: "## Log\n\n" +
			"The server's **audit log** (`log`) — a persistent history of events on your account (login, sync, changes …), across all your devices, kept for up to 30 days.\n\n" +
			"- **Period** controls how far back to fetch (`--since`).\n" +
			"- Each line shows time, event, OK/fail, level and IP. Click **ℹ** for full details.\n\n" +
			"**CLI command:**\n\n" +
			"- `keepass-deltasync log --since <duration> --limit <n>`",
		HelpAdmin: "## Administration\n\n" +
			"User administration. Requires an **admin token**, held only in memory and never stored on disk.\n\n" +
			"- **Load users** — list all users (`admin user-list`).\n" +
			"- **Create user** — create + get an enrollment token (`admin user-create`).\n" +
			"- Per user: **new enrollment token** (`user-enrollment`), **enable/disable** (`user-enable` / `user-disable`), **delete** (`user-delete`, CASCADE).\n" +
			"- **Get admin token (SQL)** — SQL to create a fresh admin token (`admin token-sql`); run it in DBeaver.\n\n" +
			"**CLI commands:**\n\n" +
			"- `keepass-deltasync admin user-list`\n" +
			"- `keepass-deltasync admin user-create <user> --display-name <name>`\n" +
			"- `keepass-deltasync admin user-enrollment <user>`\n" +
			"- `keepass-deltasync admin user-enable <user>` / `user-disable <user>`\n" +
			"- `keepass-deltasync admin user-delete <user> --yes`\n" +
			"- `keepass-deltasync admin token-sql`",
		HelpSettings: "## Settings\n\n" +
			"- **Language** — Danish or English.\n" +
			"- **Theme** — System (follow the OS), Light or Dark.\n" +
			"- **CLI path** — where the `keepass-deltasync` program lives. The GUI calls it for all work.\n" +
			"- **Show help panel** — this panel.\n" +
			"- **Autostart** — have the operating system run `keepass-deltasync daemon` at login, so background sync runs without the GUI being open. Set up in your own user scope (registry Run key on Windows, launchd on macOS, systemd --user on Linux) — no administrator needed. The GUI never runs the daemon itself.\n\n" +
			"The GUI is a shell over the `keepass-deltasync` command line; all crypto, server calls and config live in the CLI.\n\n" +
			"**Useful CLI commands:**\n\n" +
			"- `keepass-deltasync status`\n" +
			"- `keepass-deltasync enroll --server <url> <token>`",
		ResetEnroll: "Reset enrollment",

		AutostartTitle: "Autostart",
		AutostartDesc: "Run background sync (`keepass-deltasync daemon`) automatically at login, " +
			"without this app being open. Set up in your own user scope — no administrator needed.",
		AutostartEnable:      "Enable autostart",
		AutostartDisable:     "Disable autostart",
		AutostartOn:          "Autostart is on — the daemon starts at login.",
		AutostartOff:         "Autostart is off.",
		AutostartUnsupported: "Autostart is not supported on this operating system.",
		AutostartNoCLI:       "Locate the keepass-deltasync program first (CLI path above).",
		AutostartEnabled:     "Autostart enabled.",
		AutostartDisabled:    "Autostart disabled.",

		AdminHint:         "User administration. Requires an admin token (not stored on disk).",
		AdminNeedToken:    "Enter an admin token and click 'Load users'.",
		AdminLoadUsers:    "Load users",
		AdminCreateUser:   "Create user",
		AdminTokenHelp:    "Get admin token (SQL)",
		AdminUserCount:    "%d user(s)",
		AdminNewEnroll:    "New enrollment token",
		AdminEnable:       "Enable user",
		AdminDisable:      "Disable user",
		AdminDeleteUser:   "Delete user",
		AdminDisplayName:  "Display name (optional)",
		ConfirmDeleteUser: "Delete the user %q PERMANENTLY?\n\nCASCADE: all of the user's devices, databases and entries are removed. This action CANNOT be undone.",

		VersionsMenu:    "Versions / restore…",
		VersionsTitle:   "Versions — %s",
		VersionsHint:    "Paste an entry UUID to see its preserved versions (up to 3).",
		EntryUUID:       "Entry UUID",
		EntryUUIDHint:   "UUID of the entry you want versions for",
		VersionsShow:    "Show versions",
		VersionsCount:   "%d version(s)",
		VersionsNone:    "No versions — entry not found.",
		VersionsRestore: "Restore",
		ConfirmRestore:  "Restore version %s as the new current version in %q?\n\nThe server creates a new version from the selected one. Run 'Sync' (or Pull) afterwards to bring the change into your local file.",
		RestoreDoneSync: "The version was restored on the server. Run 'Sync' (or Pull) on the database to bring the change down.",
	},
}
