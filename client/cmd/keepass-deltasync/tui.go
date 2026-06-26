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

	// adminToken holdes kun i hukommelsen for sessionen (som GUI'ens
	// Administration-fane) så admin-handlinger ikke spørger om tokenet hver
	// gang. Injiceres i subprocessens miljø, aldrig i argv. Tom = spørg/brug env.
	adminToken string
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

	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(" keepass-deltasync ")
	list.SetTitleAlign(tview.AlignLeft)

	if !enrolled {
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
		list.AddItem(t.m.miAdvEnroll, t.m.miAdvEnrollDesc, 'a', t.advancedEnrollFlow)
	} else {
		// GUI'ens faner som menupunkter på forsiden. Hvert punkt åbner en
		// undermenu med fanens handlinger.
		list.AddItem(t.m.secDatabases, t.m.secDatabasesDesc, 'd', t.showDatabasesMenu)
		list.AddItem(t.m.secDevices, t.m.secDevicesDesc, 'n', t.showDevicesMenu)
		list.AddItem(t.m.secLog, t.m.secLogDesc, 'g', t.showLogMenu)
		list.AddItem(t.m.secAdmin, t.m.secAdminDesc, 'a', t.showAdminMenu)
		list.AddItem(t.m.secSettings, t.m.secSettingsDesc, 's', t.showSettingsMenu)
	}

	list.AddItem(t.m.miLanguage, fmt.Sprintf(t.m.miLanguageDescFmt, t.m.langDisplayName(t.lang)), 'o', t.toggleLanguage)
	list.AddItem(t.m.miQuit, t.m.miQuitDesc, 'q', func() { t.app.Stop() })
	t.setRoot(list)
}

// menuList laver en bordered tview-liste med en titel — fælles for alle
// undermenuerne (sektionerne).
func (t *tui) menuList(title string) *tview.List {
	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(title)
	list.SetTitleAlign(tview.AlignLeft)
	return list
}

