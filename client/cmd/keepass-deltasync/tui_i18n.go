// SPDX-License-Identifier: GPL-3.0-or-later

package main

// Sprog til TUI'en. Engelsk er default; dansk vælges via menuen ("Language /
// Sprog") og persisteres i config (cfg.Language). Strengene lever i en
// compile-time-tjekket struct frem for en map, så en manglende oversættelse
// er en byggefejl, ikke en tom streng på skærmen.

const (
	langEN = "en"
	langDA = "da"
)

// msgs holder alle bruger-synlige strenge i TUI'en for ét sprog. Felter med
// "Fmt"-suffix er fmt-format-strenge og BEVARER deres %s-pladsholdere på tværs
// af sprog.
type msgs struct {
	// Generelt / knapper
	btnOK       string
	btnCancel   string
	btnBack     string
	btnContinue string
	btnExit     string
	pressEnter  string
	errFmt      string
	cfgReadErr  string

	// Sprog-navne (egennavne — ens på begge sprog)
	langNameEN string
	langNameDA string

	// Statushoved
	statusTitle  string
	hdrFmt       string // "Server ...: %s\n... : %s\n... : %s"
	hNotSet      string
	hNotEnrolled string
	hEnrolled    string
	hNone        string

	// Database-vælger
	selectDbSuffix string // "%s — <suffix>"

	// Hovedmenu
	miEnroll, miEnrollDesc         string
	fldEnrollToken                 string
	miInit, miInitDesc             string
	fldInitName, fldInitPath       string
	miStatus, miStatusDescNoDb     string
	miDatabases, miDatabasesDescSrv string
	miSyncNow, miSyncNowDesc       string
	miPull, miPullDesc             string
	miPush, miPushDesc             string
	miStatusDesc                   string
	miDatabasesDesc                string
	miDevices, miDevicesDesc       string
	miLog, miLogDesc               string
	miDaemon, miDaemonDesc         string
	miSwitch, miSwitchDesc         string
	miAdvanced, miAdvancedDesc     string
	miQuit, miQuitDesc             string
	miLanguage, miLanguageDescFmt  string

	// Avanceret-menu
	advTitle                          string
	miVersions, miVersionsDesc        string
	fldEntryUUID                      string
	miRestore, miRestoreDesc          string
	fldVersion                        string
	miMembers, miMembersDesc          string
	miShare, miShareDesc              string
	fldUsername                       string
	miUnshare, miUnshareDesc          string
	miEnrollTok, miEnrollTokDesc      string
	miForget, miForgetDesc            string

	// Pick-titler (bruges i "<title> — vælg database")
	pkSync, pkPull, pkPush                      string
	pkVersions, pkRestore                       string
	pkMembers, pkShare, pkUnshare, pkForget     string

	// Enrollment-token-flow
	fldEtUsername string
	fldAdminToken string

	// Skift konto-wizard
	saNoServerTitle string
	saNoServerBody  string
	saIntroTitle    string
	saIntroFmt      string // %s = server-URL
	saStep1Title    string
	saStep2Title    string
	saStep2Body     string
	saStep3Title    string
	fldSaName       string
	fldSaPath       string
	fldSaUUID       string
	saDoneTitle     string
	saDoneFmt       string // %s = binding-navn

	// Sektioner på forsiden (GUI-fanerne som menupunkter)
	secDatabases, secDatabasesDesc string
	secDevices, secDevicesDesc     string
	secLog, secLogDesc             string
	secAdmin, secAdminDesc         string
	secSettings, secSettingsDesc   string

	// Undermenu-titler
	dbMenuTitle       string
	devMenuTitle      string
	logMenuTitle      string
	adminMenuTitle    string
	settingsMenuTitle string

	// Database-undermenu (slet på server)
	miDeleteDB, miDeleteDBDesc string
	pkDeleteDB                 string

	// Log-undermenu (perioder)
	miLogLatest, miLogLatestDesc string
	miLog24h, miLog24hDesc       string
	miLog7d, miLog7dDesc         string
	miLog30d, miLog30dDesc       string

	// Admin-undermenu (brugeradministration)
	miAdmUsers, miAdmUsersDesc       string
	miAdmCreate, miAdmCreateDesc     string
	miAdmEnable, miAdmEnableDesc     string
	miAdmDisable, miAdmDisableDesc   string
	miAdmDelete, miAdmDeleteDesc     string
	miAdmTokenSQL, miAdmTokenSQLDesc string
	admTokenTitle                    string
	fldAdmDisplay                    string

	// Avanceret tilmelding (admin udsteder token + enroller PC'en)
	miAdvEnroll, miAdvEnrollDesc string
	advEnrollTitle              string
	fldAdvServer                string
	fldAdvMode                  string
	advModeExisting, advModeNew string
	fldAdvDevice                string
	advEnrollMissing            string
	advEnrollNoToken            string
	advEnrollFailFmt            string // %s = fejltekst
	advEnrollOkTitle            string
}

