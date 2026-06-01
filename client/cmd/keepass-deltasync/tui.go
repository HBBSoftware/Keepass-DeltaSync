// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rivo/tview"

	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// runTui åbner en fuldskærms-menu (tview) der hjælper med at køre de
// almindelige kommandoer uden at man skal huske kommandonavne, flag og
// database-navne. Menuen er bevidst en TYND kommando-vælger: den læser kun
// config for at vise state + prefille database-listen, og udfører alt ved at
// kalde denne samme binær som en subproces (os/exec). Den taler aldrig selv
// med serveren og rører hverken crypto eller config-skrivning — så der findes
// kun én kodesti, og password-prompts virker uændret på den rigtige terminal.
func runTui(args []string) error {
	self, err := os.Executable()
	if err != nil || self == "" {
		self = os.Args[0]
	}
	t := &tui{app: tview.NewApplication(), self: self}
	t.app.EnableMouse(true)
	t.showMain()
	return t.app.Run()
}

type tui struct {
	app  *tview.Application
	self string
}

// inputField beskriver ét felt i en collectInputs-formular.
type inputField struct {
	label    string
	password bool
}

// ============================================================
// Skærme
// ============================================================

// showMain genindlæser config (så state afspejler en netop kørt enroll/init/
// sync) og bygger hovedmenuen ud fra de tre states: ikke-enrolled →
// enrolled-uden-db → klar.
func (t *tui) showMain() {
	cfg, err := config.Load()
	if err != nil {
		modal := tview.NewModal().
			SetText("Kunne ikke læse config:\n\n" + err.Error()).
			AddButtons([]string{"Afslut"}).
			SetDoneFunc(func(int, string) { t.app.Stop() })
		t.app.SetRoot(modal, true)
		return
	}

	enrolled := cfg.Server.DeviceToken != ""
	hasDB := len(cfg.Databases) > 0

	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(" keepass-deltasync ")
	list.SetTitleAlign(tview.AlignLeft)

	switch {
	case !enrolled:
		list.AddItem("Enroll enhed", "Registrér denne enhed med en enrollment-token", 'e', func() {
			t.collectInputs("Enroll enhed", []inputField{{label: "Enrollment-token"}}, func(v []string) {
				token := strings.TrimSpace(v[0])
				if token == "" {
					t.showMain()
					return
				}
				t.runSelf("enroll", token)
				t.showMain()
			})
		})

	case !hasDB:
		list.AddItem("Init database", "Knyt en lokal .kdbx til en server-database", 'i', func() {
			t.collectInputs("Init database", []inputField{
				{label: "Navn (kort id, fx 'privat')"},
				{label: "Sti til lokal .kdbx"},
			}, func(v []string) {
				name, path := strings.TrimSpace(v[0]), strings.TrimSpace(v[1])
				if name == "" || path == "" {
					t.showMain()
					return
				}
				t.runSelf("init", name, path)
				t.showMain()
			})
		})
		list.AddItem("Status", "Vis enrollment-info", 's', func() { t.runSelf("status"); t.showMain() })
		list.AddItem("Databaser", "List server-databaser", 'd', func() { t.runSelf("databases"); t.showMain() })

	default:
		list.AddItem("Synk nu", "Pull + push for en database", 'y', func() {
			t.pickDatabase("Synk", func(name string) { t.runSelf("sync", name); t.showMain() })
		})
		list.AddItem("Pull (hent)", "Hent server-ændringer ind i lokal .kdbx", 'l', func() {
			t.pickDatabase("Pull", func(name string) { t.runSelf("pull", name); t.showMain() })
		})
		list.AddItem("Push (send)", "Send lokale ændringer til serveren", 'u', func() {
			t.pickDatabase("Push", func(name string) { t.runSelf("push", name); t.showMain() })
		})
		list.AddItem("Status", "Vis enrollment + last-seen", 's', func() { t.runSelf("status"); t.showMain() })
		list.AddItem("Databaser", "List lokale + server-databaser", 'd', func() { t.runSelf("databases"); t.showMain() })
		list.AddItem("Enheder", "List enrollede enheder", 'n', func() { t.runSelf("devices"); t.showMain() })
		list.AddItem("Log", "Vis seneste audit-log", 'g', func() { t.runSelf("log"); t.showMain() })
		list.AddItem("Daemon", "Kør kontinuerlig sync (Ctrl-C stopper)", 'm', func() {
			t.runSelf("daemon", "--store-keyring")
			t.showMain()
		})
		list.AddItem("Skift konto (guided)", "Skift hvilken konto denne enhed synker fra", 'k', func() { t.switchAccountWizard() })
		list.AddItem("Avanceret …", "Versioner, gendan, deling", 'a', func() { t.showAdvanced() })
	}

	list.AddItem("Afslut", "Luk menuen", 'q', func() { t.app.Stop() })
	t.setRoot(list)
}