// showDatabasesMenu er "Databaser"-sektionen: alt der hører til de lokale
// databaser og deling — pendant til GUI'ens Databaser-fane.
func (t *tui) showDatabasesMenu() {
	list := t.menuList(t.m.dbMenuTitle)
	back := t.showDatabasesMenu

	list.AddItem(t.m.miSyncNow, t.m.miSyncNowDesc, 'y', func() {
		t.pickDatabaseThen(t.m.pkSync, back, func(name string) { t.runSelf("sync", name); back() })
	})
	list.AddItem(t.m.miPull, t.m.miPullDesc, 'l', func() {
		t.pickDatabaseThen(t.m.pkPull, back, func(name string) { t.runSelf("pull", name); back() })
	})
	list.AddItem(t.m.miPush, t.m.miPushDesc, 'u', func() {
		t.pickDatabaseThen(t.m.pkPush, back, func(name string) { t.runSelf("push", name); back() })
	})
	list.AddItem(t.m.miInit, t.m.miInitDesc, 'i', func() {
		t.collectInputsThen(t.m.miInit, []inputField{
			{label: t.m.fldInitName},
			{label: t.m.fldInitPath},
		}, back, func(v []string) {
			name, path := strings.TrimSpace(v[0]), strings.TrimSpace(v[1])
			if name == "" || path == "" {
				back()
				return
			}
			t.runSelf("init", name, path)
			back()
		})
	})
	list.AddItem(t.m.miMembers, t.m.miMembersDesc, 'p', func() {
		t.pickDatabaseThen(t.m.pkMembers, back, func(name string) { t.runSelf("shares", name); back() })
	})
	list.AddItem(t.m.miShare, t.m.miShareDesc, 'h', func() {
		t.pickDatabaseThen(t.m.pkShare, back, func(name string) {
			t.collectInputsThen(fmt.Sprintf("%s — %s", t.m.pkShare, name), []inputField{{label: t.m.fldUsername}}, back, func(v []string) {
				user := strings.TrimSpace(v[0])
				if user == "" {
					back()
					return
				}
				t.runSelf("share", name, user)
				back()
			})
		})
	})
	list.AddItem(t.m.miUnshare, t.m.miUnshareDesc, 'f', func() {
		t.pickDatabaseThen(t.m.pkUnshare, back, func(name string) {
			t.collectInputsThen(fmt.Sprintf("%s — %s", t.m.pkUnshare, name), []inputField{{label: t.m.fldUsername}}, back, func(v []string) {
				user := strings.TrimSpace(v[0])
				if user == "" {
					back()
					return
				}
				t.runSelf("unshare", name, user)
				back()
			})
		})
	})
	list.AddItem(t.m.miVersions, t.m.miVersionsDesc, 'v', func() {
		t.pickDatabaseThen(t.m.pkVersions, back, func(name string) {
			t.collectInputsThen(fmt.Sprintf("%s — %s", t.m.pkVersions, name), []inputField{{label: t.m.fldEntryUUID}}, back, func(v []string) {
				u := strings.TrimSpace(v[0])
				if u == "" {
					back()
					return
				}
				t.runSelf("versions", name, u)
				back()
			})
		})
	})
	list.AddItem(t.m.miRestore, t.m.miRestoreDesc, 'r', func() {
		t.pickDatabaseThen(t.m.pkRestore, back, func(name string) {
			t.collectInputsThen(fmt.Sprintf("%s — %s", t.m.pkRestore, name), []inputField{
				{label: t.m.fldEntryUUID},
				{label: t.m.fldVersion},
			}, back, func(v []string) {
				u, n := strings.TrimSpace(v[0]), strings.TrimSpace(v[1])
				if u == "" || n == "" {
					back()
					return
				}
				t.runSelf("restore", name, u, n)
				back()
			})
		})
	})
	list.AddItem(t.m.miDeleteDB, t.m.miDeleteDBDesc, 'x', func() {
		t.pickDatabaseThen(t.m.pkDeleteDB, back, func(name string) { t.runSelf("delete-database", name); back() })
	})
	list.AddItem(t.m.miForget, t.m.miForgetDesc, 'e', func() {
		t.pickDatabaseThen(t.m.pkForget, back, func(name string) { t.runSelf("forget", name); back() })
	})
	list.AddItem(t.m.miDatabases, t.m.miDatabasesDesc, 'b', func() { t.runSelf("databases"); back() })

	list.AddItem(t.m.btnBack, "", 'q', t.showMain)
	list.SetDoneFunc(t.showMain) // Esc
	t.setRoot(list)
}

// showDevicesMenu er "Enheder"-sektionen — pendant til GUI'ens Enheder-fane.
func (t *tui) showDevicesMenu() {
	list := t.menuList(t.m.devMenuTitle)
	back := t.showDevicesMenu
	list.AddItem(t.m.miDevices, t.m.miDevicesDesc, 'n', func() { t.runSelf("devices"); back() })
	list.AddItem(t.m.miEnrollTok, t.m.miEnrollTokDesc, 't', func() { t.genEnrollmentToken(back) })
	list.AddItem(t.m.btnBack, "", 'q', t.showMain)
	list.SetDoneFunc(t.showMain)
	t.setRoot(list)
}

// showLogMenu er "Log"-sektionen: server-audit-loggen filtreret på periode —
// pendant til GUI'ens Log-fane med 24t/7d/30d-valg.
func (t *tui) showLogMenu() {
	list := t.menuList(t.m.logMenuTitle)
	back := t.showLogMenu
	list.AddItem(t.m.miLogLatest, t.m.miLogLatestDesc, '1', func() { t.runSelf("log"); back() })
	list.AddItem(t.m.miLog24h, t.m.miLog24hDesc, '2', func() { t.runSelf("log", "--since", "24h"); back() })
	list.AddItem(t.m.miLog7d, t.m.miLog7dDesc, '3', func() { t.runSelf("log", "--since", "168h"); back() })
	list.AddItem(t.m.miLog30d, t.m.miLog30dDesc, '4', func() { t.runSelf("log", "--since", "720h"); back() })
	list.AddItem(t.m.btnBack, "", 'q', t.showMain)
	list.SetDoneFunc(t.showMain)
	t.setRoot(list)
}