// messagesFor returnerer strengsættet for et sprogvalg samt den normaliserede
// sprogkode (engelsk for alt ukendt/tomt).
func messagesFor(lang string) (*msgs, string) {
	switch lang {
	case langDA:
		return &daMsgs, langDA
	default:
		return &enMsgs, langEN
	}
}

// langDisplayName returnerer det aktive sprogs eget navn ("English"/"Dansk").
func (m *msgs) langDisplayName(lang string) string {
	if lang == langDA {
		return m.langNameDA
	}
	return m.langNameEN
}

var enMsgs = msgs{
	btnOK:       "OK",
	btnCancel:   "Cancel",
	btnBack:     "‹ Back",
	btnContinue: "Continue",
	btnExit:     "Exit",
	pressEnter:  "\nPress Enter to return to the menu … ",
	errFmt:      "\n[error] %v\n",
	cfgReadErr:  "Could not read config:\n\n",

	langNameEN: "English",
	langNameDA: "Dansk",

	statusTitle:  " Status ",
	hdrFmt:       "Server    : %s\nDevice    : %s\nDatabases : %s",
	hNotSet:      "(not set)",
	hNotEnrolled: "[red]not enrolled[-]",
	hEnrolled:    "[green]enrolled[-]",
	hNone:        "(none)",

	selectDbSuffix: "select database",

	miEnroll: "Enroll device", miEnrollDesc: "Register this device with an enrollment token",
	fldEnrollToken: "Enrollment token",
	miInit:         "Init database", miInitDesc: "Bind a local .kdbx to a server database",
	fldInitName: "Name (short id, e.g. 'private')", fldInitPath: "Path to local .kdbx",
	miStatus: "Status", miStatusDescNoDb: "Show enrollment info",
	miDatabases: "Databases", miDatabasesDescSrv: "List server databases",
	miSyncNow: "Sync now", miSyncNowDesc: "Pull + push for a database",
	miPull: "Pull", miPullDesc: "Fetch server changes into the local .kdbx",
	miPush: "Push", miPushDesc: "Send local changes to the server",
	miStatusDesc:    "Show enrollment + last-seen",
	miDatabasesDesc: "List local + server databases",
	miDevices:       "Devices", miDevicesDesc: "List enrolled devices",
	miLog: "Log", miLogDesc: "Show the latest audit log",
	miDaemon: "Daemon", miDaemonDesc: "Run continuous sync (Ctrl-C stops)",
	miSwitch: "Switch account (guided)", miSwitchDesc: "Change which account this device syncs from",
	miAdvanced: "Advanced …", miAdvancedDesc: "Versions, restore, sharing",
	miQuit: "Quit", miQuitDesc: "Close the menu",
	miLanguage: "Language / Sprog", miLanguageDescFmt: "Currently: %s — select to switch",

	advTitle:   " Advanced ",
	miVersions: "Versions", miVersionsDesc: "List server versions of an entry",
	fldEntryUUID: "Entry UUID",
	miRestore:    "Restore version", miRestoreDesc: "Roll an entry back to version 1-3",
	fldVersion: "Version (1-3)",
	miMembers:  "Members", miMembersDesc: "List members of a database (owner only)",
	miShare: "Share database", miShareDesc: "Give another user access",
	fldUsername: "Username",
	miUnshare:   "Remove member", miUnshareDesc: "Remove a user (or yourself)",
	miEnrollTok: "Enrollment token for a new device", miEnrollTokDesc: "Generate a token for e.g. the Android app (needs admin token)",
	miForget: "Forget database", miForgetDesc: "Remove a local binding from config (leaves server/file untouched)",

	pkSync: "Sync", pkPull: "Pull", pkPush: "Push",
	pkVersions: "Versions", pkRestore: "Restore",
	pkMembers: "Members", pkShare: "Share", pkUnshare: "Remove", pkForget: "Forget database",

	fldEtUsername: "Username (e.g. 'hans')",
	fldAdminToken: "Admin token",

	saNoServerTitle: "Switch account",
	saNoServerBody:  "No server in config yet. Use 'Enroll device' first.",
	saIntroTitle:    "Switch account — how it works",
	saIntroFmt: "This changes which account this device syncs from — on the SAME server:\n\n" +
		"  %s\n\n" +
		"Steps:\n" +
		"  1) Enroll against the other account (paste an enrollment token)\n" +
		"  2) See the account's databases + UUID\n" +
		"  3) Bind a local .kdbx to that database\n" +
		"  4) Sync everything down\n\n" +
		"Important:\n" +
		"  • Your current server block will be OVERWRITTEN (new device identity).\n" +
		"  • The local .kdbx must use the SAME master password as the database.\n" +
		"    If you have no local copy, create an empty .kdbx in KeePassXC with the\n" +
		"    right password — sync fills in the rest.\n" +
		"  • The old binding becomes stale; clean it up afterwards via\n" +
		"    Advanced → Forget database.",
	saStep1Title: "Switch account · 1/3 — enroll against the other account",
	saStep2Title: "Switch account · 2/3 — find the database",
	saStep2Body:  "The new account's databases are shown now.\n\nNote the UUID of the database you want to bind to — you'll paste it in the next step.",
	saStep3Title: "Switch account · 3/3 — bind local .kdbx",
	fldSaName:    "Local name (e.g. 'passwords')",
	fldSaPath:    "Path to local .kdbx",
	fldSaUUID:    "Remote database UUID",
	saDoneTitle:  "Switch account — almost done",
	saDoneFmt: "The binding '%s' is set up.\n\n" +
		"Choose Continue to sync now (you'll be asked for the master password),\n" +
		"or Cancel to go back and sync later.\n\n" +
		"Remember: the old binding can be removed via Advanced → Forget database.",

	secDatabases: "Databases", secDatabasesDesc: "Sync, share, versions, add/remove",
	secDevices: "Devices", secDevicesDesc: "List devices, issue enrollment tokens",
	secLog: "Log", secLogDesc: "Server audit log (by period)",
	secAdmin: "Admin", secAdminDesc: "User administration (needs admin token)",
	secSettings: "Settings", secSettingsDesc: "Status, switch account, daemon, language",

	dbMenuTitle:       " Databases ",
	devMenuTitle:      " Devices ",
	logMenuTitle:      " Log ",
	adminMenuTitle:    " Admin ",
	settingsMenuTitle: " Settings ",

	miDeleteDB: "Delete on server", miDeleteDBDesc: "PERMANENTLY delete a database for everyone (asks to confirm)",
	pkDeleteDB: "Delete on server",

	miLogLatest: "Latest", miLogLatestDesc: "Most recent audit entries",
	miLog24h: "Last 24 hours", miLog24hDesc: "Entries from the last 24h",
	miLog7d: "Last 7 days", miLog7dDesc: "Entries from the last 7 days",
	miLog30d: "Last 30 days", miLog30dDesc: "Entries from the last 30 days",

	miAdmUsers: "List users", miAdmUsersDesc: "Show all users (devices/databases count)",
	miAdmCreate: "Create user", miAdmCreateDesc: "Create a user + return an enrollment token",
	miAdmEnable: "Enable user", miAdmEnableDesc: "Re-enable a disabled user",
	miAdmDisable: "Disable user", miAdmDisableDesc: "Block a user's auth (data kept)",
	miAdmDelete: "Delete user", miAdmDeleteDesc: "Permanently delete a user (CASCADE; asks to confirm)",
	miAdmTokenSQL: "Admin token SQL", miAdmTokenSQLDesc: "Print SQL to mint a fresh admin token (DBeaver)",
	admTokenTitle: " Admin token ",
	fldAdmDisplay: "Display name (optional)",

	miAdvEnroll: "Advanced enrollment (admin)", miAdvEnrollDesc: "Issue a token and enroll this device in one step",
	advEnrollTitle: "Advanced enrollment — administrator",
	fldAdvServer:   "Server URL (e.g. https://deltasync.example.dk)",
	fldAdvMode:     "User",
	advModeExisting: "Existing user", advModeNew: "Create new user",
	fldAdvDevice:     "Device name (optional)",
	advEnrollMissing: "Server URL, admin token and username are required.",
	advEnrollNoToken: "Could not read the enrollment token from the server's response:",
	advEnrollFailFmt: "Issuing the enrollment token failed:\n\n%s",
	advEnrollOkTitle: "Enrolled",
}