// showAdvanced er undermenuen til de mindre brugte operationer der kræver
// ekstra input (UUID, version, brugernavn).
func (t *tui) showAdvanced() {
	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(" Avanceret ")
	list.SetTitleAlign(tview.AlignLeft)

	list.AddItem("Versioner", "List server-versioner af en entry", 'v', func() {
		t.pickDatabaseThen("Versioner", t.showAdvanced, func(name string) {
			t.collectInputs("Versioner — "+name, []inputField{{label: "Entry-UUID"}}, func(v []string) {
				u := strings.TrimSpace(v[0])
				if u == "" {
					t.showAdvanced()
					return
				}
				t.runSelf("versions", name, u)
				t.showAdvanced()
			})
		})
	})
	list.AddItem("Gendan version", "Rul en entry tilbage til version 1-3", 'r', func() {
		t.pickDatabaseThen("Gendan", t.showAdvanced, func(name string) {
			t.collectInputs("Gendan — "+name, []inputField{
				{label: "Entry-UUID"},
				{label: "Version (1-3)"},
			}, func(v []string) {
				u, n := strings.TrimSpace(v[0]), strings.TrimSpace(v[1])
				if u == "" || n == "" {
					t.showAdvanced()
					return
				}
				t.runSelf("restore", name, u, n)
				t.showAdvanced()
			})
		})
	})
	list.AddItem("Medlemmer", "List medlemmer af en database (kun owner)", 'p', func() {
		t.pickDatabaseThen("Medlemmer", t.showAdvanced, func(name string) {
			t.runSelf("shares", name)
			t.showAdvanced()
		})
	})
	list.AddItem("Del database", "Giv en anden bruger adgang", 'd', func() {
		t.pickDatabaseThen("Del", t.showAdvanced, func(name string) {
			t.collectInputs("Del — "+name, []inputField{{label: "Brugernavn"}}, func(v []string) {
				user := strings.TrimSpace(v[0])
				if user == "" {
					t.showAdvanced()
					return
				}
				t.runSelf("share", name, user)
				t.showAdvanced()
			})
		})
	})
	list.AddItem("Fjern medlem", "Fjern en bruger (eller dig selv)", 'f', func() {
		t.pickDatabaseThen("Fjern", t.showAdvanced, func(name string) {
			t.collectInputs("Fjern — "+name, []inputField{{label: "Brugernavn"}}, func(v []string) {
				user := strings.TrimSpace(v[0])
				if user == "" {
					t.showAdvanced()
					return
				}
				t.runSelf("unshare", name, user)
				t.showAdvanced()
			})
		})
	})

	list.AddItem("Enrollment-token til ny enhed", "Generér token til fx Android-appen (kræver admin-token)", 't', func() {
		t.enrollmentTokenFlow()
	})
	list.AddItem("Glem database", "Fjern en lokal binding fra config (rører ikke server/fil)", 'x', func() {
		t.pickDatabaseThen("Glem database", t.showAdvanced, func(name string) {
			t.runSelf("forget", name)
			t.showAdvanced()
		})
	})

	list.AddItem("‹ Tilbage", "", 'q', t.showMain)
	list.SetDoneFunc(t.showMain) // Esc
	t.setRoot(list)
}

// ============================================================
// Genbrugelige byggeklodser
// ============================================================

// pickDatabase viser en liste over konfigurerede databaser og kalder onPick
// med det valgte navn. "Tilbage"/Esc fører til hovedmenuen.
func (t *tui) pickDatabase(title string, onPick func(name string)) {
	t.pickDatabaseThen(title, t.showMain, onPick)
}

// pickDatabaseThen er som pickDatabase men lader caller bestemme hvor
// "Tilbage"/Esc fører hen (fx tilbage til Avanceret-menuen).
func (t *tui) pickDatabaseThen(title string, back func(), onPick func(name string)) {
	cfg, err := config.Load()
	if err != nil || len(cfg.Databases) == 0 {
		back()
		return
	}

	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(fmt.Sprintf(" %s — vælg database ", title))
	list.SetTitleAlign(tview.AlignLeft)

	for i, d := range cfg.Databases {
		name := d.Name
		shortcut := rune(0)
		if i < 9 {
			shortcut = rune('1' + i)
		}
		list.AddItem(name, d.LocalPath, shortcut, func() { onPick(name) })
	}
	list.AddItem("‹ Tilbage", "", 'q', back)
	list.SetDoneFunc(back) // Esc
	t.setRoot(list)
}