// showAdminMenu er "Admin"-sektionen: fuld brugeradministration oven på
// `admin`-subkommandoerne — pendant til GUI'ens Administration-fane. Admin-tokenet
// huskes for sessionen (t.adminToken) og injiceres i subprocessens miljø.
func (t *tui) showAdminMenu() {
	list := t.menuList(t.m.adminMenuTitle)
	back := t.showAdminMenu

	list.AddItem(t.m.miAdmUsers, t.m.miAdmUsersDesc, 'l', func() {
		t.withAdminToken(back, func(env []string) { t.runSelfEnv(env, "admin", "user-list"); back() })
	})
	list.AddItem(t.m.miAdmCreate, t.m.miAdmCreateDesc, 'c', func() {
		t.withAdminToken(back, func(env []string) {
			t.collectInputsThen(t.m.miAdmCreate, []inputField{
				{label: t.m.fldEtUsername},
				{label: t.m.fldAdmDisplay},
			}, back, func(v []string) {
				user := strings.TrimSpace(v[0])
				if user == "" {
					back()
					return
				}
				args := []string{"admin", "user-create", user}
				if d := strings.TrimSpace(v[1]); d != "" {
					args = append(args, "--display-name", d)
				}
				t.runSelfEnv(env, args...)
				back()
			})
		})
	})
	list.AddItem(t.m.miAdmEnable, t.m.miAdmEnableDesc, 'a', func() {
		t.adminUserAction(back, "user-enable", t.m.miAdmEnable)
	})
	list.AddItem(t.m.miAdmDisable, t.m.miAdmDisableDesc, 'x', func() {
		t.adminUserAction(back, "user-disable", t.m.miAdmDisable)
	})
	list.AddItem(t.m.miAdmDelete, t.m.miAdmDeleteDesc, 'r', func() {
		t.adminUserAction(back, "user-delete", t.m.miAdmDelete)
	})
	list.AddItem(t.m.miEnrollTok, t.m.miEnrollTokDesc, 't', func() { t.genEnrollmentToken(back) })
	list.AddItem(t.m.miAdmTokenSQL, t.m.miAdmTokenSQLDesc, 's', func() { t.runSelf("admin", "token-sql"); back() })

	list.AddItem(t.m.btnBack, "", 'q', t.showMain)
	list.SetDoneFunc(t.showMain)
	t.setRoot(list)
}

// adminUserAction kører en admin-subkommando der tager ét brugernavn
// (user-enable/user-disable/user-delete). user-delete spørger selv om bekræftelse
// i terminalen (interaktiv stdin), så vi behøver ingen ekstra dialog her.
func (t *tui) adminUserAction(back func(), sub, title string) {
	t.withAdminToken(back, func(env []string) {
		t.collectInputsThen(title, []inputField{{label: t.m.fldEtUsername}}, back, func(v []string) {
			user := strings.TrimSpace(v[0])
			if user == "" {
				back()
				return
			}
			t.runSelfEnv(env, "admin", sub, user)
			back()
		})
	})
}

