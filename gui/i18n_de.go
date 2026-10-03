// SPDX-License-Identifier: GPL-3.0-or-later

package main

// Deutsch. Übersetzt aus dem Englischen; die Feldreihenfolge entspricht
// langEN in i18n.go, damit sich beide leicht vergleichen lassen.

func init() {
	dicts[langDE] = &dict{
		AppTitle: "KeePass Delta-Sync",

		OK: "OK", Cancel: "Abbrechen", Close: "Schließen", Save: "Speichern",
		Back: "Zurück", Next: "Weiter", Finish: "Fertigstellen", Browse: "Durchsuchen…",
		Working: "Arbeitet…", Done: "Fertig", Error: "Fehler",
		Copy: "Kopieren", DBInfoTitle: "Datenbankinfo",

		CLINotFound:     "Das Programm „keepass-deltasync“ wurde nicht gefunden.",
		CLILocate:       "keepass-deltasync suchen",
		CLILocateHint:   "Geben Sie den Pfad zum Programm keepass-deltasync (der CLI) an. Es liegt normalerweise neben dieser App.",
		CLIPathLabel:    "Pfad zur CLI",
		CLISelectBinary: "Das Programm keepass-deltasync auswählen",

		WizardWelcomeTitle: "Willkommen",
		WizardWelcomeBody: "Dieser Assistent hilft Ihnen, Ihre KeePass-Datenbank " +
			"zwischen Ihren Geräten zu synchronisieren.\n\nSie benötigen:\n  • Die Serveradresse von Ihrem Administrator\n  • Ein Enrollment-Token (Einmalcode)\n  • Ihre .kdbx-Datei",
		WizardStart:      "Assistent starten",
		WizardStepEnroll: "Schritt 1 von 2 — Gerät registrieren",
		WizardStepDB:     "Schritt 2 von 2 — Datenbank hinzufügen",
		ServerURL:        "Serveradresse",
		ServerURLHint:    "z. B. https://deltasync.example.de",
		EnrollToken:      "Enrollment-Token",
		EnrollTokenHint:  "Der Einmalcode von Ihrem Administrator",
		DeviceName:       "Gerätename (optional)",
		DeviceNameHint:   "Wird in Admin-Listen angezeigt. Leer = Computername.",
		EnrollButton:     "Dieses Gerät registrieren",
		EnrollOK:         "Gerät registriert!",
		WizardAddDB:      "Fügen Sie Ihre erste Datenbank hinzu",
		WizardAddDBBody:  "Registrieren Sie eine lokale .kdbx-Datei für die Synchronisierung. Weitere können Sie jederzeit hinzufügen.",
		DBName:           "Name",
		DBNameHint:       "Ein kurzer Name, z. B. „privat“ oder „arbeit“",
		KdbxFile:         "KeePass-Datei (.kdbx)",
		KdbxFileHint:     "Pfad zu Ihrer lokalen Datenbank",
		CreateDBButton:   "Datenbank anlegen",
		SkipForNow:       "Vorerst überspringen",
		WizardDone:       "Alles bereit!",
		WizardDoneBody:   "Sie können jetzt synchronisieren. Mit der Schaltfläche „Sync“ an einer Datenbank senden und holen Sie Änderungen.",

		WizardAdvanced:     "Erweitert (Administrator)",
		WizardFirefox:      "Aus Firefox suchen — ohne Konto",
		FFIntro:            "Durchsuchen Sie Ihre KeePass-Einträge aus Firefox und springen Sie direkt zur passenden Website. Das braucht weder Konto noch Server — nur zweierlei, einmalig: das Programm auf Ihre .kdbx zeigen lassen und es bei Firefox registrieren. Das Ausfüllen der Anmeldedaten bleibt bei KeePassXC-Browser; die Erweiterung sieht nie ein Passwort.",
		FFAddLocal:         "Datenbank registrieren…",
		FFAddLocalTitle:    "Eine Datenbank für die Suche registrieren",
		FFAddLocalDone:     "Die Datenbank ist registriert. Als Nächstes: Bei Firefox registrieren — danach Firefox neu starten.",
		FFSavePassword:     "Das Master-Passwort im Schlüsselbund des Systems speichern (damit das Popup nicht danach fragt)",
		FFPasswordMissing:  "Geben Sie das Master-Passwort ein oder entfernen Sie den Haken — dann fragt die Erweiterung danach.",
		FFInstallHost:      "Bei Firefox registrieren",
		FFHostInstalled:    "Bei Firefox registriert",
		FFRestartFirefox:   "Starten Sie Firefox neu — erst dadurch wird die Registrierung übernommen.",
		FFPreview:          "Anzeigen, was geschrieben würde",
		FFPreviewTitle:     "Probelauf (es wird nichts geschrieben)",
		FFUninstallHost:    "Registrierung entfernen",
		FFUninstallConfirm: "Die Registrierung aus jeder Firefox-Variante auf diesem Rechner entfernen? Weder die Datenbank noch die Datei werden angerührt.",
		FFGuide:            "Anleitung",
		FFProbe:            "Testen",
		FFProbeTitle:       "Was der Browser bekäme — %s",
		FFProbePwdHint:     "Optional — wird aus dem Schlüsselbund genommen, falls dort gespeichert",
		FFProbeEmpty:       "Der Index ist leer: keine Einträge mit verwendbarer URL.",
		FFCount:            "%d Datenbank(en) aus Firefox durchsuchbar",
		FFNone:             "Noch keine Datenbanken registriert — beginnen Sie mit „Datenbank registrieren“.",
		FFKindSynced:       "(synchronisiert)",
		FFKindLocal:        "(nur lokal)",
		FFCLIPath:          "Programm:",
		HelpFirefox: "## Firefox\n\n" +
			"Volltextsuche über Ihre Einträge aus der Adressleiste von Firefox (`kp` + Leertaste) oder aus dem Popup (**Alt+Umschalt+K**), und ein Tastendruck öffnet die Website des Eintrags. Die Erweiterung erhält **nur** Titel, URLs und Gruppenpfad — niemals Passwörter, Benutzernamen, Notizen oder Anhänge.\n\n" +
			"Die Einrichtung besteht aus zwei Schritten, einmal pro Rechner:\n\n" +
			"- **Datenbank registrieren** — zeigt dem Programm eine `.kdbx` (`add-local`). Kein Server, kein Konto, nichts wird hochgeladen. Setzen Sie den Haken, und das Master-Passwort wandert in den Schlüsselbund des Systems, sodass das Popup nicht danach fragt.\n" +
			"- **Bei Firefox registrieren** — schreibt das Native-Messaging-Manifest, damit Firefox den Host starten darf (`install-browser-host`). **Danach Firefox neu starten.**\n\n" +
			"Außerdem: **Testen** zeigt genau den Index, den der Browser erhalten würde (`browser-host --probe`), und **Anzeigen, was geschrieben würde** ist dieselbe Registrierung, ohne etwas anzufassen (`--dry-run`).\n\n" +
			"Das Add-on selbst wird von addons.mozilla.org installiert — siehe **Anleitung**.\n\n" +
			"**CLI-Befehle:**\n\n" +
			"- `keepass-deltasync add-local <name> <file.kdbx>`\n" +
			"- `keepass-deltasync install-browser-host`\n" +
			"- `keepass-deltasync browser-host --probe <name>`\n" +
			"- `keepass-deltasync uninstall-browser-host`",
		WizardStepAdvanced: "Erweiterte Registrierung — Administrator",
		AdvancedIntro: "Wenn Sie ein Admin-Token haben, können Sie ein Enrollment-Token ausstellen und diesen PC in einem Schritt registrieren — " +
			"ganz ohne vorab erhaltenes Token. Wählen Sie einen vorhandenen Benutzer oder legen Sie einen neuen an.",
		AdvUserMode:     "Benutzer",
		AdvExistingUser: "Vorhandener Benutzer",
		AdvNewUser:      "Neuen Benutzer anlegen",
		AdvEnrollButton: "Token ausstellen und diesen PC registrieren",
		AdvIssuingToken: "Enrollment-Token wird ausgestellt…",
		AdvEnrolling:    "Dieser PC wird registriert…",
		AdvNoTokenErr:   "Das Enrollment-Token konnte nicht aus der Antwort des Servers gelesen werden. Einzelheiten stehen im Protokoll.",

		TabDatabases: "Datenbanken",
		TabDevices:   "Geräte",
		TabFirefox:   "Firefox",
		TabActivity:  "Aktivität",
		TabLog:       "Protokoll",
		TabAdmin:     "Verwaltung",
		TabSettings:  "Einstellungen",
		ActivityHint: "Hier erscheint die Ausgabe der CLI-Aufrufe (Sync, Hinzufügen/Entfernen, Fehler …).",
		Clear:        "Leeren",

		LogHint:         "Aktivitätsprotokoll des Servers (Audit) — der Verlauf über alle Geräte, bis zu 30 Tage aufbewahrt.",
		LogCount:        "%d Protokolleinträge",
		LogEmpty:        "(keine Protokolleinträge in diesem Zeitraum)",
		LogPeriodLabel:  "Zeitraum:",
		LogPeriod24h:    "Letzte 24 Stunden",
		LogPeriod7d:     "Letzte 7 Tage",
		LogPeriod30d:    "Letzte 30 Tage",
		LogPeriodAll:    "Alle",
		LogDetailsTitle: "Protokolldetails",
		LogColTime:      "Zeit",
		LogColEvent:     "Ereignis",
		LogColLevel:     "Stufe",
		LogColIP:        "IP-Adresse",
		LogOK:           "OK",
		LogFail:         "Fehlgeschlagen",

		ColEnrolled:     "Registriert",
		ColLastSeen:     "Zuletzt gesehen",
		ThisDevice:      "● dieses Gerät",
		DevCount:        "%d Gerät(e) im Konto",
		DeviceInfoTitle: "Geräteinfo",

		AddDevice:           "Gerät hinzufügen",
		AddDeviceCreate:     "Token erzeugen",
		RemoveDevice:        "Gerät entfernen",
		Username:            "Benutzername",
		AdminToken:          "Admin-Token",
		EnrollTokenCreated:  "Enrollment-Token für neues Gerät",
		SelectDeviceFirst:   "Wählen Sie zuerst ein Gerät in der Liste aus.",
		CannotRemoveCurrent: "Das Gerät, das Sie gerade verwenden, können Sie hier nicht entfernen.",
		ConfirmRemoveDevice: "Gerät %q entfernen (widerrufen)? Sein Token wird ungültig.",
		StatusBox:           "Status",
		NotEnrolled:         "Noch nicht registriert.",
		Refresh:             "Aktualisieren",
		Sync:                "Sync",
		SyncSelected:        "Auswahl synchronisieren",
		SyncAll:             "Alle synchronisieren",
		SelectFirst:         "Wählen Sie zuerst eine Datenbank in der Liste aus.",
		AddDatabase:         "Datenbank hinzufügen",
		ForgetDatabase:      "Datenbank vergessen",
		ConfirmForget:       "Die lokale Verknüpfung für %q vergessen? Die .kdbx-Datei und die Datenbank auf dem Server werden NICHT angerührt — nur die Verknüpfung in diesem Client wird entfernt.",
		MoreActions:         "Weitere Aktionen",
		PushNow:             "Jetzt pushen (nur hochladen)",
		PullNow:             "Jetzt pullen (nur herunterladen)",
		DeleteOnServer:      "Auf dem Server löschen",
		ConfirmDeleteServer: "Die Datenbank %q ENDGÜLTIG auf dem Server löschen?\n\nDas entfernt ALLE Einträge, Versionen, Freigaben und den Verlauf — für jeden Benutzer. Diese Aktion kann NICHT rückgängig gemacht werden. Ihre lokale .kdbx-Datei bleibt unberührt.",
		NoDatabases:         "Noch keine Datenbanken. Klicken Sie auf „Datenbank hinzufügen“, um zu beginnen.",
		DBCount:             "%d Datenbank(en) verbunden",
		ColName:             "Name",
		ColStatus:           "Status",
		ColID:               "ID",
		ColCreated:          "Erstellt",
		ColPath:             "Lokaler Pfad",
		BoundLocally:        "● bereit",
		OnServerOnly:        "○ nur Server",

		MembersTitle:       "Mit der Datenbank verbunden",
		MembersOf:          "Mit %q verbunden",
		SelectToSeeMembers: "Wählen Sie oben eine Datenbank aus, um zu sehen, wer mit ihr verbunden ist.",
		MemberCount:        "%d Mitglied(er)",
		MembersNeedBound:   "Die Datenbank muss lokal eingerichtet sein, um Mitglieder anzuzeigen.",
		MembersUnavailable: "Mitglieder können nicht abgerufen werden (nur die Eigentümerin oder der Eigentümer sieht das):",
		NoMembers:          "Nur Sie — noch mit niemandem geteilt.",
		ColRole:            "Rolle",
		ColUser:            "Benutzername",
		ColDisplay:         "Anzeigename",
		ColAdded:           "Hinzugefügt",

		ShareDatabase:     "Datenbank teilen",
		ShareTitle:        "%q mit einem Benutzer teilen",
		ShareWith:         "Teilen mit (Benutzername)",
		RemoveMember:      "Mitglied entfernen",
		SelectMemberFirst: "Wählen Sie zuerst ein Mitglied in der Liste aus.",
		CannotRemoveOwner: "Die Eigentümerin oder der Eigentümer kann nicht entfernt werden.",
		ConfirmUnshare:    "%s aus %q entfernen?",
		SetupShared:       "Lokal einrichten",
		SetupSharedTitle:  "Die geteilte Datenbank %q lokal einrichten",
		AlreadyLocal:      "Die Datenbank ist bereits lokal eingerichtet.",
		NewLocalPassword:  "Neues lokales Passwort",
		BindExisting:      "Vorhandene .kdbx verbinden (Ihre eigene Datenbank, neues Gerät)",
		BoundNowSync:      "Verbunden! Klicken Sie auf ⟳ (Sync), um Einträge zu holen.",
		MasterPwd:         "Master-Passwort",
		MasterPwdFor:      "Master-Passwort für",
		Language:          "Sprache",
		ThemeLabel:        "Design",
		ThemeSystem:       "System (dem Betriebssystem folgen)",
		ThemeLight:        "Hell",
		ThemeDark:         "Dunkel",
		HelpPanelLabel:    "Hilfebereich anzeigen",
		HelpPanelDesc:     "Ist das aktiviert, beschreibt ein Bereich am unteren Fensterrand den gerade geöffneten Tab — was die Seite tut und welchen Befehlen im Programm keepass-deltasync die Schaltflächen entsprechen.",

		UpdateAvailable:  "Version %s ist verfügbar — Sie verwenden %s.",
		UpdateDownload:   "Herunterladen",
		UpdateCheckLabel: "Nach Updates suchen",
		UpdateCheckDesc:  "Ist das aktiviert, fragt das Programm beim Start bei GitLab nach, ob eine neuere Version vorliegt, und zeigt gegebenenfalls oben eine Zeile an. Es wird nichts über Sie oder Ihre Datenbanken gesendet — nur eine gewöhnliche Abfrage der Releases-Seite des Projekts.",

		HelpTitle: "Über diese Seite",
		HelpDatabases: "## Datenbanken\n\n" +
			"Ihre Datenbanken und deren Freigaben. **● (gefüllter Kreis)** = mit einer lokalen `.kdbx`-Datei verknüpft und synchronisierbereit. **○ (offener Kreis)** = existiert nur auf dem Server.\n\n" +
			"**Aktionen pro Datenbank:**\n\n" +
			"- **Sync** — Änderungen senden und holen (`sync`).\n" +
			"- **Teilen** — einem anderen Benutzer Zugriff geben (`share`).\n" +
			"- **Vergessen** — nur die lokale Verknüpfung entfernen; Server und Datei bleiben unberührt (`forget`).\n" +
			"- **⋮ Mehr** — Push (nur hochladen), Pull (nur herunterladen), Versionen/Wiederherstellen und **Auf dem Server löschen** (`delete-database`, endgültig für alle).\n\n" +
			"Existiert eine Datenbank nur auf dem Server: Ihre eigene `.kdbx` **verbinden** (`init --bind`) oder die geteilte Kopie **einrichten** (`init-shared`).\n\n" +
			"Oben: **Datenbank hinzufügen** (`init`) und **Alle synchronisieren**.\n\n" +
			"**CLI-Befehle:**\n\n" +
			"- `keepass-deltasync databases`\n" +
			"- `keepass-deltasync init <name> <path>`\n" +
			"- `keepass-deltasync sync --password-stdin <db>`\n" +
			"- `keepass-deltasync push --password-stdin <db>` / `pull --password-stdin <db>`\n" +
			"- `keepass-deltasync share --password-stdin <db> <user>` / `unshare <db> <user>` / `shares <db>`\n" +
			"- `keepass-deltasync forget <db>` / `delete-database <db|uuid>`\n" +
			"- `keepass-deltasync init --bind <uuid> <db> <path>` / `init-shared --password-stdin <remote> <path>`\n" +
			"- `keepass-deltasync versions <db> <entry-uuid>` / `restore <db> <entry-uuid> <version>`",
		HelpDevices: "## Geräte\n\n" +
			"Die in Ihrem Konto registrierten Geräte (`devices`). Das **● markierte** ist das Gerät, das Sie gerade verwenden.\n\n" +
			"- **Gerät hinzufügen** — ein Enrollment-Token erzeugen, mit dem sich ein neues Gerät registriert (`admin user-enrollment`).\n" +
			"- **Entfernen** — den Zugriff eines Geräts widerrufen (`devices remove`). Das Gerät, an dem Sie sitzen, können Sie nicht entfernen.\n" +
			"- **Info** — die Geräte-ID sowie Registrierungs- und Zuletzt-gesehen-Zeitpunkt anzeigen.\n\n" +
			"**CLI-Befehle:**\n\n" +
			"- `keepass-deltasync devices`\n" +
			"- `keepass-deltasync devices remove <id>`\n" +
			"- `keepass-deltasync admin user-enrollment <user>`",
		HelpActivity: "## Aktivität\n\n" +
			"Die rohe Ausgabe der CLI-Aufrufe, die die GUI in **dieser Sitzung** macht (Sync, Hinzufügen/Entfernen, Fehler …). Sie wird beim Schließen der App zurückgesetzt.\n\n" +
			"Für den Verlauf über Sitzungen hinweg nutzen Sie den Tab **Protokoll**, der das Audit-Protokoll des Servers abruft.\n\n" +
			"*Kein eigener CLI-Befehl — der Tab zeigt nur die Ausgabe der anderen Befehle.*",
		HelpLog: "## Protokoll\n\n" +
			"Das **Audit-Protokoll** des Servers (`log`) — ein dauerhafter Verlauf der Ereignisse in Ihrem Konto (Anmeldung, Sync, Änderungen …) über all Ihre Geräte hinweg, bis zu 30 Tage aufbewahrt.\n\n" +
			"- **Zeitraum** bestimmt, wie weit zurück abgerufen wird (`--since`).\n" +
			"- Jede Zeile zeigt Zeit, Ereignis, OK/Fehlgeschlagen, Stufe und IP. Klicken Sie auf **ℹ** für alle Einzelheiten.\n\n" +
			"**CLI-Befehl:**\n\n" +
			"- `keepass-deltasync log --since <duration> --limit <n>`",
		HelpAdmin: "## Verwaltung\n\n" +
			"Benutzerverwaltung. Erfordert ein **Admin-Token**, das nur im Arbeitsspeicher gehalten und nie auf der Festplatte gespeichert wird.\n\n" +
			"- **Benutzer laden** — alle Benutzer auflisten (`admin user-list`).\n" +
			"- **Benutzer anlegen** — anlegen und ein Enrollment-Token erhalten (`admin user-create`).\n" +
			"- Pro Benutzer: **neues Enrollment-Token** (`user-enrollment`), **aktivieren/deaktivieren** (`user-enable` / `user-disable`), **löschen** (`user-delete`, CASCADE).\n" +
			"- **Admin-Token holen (SQL)** — SQL, das ein frisches Admin-Token anlegt (`admin token-sql`); führen Sie es in DBeaver aus.\n\n" +
			"**CLI-Befehle:**\n\n" +
			"- `keepass-deltasync admin user-list`\n" +
			"- `keepass-deltasync admin user-create <user> --display-name <name>`\n" +
			"- `keepass-deltasync admin user-enrollment <user>`\n" +
			"- `keepass-deltasync admin user-enable <user>` / `user-disable <user>`\n" +
			"- `keepass-deltasync admin user-delete <user> --yes`\n" +
			"- `keepass-deltasync admin token-sql`",
		HelpSettings: "## Einstellungen\n\n" +
			"- **Sprache** — Dänisch, Englisch, Deutsch, Französisch oder Spanisch.\n" +
			"- **Design** — System (dem Betriebssystem folgen), Hell oder Dunkel.\n" +
			"- **CLI-Pfad** — wo das Programm `keepass-deltasync` liegt. Die GUI ruft es für alle Arbeiten auf.\n" +
			"- **Hilfebereich anzeigen** — dieser Bereich.\n" +
			"- **Autostart** — das Betriebssystem `keepass-deltasync daemon` bei der Anmeldung ausführen lassen, damit die Hintergrund-Synchronisierung auch ohne geöffnete GUI läuft. Wird in Ihrem eigenen Benutzerkontext eingerichtet (Run-Schlüssel in der Registry unter Windows, launchd unter macOS, systemd --user unter Linux) — kein Administrator nötig. Die GUI startet den Daemon nie selbst.\n\n" +
			"Die GUI ist eine Hülle um die Kommandozeile `keepass-deltasync`; die gesamte Kryptografie, alle Serveraufrufe und die Konfiguration liegen in der CLI.\n\n" +
			"**Nützliche CLI-Befehle:**\n\n" +
			"- `keepass-deltasync status`\n" +
			"- `keepass-deltasync enroll --server <url> <token>`",
		ResetEnroll: "Registrierung zurücksetzen",

		AutostartTitle: "Autostart",
		AutostartDesc: "Die Hintergrund-Synchronisierung (`keepass-deltasync daemon`) automatisch bei der Anmeldung ausführen, " +
			"ohne dass diese App geöffnet ist. Wird in Ihrem eigenen Benutzerkontext eingerichtet — kein Administrator nötig.",
		AutostartEnable:      "Autostart aktivieren",
		AutostartDisable:     "Autostart deaktivieren",
		AutostartOn:          "Autostart ist an — der Daemon startet bei der Anmeldung.",
		AutostartOff:         "Autostart ist aus.",
		AutostartUnsupported: "Autostart wird auf diesem Betriebssystem nicht unterstützt.",
		AutostartNoCLI:       "Geben Sie zuerst den Pfad zum Programm keepass-deltasync an (CLI-Pfad oben).",
		AutostartEnabled:     "Autostart aktiviert.",
		AutostartDisabled:    "Autostart deaktiviert.",

		AdminHint:         "Benutzerverwaltung. Erfordert ein Admin-Token (wird nicht auf der Festplatte gespeichert).",
		AdminNeedToken:    "Geben Sie ein Admin-Token ein und klicken Sie auf „Benutzer laden“.",
		AdminLoadUsers:    "Benutzer laden",
		AdminCreateUser:   "Benutzer anlegen",
		AdminTokenHelp:    "Admin-Token holen (SQL)",
		AdminUserCount:    "%d Benutzer",
		AdminNewEnroll:    "Neues Enrollment-Token",
		AdminEnable:       "Benutzer aktivieren",
		AdminDisable:      "Benutzer deaktivieren",
		AdminDeleteUser:   "Benutzer löschen",
		AdminDisplayName:  "Anzeigename (optional)",
		ConfirmDeleteUser: "Den Benutzer %q ENDGÜLTIG löschen?\n\nCASCADE: alle Geräte, Datenbanken und Einträge dieses Benutzers werden entfernt. Diese Aktion kann NICHT rückgängig gemacht werden.",

		VersionsMenu:    "Versionen / wiederherstellen…",
		VersionsTitle:   "Versionen — %s",
		VersionsHint:    "Fügen Sie die UUID eines Eintrags ein, um dessen aufbewahrte Versionen zu sehen (bis zu 3).",
		EntryUUID:       "Eintrags-UUID",
		EntryUUIDHint:   "UUID des Eintrags, dessen Versionen Sie sehen möchten",
		VersionsShow:    "Versionen anzeigen",
		VersionsCount:   "%d Version(en)",
		VersionsNone:    "Keine Versionen — Eintrag nicht gefunden.",
		VersionsRestore: "Wiederherstellen",
		ConfirmRestore:  "Version %s als neue aktuelle Version in %q wiederherstellen?\n\nDer Server erzeugt aus der gewählten Version eine neue. Führen Sie danach „Sync“ (oder Pull) aus, um die Änderung in Ihre lokale Datei zu holen.",
		RestoreDoneSync: "Die Version wurde auf dem Server wiederhergestellt. Führen Sie „Sync“ (oder Pull) an der Datenbank aus, um die Änderung herunterzuholen.",
	}
}