// collectInputs viser en formular med de angivne felter og kalder onSubmit med
// værdierne i samme rækkefølge. Annullér/Esc fører til hovedmenuen.
func (t *tui) collectInputs(title string, fields []inputField, onSubmit func(vals []string)) {
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(fmt.Sprintf(" %s ", title))
	form.SetTitleAlign(tview.AlignLeft)

	inputs := make([]*tview.InputField, len(fields))
	for i, f := range fields {
		in := tview.NewInputField().SetLabel(f.label + ": ")
		if f.password {
			in.SetMaskCharacter('*')
		}
		form.AddFormItem(in)
		inputs[i] = in
	}
	form.AddButton("OK", func() {
		vals := make([]string, len(inputs))
		for i, in := range inputs {
			vals[i] = in.GetText()
		}
		onSubmit(vals)
	})
	form.AddButton("Annullér", t.showMain)
	form.SetCancelFunc(t.showMain) // Esc
	t.setRoot(form)
}

// setRoot tegner en skærm: statushoved øverst, body nedenunder.
func (t *tui) setRoot(body tview.Primitive) {
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(t.headerView(), 5, 0, false).
		AddItem(body, 0, 1, true)
	t.app.SetRoot(layout, true)
}

// headerView bygger statushovedet ud fra den aktuelle config.
func (t *tui) headerView() *tview.TextView {
	cfg, _ := config.Load()

	server := "(ikke sat)"
	state := "[red]ikke enrolled[-]"
	dbs := "(ingen)"
	if cfg != nil {
		if cfg.Server.URL != "" {
			server = cfg.Server.URL
		}
		if cfg.Server.DeviceToken != "" {
			state = "[green]enrolled[-]"
		}
		if len(cfg.Databases) > 0 {
			names := make([]string, 0, len(cfg.Databases))
			for _, d := range cfg.Databases {
				names = append(names, d.Name)
			}
			dbs = strings.Join(names, ", ")
		}
	}

	tv := tview.NewTextView().SetDynamicColors(true)
	tv.SetBorder(true)
	tv.SetTitle(" Status ")
	tv.SetTitleAlign(tview.AlignLeft)
	fmt.Fprintf(tv, "Server    : %s\nEnhed     : %s\nDatabaser : %s", server, state, dbs)
	return tv
}

// runSelf suspenderer skærmen, kører denne binær som en subproces med den
// rigtige terminal (så password-prompts virker), og vender tilbage til menuen
// på et Enter-tryk. Dette er det eneste sted TUI'en faktisk udfører noget.
func (t *tui) runSelf(args ...string) {
	t.runSelfEnv(nil, args...)
}

// runSelfEnv er som runSelf men tilføjer extraEnv ("KEY=value") til
// subprocessens miljø — bruges til at give admin-kommandoer en admin-token
// uden at lægge hemmeligheden i argv (synlig i procesliste) eller kræve at
// brugeren selv har eksporteret env-var'en før TUI'en blev startet.
func (t *tui) runSelfEnv(extraEnv []string, args ...string) {
	t.app.Suspend(func() {
		fmt.Printf("\n› %s %s\n\n", filepath.Base(t.self), strings.Join(args, " "))

		cmd := exec.Command(t.self, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if len(extraEnv) > 0 {
			cmd.Env = append(os.Environ(), extraEnv...)
		}
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "\n[fejl] %v\n", err)
		}

		fmt.Print("\nTryk Enter for at vende tilbage til menuen … ")
		bufio.NewReader(os.Stdin).ReadString('\n')
	})
}