var daMsgs = msgs{
	btnOK:       "OK",
	btnCancel:   "Annullér",
	btnBack:     "‹ Tilbage",
	btnContinue: "Fortsæt",
	btnExit:     "Afslut",
	pressEnter:  "\nTryk Enter for at vende tilbage til menuen … ",
	errFmt:      "\n[fejl] %v\n",
	cfgReadErr:  "Kunne ikke læse config:\n\n",

	langNameEN: "English",
	langNameDA: "Dansk",

	statusTitle:  " Status ",
	hdrFmt:       "Server    : %s\nEnhed     : %s\nDatabaser : %s",
	hNotSet:      "(ikke sat)",
	hNotEnrolled: "[red]ikke enrolled[-]",
	hEnrolled:    "[green]enrolled[-]",
	hNone:        "(ingen)",

	selectDbSuffix: "vælg database",

	miEnroll: "Enroll enhed", miEnrollDesc: "Registrér denne enhed med en enrollment-token",
	fldEnrollToken: "Enrollment-token",
	miInit:         "Init database", miInitDesc: "Knyt en lokal .kdbx til en server-database",
	fldInitName: "Navn (kort id, fx 'privat')", fldInitPath: "Sti til lokal .kdbx",
	miStatus: "Status", miStatusDescNoDb: "Vis enrollment-info",
	miDatabases: "Databaser", miDatabasesDescSrv: "List server-databaser",
	miSyncNow: "Synk nu", miSyncNowDesc: "Pull + push for en database",
	miPull: "Pull (hent)", miPullDesc: "Hent server-ændringer ind i lokal .kdbx",
	miPush: "Push (send)", miPushDesc: "Send lokale ændringer til serveren",
	miStatusDesc:    "Vis enrollment + last-seen",
	miDatabasesDesc: "List lokale + server-databaser",
	miDevices:       "Enheder", miDevicesDesc: "List enrollede enheder",
	miLog: "Log", miLogDesc: "Vis seneste audit-log",
	miDaemon: "Daemon", miDaemonDesc: "Kør kontinuerlig sync (Ctrl-C stopper)",
	miSwitch: "Skift konto (guided)", miSwitchDesc: "Skift hvilken konto denne enhed synker fra",
	miAdvanced: "Avanceret …", miAdvancedDesc: "Versioner, gendan, deling",
	miQuit: "Afslut", miQuitDesc: "Luk menuen",
	miLanguage: "Language / Sprog", miLanguageDescFmt: "Aktuelt: %s — vælg for at skifte",

	advTitle:   " Avanceret ",
	miVersions: "Versioner", miVersionsDesc: "List server-versioner af en entry",
	fldEntryUUID: "Entry-UUID",
	miRestore:    "Gendan version", miRestoreDesc: "Rul en entry tilbage til version 1-3",
	fldVersion: "Version (1-3)",
	miMembers:  "Medlemmer", miMembersDesc: "List medlemmer af en database (kun owner)",
	miShare: "Del database", miShareDesc: "Giv en anden bruger adgang",
	fldUsername: "Brugernavn",
	miUnshare:   "Fjern medlem", miUnshareDesc: "Fjern en bruger (eller dig selv)",
	miEnrollTok: "Enrollment-token til ny enhed", miEnrollTokDesc: "Generér token til fx Android-appen (kræver admin-token)",
	miForget: "Glem database", miForgetDesc: "Fjern en lokal binding fra config (rører ikke server/fil)",

	pkSync: "Synk", pkPull: "Pull", pkPush: "Push",
	pkVersions: "Versioner", pkRestore: "Gendan",
	pkMembers: "Medlemmer", pkShare: "Del", pkUnshare: "Fjern", pkForget: "Glem database",

	fldEtUsername: "Brugernavn (fx 'hans')",
	fldAdminToken: "Admin-token",

	saNoServerTitle: "Skift konto",
	saNoServerBody:  "Ingen server i config endnu. Brug 'Enroll enhed' først.",
	saIntroTitle:    "Skift konto — sådan virker det",
	saIntroFmt: "Dette skifter hvilken konto denne enhed synker fra — på SAMME server:\n\n" +
		"  %s\n\n" +
		"Trin:\n" +
		"  1) Enroll mod den anden konto (indsæt et enrollment-token)\n" +
		"  2) Se kontoens databaser + UUID\n" +
		"  3) Bind en lokal .kdbx til den database\n" +
		"  4) Synk alt ned\n\n" +
		"Vigtigt:\n" +
		"  • Din nuværende server-blok bliver OVERSKREVET (ny enhed-identitet).\n" +
		"  • Den lokale .kdbx skal bruge SAMME master-password som databasen.\n" +
		"    Har du ingen lokal kopi, så lav en tom .kdbx i KeePassXC med det\n" +
		"    rigtige password — sync fylder resten i.\n" +
		"  • Den gamle binding bliver forældet; ryd op bagefter med\n" +
		"    Avanceret → Glem database.",
	saStep1Title: "Skift konto · 1/3 — enroll mod den anden konto",
	saStep2Title: "Skift konto · 2/3 — find databasen",
	saStep2Body:  "Nu vises den nye kontos databaser.\n\nNotér UUID'et på den database du vil binde til — du skal indsætte det i næste trin.",
	saStep3Title: "Skift konto · 3/3 — bind lokal .kdbx",
	fldSaName:    "Lokalt navn (fx 'adgangskoder')",
	fldSaPath:    "Sti til lokal .kdbx",
	fldSaUUID:    "Remote database-UUID",
	saDoneTitle:  "Skift konto — næsten færdig",
	saDoneFmt: "Bindingen '%s' er sat op.\n\n" +
		"Vælg Fortsæt for at synke nu (du bliver bedt om master-passwordet),\n" +
		"eller Annullér for at vende tilbage og synke senere.\n\n" +
		"Husk: den gamle binding kan fjernes med Avanceret → Glem database.",

	secDatabases: "Databaser", secDatabasesDesc: "Synk, deling, versioner, tilføj/fjern",
	secDevices: "Enheder", secDevicesDesc: "Vis enheder, udsted enrollment-tokens",
	secLog: "Log", secLogDesc: "Server-audit-log (efter periode)",
	secAdmin: "Admin", secAdminDesc: "Brugeradministration (kræver admin-token)",
	secSettings: "Indstillinger", secSettingsDesc: "Status, skift konto, daemon, sprog",

	dbMenuTitle:       " Databaser ",
	devMenuTitle:      " Enheder ",
	logMenuTitle:      " Log ",
	adminMenuTitle:    " Admin ",
	settingsMenuTitle: " Indstillinger ",

	miDeleteDB: "Slet på server", miDeleteDBDesc: "Slet en database PERMANENT for alle (beder om bekræftelse)",
	pkDeleteDB: "Slet på server",

	miLogLatest: "Seneste", miLogLatestDesc: "De nyeste audit-poster",
	miLog24h: "Sidste 24 timer", miLog24hDesc: "Poster fra de seneste 24t",
	miLog7d: "Sidste 7 dage", miLog7dDesc: "Poster fra de seneste 7 dage",
	miLog30d: "Sidste 30 dage", miLog30dDesc: "Poster fra de seneste 30 dage",

	miAdmUsers: "Vis brugere", miAdmUsersDesc: "List alle brugere (antal enheder/databaser)",
	miAdmCreate: "Opret bruger", miAdmCreateDesc: "Opret en bruger + få et enrollment-token",
	miAdmEnable: "Aktivér bruger", miAdmEnableDesc: "Genaktivér en deaktiveret bruger",
	miAdmDisable: "Deaktivér bruger", miAdmDisableDesc: "Bloker en brugers login (data bevares)",
	miAdmDelete: "Slet bruger", miAdmDeleteDesc: "Slet en bruger permanent (CASCADE; beder om bekræftelse)",
	miAdmTokenSQL: "Admin-token SQL", miAdmTokenSQLDesc: "Print SQL til at lave en frisk admin-token (DBeaver)",
	admTokenTitle: " Admin-token ",
	fldAdmDisplay: "Visningsnavn (valgfrit)",

	miAdvEnroll: "Avanceret tilmelding (admin)", miAdvEnrollDesc: "Udsted et token og tilmeld denne enhed i ét hug",
	advEnrollTitle: "Avanceret tilmelding — administrator",
	fldAdvServer:   "Server-URL (fx https://deltasync.example.dk)",
	fldAdvMode:     "Bruger",
	advModeExisting: "Eksisterende bruger", advModeNew: "Opret ny bruger",
	fldAdvDevice:     "Enhedsnavn (valgfrit)",
	advEnrollMissing: "Server-URL, admin-token og brugernavn er påkrævet.",
	advEnrollNoToken: "Kunne ikke udlæse enrollment-tokenet fra serverens svar:",
	advEnrollFailFmt: "Udstedelse af enrollment-token fejlede:\n\n%s",
	advEnrollOkTitle: "Tilmeldt",
}