// withAdminToken sikrer at vi har en admin-token og kalder action med den
// env-slice der injicerer den i subprocessen. Kilder i rækkefølge:
// (1) $KEEPASS_DELTASYNC_ADMIN_TOKEN (CLI'en læser den selv → env=nil),
// (2) sessionens cache (t.adminToken), (3) en maskeret prompt. back er hvor vi
// vender tilbage ved annullering.
func (t *tui) withAdminToken(back func(), action func(env []string)) {
	if os.Getenv(adminTokenEnvVar) != "" {
		action(nil)
		return
	}
	if t.adminToken != "" {
		action([]string{adminTokenEnvVar + "=" + t.adminToken})
		return
	}
	t.collectInputsThen(t.m.admTokenTitle, []inputField{{label: t.m.fldAdminToken, password: true}}, back, func(v []string) {
		tok := strings.TrimSpace(v[0])
		if tok == "" {
			back()
			return
		}
		t.adminToken = tok
		action([]string{adminTokenEnvVar + "=" + tok})
	})
}

// genEnrollmentToken udsteder et enrollment-token til en eksisterende bruger via
// `admin user-enrollment` (fx til Android-appen eller en ny PC). back styrer hvor
// vi vender tilbage (Admin- eller Enheder-menuen).
func (t *tui) genEnrollmentToken(back func()) {
	t.withAdminToken(back, func(env []string) {
		t.collectInputsThen(t.m.miEnrollTok, []inputField{{label: t.m.fldEtUsername}}, back, func(v []string) {
			user := strings.TrimSpace(v[0])
			if user == "" {
				back()
				return
			}
			t.runSelfEnv(env, "admin", "user-enrollment", user)
			back()
		})
	})
}

// showSettingsMenu er "Indstillinger"-sektionen — pendant til GUI'ens
// Indstillinger-fane (status, skift konto, daemon, sprog).
func (t *tui) showSettingsMenu() {
	list := t.menuList(t.m.settingsMenuTitle)
	back := t.showSettingsMenu
	list.AddItem(t.m.miStatus, t.m.miStatusDesc, 's', func() { t.runSelf("status"); back() })
	list.AddItem(t.m.miSwitch, t.m.miSwitchDesc, 'k', func() { t.switchAccountWizard() })
	list.AddItem(t.m.miDaemon, t.m.miDaemonDesc, 'm', func() { t.runSelf("daemon", "--store-keyring"); back() })
	list.AddItem(t.m.miLanguage, fmt.Sprintf(t.m.miLanguageDescFmt, t.m.langDisplayName(t.lang)), 'o', t.toggleLanguage)
	list.AddItem(t.m.btnBack, "", 'q', t.showMain)
	list.SetDoneFunc(t.showMain)
	t.setRoot(list)
}

// advancedEnrollFlow er TUI-pendanten til GUI'ens avancerede tilmelding: en
// administrator udfylder server + admin-token + bruger, hvorefter vi (1) udsteder
// et enrollment-token via admin-kommandoen og (2) kører `enroll` med det — uden at
// brugeren selv skal kopiere tokenet rundt.
func (t *tui) advancedEnrollFlow() {
	server := tview.NewInputField().SetLabel(t.m.fldAdvServer + ": ")
	admin := tview.NewInputField().SetLabel(t.m.fldAdminToken + ": ").SetMaskCharacter('*')
	username := tview.NewInputField().SetLabel(t.m.fldEtUsername + ": ")
	display := tview.NewInputField().SetLabel(t.m.fldAdmDisplay + ": ")
	device := tview.NewInputField().SetLabel(t.m.fldAdvDevice + ": ")
	newUser := false

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" " + t.m.advEnrollTitle + " ")
	form.SetTitleAlign(tview.AlignLeft)
	form.AddFormItem(server)
	form.AddFormItem(admin)
	form.AddDropDown(t.m.fldAdvMode+": ", []string{t.m.advModeExisting, t.m.advModeNew}, 0, func(_ string, idx int) { newUser = idx == 1 })
	form.AddFormItem(username)
	form.AddFormItem(display)
	form.AddFormItem(device)
	form.AddButton(t.m.btnOK, func() {
		srv := strings.TrimSpace(server.GetText())
		tok := strings.TrimSpace(admin.GetText())
		usr := strings.TrimSpace(username.GetText())
		if srv == "" || tok == "" || usr == "" {
			t.showNotice(t.m.advEnrollTitle, t.m.advEnrollMissing, t.advancedEnrollFlow)
			return
		}
		t.doAdvancedEnroll(srv, tok, usr, strings.TrimSpace(display.GetText()), strings.TrimSpace(device.GetText()), newUser)
	})
	form.AddButton(t.m.btnCancel, t.showMain)
	form.SetCancelFunc(t.showMain)
	t.setRoot(form)
}