// enrollmentTokenFlow genererer et enrollment-token til en ny enhed (fx
// Android-appen) via `admin user-enrollment <username>`. Admin-token tages fra
// $KEEPASS_DELTASYNC_ADMIN_TOKEN hvis sat; ellers spørger vi (maskeret) og
// injicerer den i subprocessens miljø.
func (t *tui) enrollmentTokenFlow() {
	hasAdminEnv := os.Getenv(adminTokenEnvVar) != ""

	fields := []inputField{{label: "Brugernavn (fx 'hans')"}}
	if !hasAdminEnv {
		fields = append(fields, inputField{label: "Admin-token", password: true})
	}

	t.collectInputs("Enrollment-token til ny enhed", fields, func(v []string) {
		user := strings.TrimSpace(v[0])
		if user == "" {
			t.showAdvanced()
			return
		}
		var extraEnv []string
		if !hasAdminEnv {
			adminTok := strings.TrimSpace(v[1])
			if adminTok == "" {
				t.showAdvanced()
				return
			}
			extraEnv = []string{adminTokenEnvVar + "=" + adminTok}
		}
		t.runSelfEnv(extraEnv, "admin", "user-enrollment", user)
		t.showAdvanced()
	})
}

// ============================================================
// Skift konto-wizard
// ============================================================

// switchAccountWizard guider gennem at skifte hvilken konto denne enhed synker
// fra — på samme server. Den kæder de eksisterende kommandoer:
// enroll → databases → init --bind → sync. Hvert trin udføres ved at shelle ud
// (runSelf), så master-password-prompten i sync virker uændret.
func (t *tui) switchAccountWizard() {
	cfg, err := config.Load()
	if err != nil || cfg.Server.URL == "" {
		t.showNotice("Skift konto",
			"Ingen server i config endnu. Brug 'Enroll enhed' først.", t.showMain)
		return
	}
	server := cfg.Server.URL

	intro := "Dette skifter hvilken konto denne enhed synker fra — på SAMME server:\n\n" +
		"  " + server + "\n\n" +
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
		"    Avanceret → Glem database."
	t.showNotice("Skift konto — sådan virker det", intro, func() {
		t.switchStepEnroll()
	})
}

func (t *tui) switchStepEnroll() {
	t.collectInputs("Skift konto · 1/3 — enroll mod den anden konto", []inputField{
		{label: "Enrollment-token"},
	}, func(v []string) {
		token := strings.TrimSpace(v[0])
		if token == "" {
			t.showMain()
			return
		}
		// Server-URL'en ligger allerede i config (samme server) → ingen --server.
		t.runSelf("enroll", token)
		t.switchStepDatabases()
	})
}

func (t *tui) switchStepDatabases() {
	t.showNotice("Skift konto · 2/3 — find databasen",
		"Nu vises den nye kontos databaser.\n\nNotér UUID'et på den database du vil binde til — du skal indsætte det i næste trin.",
		func() {
			t.runSelf("databases")
			t.switchStepBind()
		})
}

func (t *tui) switchStepBind() {
	t.collectInputs("Skift konto · 3/3 — bind lokal .kdbx", []inputField{
		{label: "Lokalt navn (fx 'adgangskoder')"},
		{label: "Sti til lokal .kdbx"},
		{label: "Remote database-UUID"},
	}, func(v []string) {
		name := strings.TrimSpace(v[0])
		path := strings.TrimSpace(v[1])
		uuid := strings.TrimSpace(v[2])
		if name == "" || path == "" || uuid == "" {
			t.showMain()
			return
		}
		t.runSelf("init", name, path, "--bind", uuid)
		t.switchStepSync(name)
	})
}

func (t *tui) switchStepSync(name string) {
	t.showNotice("Skift konto — næsten færdig",
		"Bindingen '"+name+"' er sat op.\n\n"+
			"Vælg Fortsæt for at synke nu (du bliver bedt om master-passwordet),\n"+
			"eller Annullér for at vende tilbage og synke senere.\n\n"+
			"Husk: den gamle binding kan fjernes med Avanceret → Glem database.",
		func() {
			t.runSelf("sync", name)
			t.showMain()
		})
}

// showNotice viser en informations-/bekræftelses-skærm med rulbar tekst og
// to valg: Fortsæt (onContinue) eller Annullér (tilbage til hovedmenuen).
func (t *tui) showNotice(title, body string, onContinue func()) {
	tv := tview.NewTextView().SetWrap(true)
	tv.SetText(body)
	tv.SetBorder(true)
	tv.SetTitle(" " + title + " ")
	tv.SetTitleAlign(tview.AlignLeft)

	choices := tview.NewList()
	choices.ShowSecondaryText(false)
	choices.AddItem("Fortsæt", "", 0, onContinue)
	choices.AddItem("Annullér", "", 0, t.showMain)
	choices.SetDoneFunc(t.showMain) // Esc

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tv, 0, 1, false).
		AddItem(choices, 4, 0, true)
	t.app.SetRoot(layout, true)
}
