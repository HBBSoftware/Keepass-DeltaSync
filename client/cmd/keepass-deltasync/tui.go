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
// med serveren og rører hverken crypto eller config-skrivning — undtagen det
// ene UI-præference-felt cfg.Language som sprog-skifteren persisterer.
//
// Sproget er engelsk som default; brugeren kan skifte til dansk via
// menupunktet "Language / Sprog", og valget huskes i config.
func runTui(args []string) error {
	self, err := os.Executable()
	if err != nil || self == "" {
		self = os.Args[0]
	}

	lang := ""
	if cfg, err := config.Load(); err == nil && cfg != nil {
		lang = cfg.Language
	}
	m, resolved := messagesFor(lang)

	t := &tui{app: tview.NewApplication(), self: self, m: m, lang: resolved}
	t.app.EnableMouse(true)
	t.showMain()
	return t.app.Run()
}

type tui struct {
	app  *tview.Application
	self string
	m    *msgs  // aktivt sprogs strenge
	lang string // normaliseret sprogkode ("en"/"da")
}

// inputField beskriver ét felt i en collectInputs-formular.
type inputField struct {
	label    string
	password bool
}

// ============================================================
// Sprog
// ============================================================

// toggleLanguage skifter mellem engelsk og dansk, persisterer valget i config
// (best-effort — UI'en skifter for denne session uanset om skrivningen
// lykkes), genindlæser strengsættet og gentegner hovedmenuen.
func (t *tui) toggleLanguage() {
	next := langDA
	if t.lang == langDA {
		next = langEN
	}
	if cfg, err := config.Load(); err == nil && cfg != nil {
		cfg.Language = next
		_ = config.Save(cfg)
	}
	t.m, t.lang = messagesFor(next)
	t.showMain()
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
			SetText(t.m.cfgReadErr + err.Error()).
			AddButtons([]string{t.m.btnExit}).
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
		list.AddItem(t.m.miEnroll, t.m.miEnrollDesc, 'e', func() {
			t.collectInputs(t.m.miEnroll, []inputField{{label: t.m.fldEnrollToken}}, func(v []string) {
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
		list.AddItem(t.m.miInit, t.m.miInitDesc, 'i', func() {
			t.collectInputs(t.m.miInit, []inputField{
				{label: t.m.fldInitName},
				{label: t.m.fldInitPath},
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
		list.AddItem(t.m.miStatus, t.m.miStatusDescNoDb, 's', func() { t.runSelf("status"); t.showMain() })
		list.AddItem(t.m.miDatabases, t.m.miDatabasesDescSrv, 'd', func() { t.runSelf("databases"); t.showMain() })

	default:
		list.AddItem(t.m.miSyncNow, t.m.miSyncNowDesc, 'y', func() {
			t.pickDatabase(t.m.pkSync, func(name string) { t.runSelf("sync", name); t.showMain() })
		})
		list.AddItem(t.m.miPull, t.m.miPullDesc, 'l', func() {
			t.pickDatabase(t.m.pkPull, func(name string) { t.runSelf("pull", name); t.showMain() })
		})
		list.AddItem(t.m.miPush, t.m.miPushDesc, 'u', func() {
			t.pickDatabase(t.m.pkPush, func(name string) { t.runSelf("push", name); t.showMain() })
		})
		list.AddItem(t.m.miStatus, t.m.miStatusDesc, 's', func() { t.runSelf("status"); t.showMain() })
		list.AddItem(t.m.miDatabases, t.m.miDatabasesDesc, 'd', func() { t.runSelf("databases"); t.showMain() })
		list.AddItem(t.m.miDevices, t.m.miDevicesDesc, 'n', func() { t.runSelf("devices"); t.showMain() })
		list.AddItem(t.m.miLog, t.m.miLogDesc, 'g', func() { t.runSelf("log"); t.showMain() })
		list.AddItem(t.m.miDaemon, t.m.miDaemonDesc, 'm', func() {
			t.runSelf("daemon", "--store-keyring")
			t.showMain()
		})
		list.AddItem(t.m.miSwitch, t.m.miSwitchDesc, 'k', func() { t.switchAccountWizard() })
		list.AddItem(t.m.miAdvanced, t.m.miAdvancedDesc, 'a', func() { t.showAdvanced() })
	}

	list.AddItem(t.m.miLanguage, fmt.Sprintf(t.m.miLanguageDescFmt, t.m.langDisplayName(t.lang)), 'o', t.toggleLanguage)
	list.AddItem(t.m.miQuit, t.m.miQuitDesc, 'q', func() { t.app.Stop() })
	t.setRoot(list)
}

// showAdvanced er undermenuen til de mindre brugte operationer der kræver
// ekstra input (UUID, version, brugernavn).
func (t *tui) showAdvanced() {
	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(t.m.advTitle)
	list.SetTitleAlign(tview.AlignLeft)

	list.AddItem(t.m.miVersions, t.m.miVersionsDesc, 'v', func() {
		t.pickDatabaseThen(t.m.pkVersions, t.showAdvanced, func(name string) {
			t.collectInputs(fmt.Sprintf("%s — %s", t.m.pkVersions, name), []inputField{{label: t.m.fldEntryUUID}}, func(v []string) {
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
	list.AddItem(t.m.miRestore, t.m.miRestoreDesc, 'r', func() {
		t.pickDatabaseThen(t.m.pkRestore, t.showAdvanced, func(name string) {
			t.collectInputs(fmt.Sprintf("%s — %s", t.m.pkRestore, name), []inputField{
				{label: t.m.fldEntryUUID},
				{label: t.m.fldVersion},
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
	list.AddItem(t.m.miMembers, t.m.miMembersDesc, 'p', func() {
		t.pickDatabaseThen(t.m.pkMembers, t.showAdvanced, func(name string) {
			t.runSelf("shares", name)
			t.showAdvanced()
		})
	})
	list.AddItem(t.m.miShare, t.m.miShareDesc, 'd', func() {
		t.pickDatabaseThen(t.m.pkShare, t.showAdvanced, func(name string) {
			t.collectInputs(fmt.Sprintf("%s — %s", t.m.pkShare, name), []inputField{{label: t.m.fldUsername}}, func(v []string) {
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
	list.AddItem(t.m.miUnshare, t.m.miUnshareDesc, 'f', func() {
		t.pickDatabaseThen(t.m.pkUnshare, t.showAdvanced, func(name string) {
			t.collectInputs(fmt.Sprintf("%s — %s", t.m.pkUnshare, name), []inputField{{label: t.m.fldUsername}}, func(v []string) {
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

	list.AddItem(t.m.miEnrollTok, t.m.miEnrollTokDesc, 't', func() {
		t.enrollmentTokenFlow()
	})
	list.AddItem(t.m.miForget, t.m.miForgetDesc, 'x', func() {
		t.pickDatabaseThen(t.m.pkForget, t.showAdvanced, func(name string) {
			t.runSelf("forget", name)
			t.showAdvanced()
		})
	})

	list.AddItem(t.m.btnBack, "", 'q', t.showMain)
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
	list.SetTitle(fmt.Sprintf(" %s — %s ", title, t.m.selectDbSuffix))
	list.SetTitleAlign(tview.AlignLeft)

	for i, d := range cfg.Databases {
		name := d.Name
		shortcut := rune(0)
		if i < 9 {
			shortcut = rune('1' + i)
		}
		list.AddItem(name, d.LocalPath, shortcut, func() { onPick(name) })
	}
	list.AddItem(t.m.btnBack, "", 'q', back)
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
	form.AddButton(t.m.btnOK, func() {
		vals := make([]string, len(inputs))
		for i, in := range inputs {
			vals[i] = in.GetText()
		}
		onSubmit(vals)
	})
	form.AddButton(t.m.btnCancel, t.showMain)
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

	server := t.m.hNotSet
	state := t.m.hNotEnrolled
	dbs := t.m.hNone
	if cfg != nil {
		if cfg.Server.URL != "" {
			server = cfg.Server.URL
		}
		if cfg.Server.DeviceToken != "" {
			state = t.m.hEnrolled
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
	tv.SetTitle(t.m.statusTitle)
	tv.SetTitleAlign(tview.AlignLeft)
	fmt.Fprintf(tv, t.m.hdrFmt, server, state, dbs)
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
			fmt.Fprintf(os.Stderr, t.m.errFmt, err)
		}

		fmt.Print(t.m.pressEnter)
		bufio.NewReader(os.Stdin).ReadString('\n')
	})
}

// enrollmentTokenFlow genererer et enrollment-token til en ny enhed (fx
// Android-appen) via `admin user-enrollment <username>`. Admin-token tages fra
// $KEEPASS_DELTASYNC_ADMIN_TOKEN hvis sat; ellers spørger vi (maskeret) og
// injicerer den i subprocessens miljø.
func (t *tui) enrollmentTokenFlow() {
	hasAdminEnv := os.Getenv(adminTokenEnvVar) != ""

	fields := []inputField{{label: t.m.fldEtUsername}}
	if !hasAdminEnv {
		fields = append(fields, inputField{label: t.m.fldAdminToken, password: true})
	}

	t.collectInputs(t.m.miEnrollTok, fields, func(v []string) {
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
		t.showNotice(t.m.saNoServerTitle, t.m.saNoServerBody, t.showMain)
		return
	}
	server := cfg.Server.URL

	t.showNotice(t.m.saIntroTitle, fmt.Sprintf(t.m.saIntroFmt, server), func() {
		t.switchStepEnroll()
	})
}

func (t *tui) switchStepEnroll() {
	t.collectInputs(t.m.saStep1Title, []inputField{
		{label: t.m.fldEnrollToken},
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
	t.showNotice(t.m.saStep2Title, t.m.saStep2Body, func() {
		t.runSelf("databases")
		t.switchStepBind()
	})
}

func (t *tui) switchStepBind() {
	t.collectInputs(t.m.saStep3Title, []inputField{
		{label: t.m.fldSaName},
		{label: t.m.fldSaPath},
		{label: t.m.fldSaUUID},
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
	t.showNotice(t.m.saDoneTitle, fmt.Sprintf(t.m.saDoneFmt, name), func() {
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
	choices.AddItem(t.m.btnContinue, "", 0, onContinue)
	choices.AddItem(t.m.btnCancel, "", 0, t.showMain)
	choices.SetDoneFunc(t.showMain) // Esc

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tv, 0, 1, false).
		AddItem(choices, 4, 0, true)
	t.app.SetRoot(layout, true)
}