// doAdvancedEnroll kører de to trin og viser resultatet. Begge trin opsamles
// (runCapture) frem for at suspendere skærmen, fordi vi selv skal læse tokenet
// fra trin 1's output og føre det videre til `enroll`.
func (t *tui) doAdvancedEnroll(server, adminTok, username, display, device string, newUser bool) {
	env := []string{adminTokenEnvVar + "=" + adminTok}
	var out, errOut string
	var err error
	if newUser {
		args := []string{"admin", "user-create", username, "--server", server}
		if display != "" {
			args = append(args, "--display-name", display)
		}
		out, errOut, err = t.runCapture(env, args...)
	} else {
		out, errOut, err = t.runCapture(env, "admin", "user-enrollment", username, "--server", server)
	}
	if err != nil {
		t.showNotice(t.m.advEnrollTitle, fmt.Sprintf(t.m.advEnrollFailFmt, firstNonEmpty(strings.TrimSpace(errOut), err.Error())), t.showMain)
		return
	}
	token := parseEnrollToken(out)
	if token == "" {
		t.showNotice(t.m.advEnrollTitle, t.m.advEnrollNoToken+"\n\n"+out, t.showMain)
		return
	}

	enrollArgs := []string{"enroll", "--server", server}
	if device != "" {
		enrollArgs = append(enrollArgs, "--device-name", device)
	}
	enrollArgs = append(enrollArgs, token)
	eOut, eErr, eErrErr := t.runCapture(nil, enrollArgs...)
	if eErrErr != nil {
		t.showNotice(t.m.advEnrollTitle, fmt.Sprintf(t.m.advEnrollFailFmt, firstNonEmpty(strings.TrimSpace(eErr), eErrErr.Error())), t.showMain)
		return
	}
	t.showNotice(t.m.advEnrollOkTitle, strings.TrimSpace(eOut), t.showMain)
}

// runCapture kører denne binær UDEN at suspendere skærmen og opsamler stdout/
// stderr — bruges når TUI'en selv skal læse kommandoens output (fx udtrække et
// enrollment-token), i modsætning til runSelf der viser output interaktivt.
func (t *tui) runCapture(extraEnv []string, args ...string) (string, string, error) {
	cmd := exec.Command(t.self, args...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return out.String(), errb.String(), err
}

// parseEnrollToken trækker enrollment-tokenet ud af `admin user-create`/
// `user-enrollment`-output. Begge slutter med en linje på formen
// "keepass-deltasync enroll --server <url> <token>" hvor tokenet er sidste felt.
func parseEnrollToken(stdout string) string {
	for _, line := range strings.Split(stdout, "\n") {
		l := strings.TrimSpace(line)
		if strings.Contains(l, "enroll --server ") {
			f := strings.Fields(l)
			if len(f) > 0 {
				return f[len(f)-1]
			}
		}
	}
	return ""
}

// firstNonEmpty returnerer a hvis den ikke er tom, ellers b.
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
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
	t.collectInputsThen(title, fields, t.showMain, onSubmit)
}

// collectInputsThen er som collectInputs men lader caller bestemme hvor
// Annullér/Esc fører hen (fx tilbage til en sektions-undermenu).
func (t *tui) collectInputsThen(title string, fields []inputField, back func(), onSubmit func(vals []string)) {
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
	form.AddButton(t.m.btnCancel, back)
	form.SetCancelFunc(back) // Esc
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
