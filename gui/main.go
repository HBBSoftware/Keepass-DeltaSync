// SPDX-License-Identifier: GPL-3.0-or-later

// keepass-deltasync-gui er en grafisk skal oven på keepass-deltasync-CLI'en.
// Den hjælper en ny bruger i gang via en guide (tilmeld enhed → tilføj database)
// og giver derefter et simpelt dashboard til at synkronisere. Al rigtig logik —
// krypto, server-kald, config — ligger i CLI'en; GUI'en kalder den bare som en
// subproces. Se README.md for arkitektur og hvordan man bygger til Windows,
// Linux og macOS.
package main

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ui samler al delt tilstand for vinduet. Der er kun ét vindue; vi swapper
// indholdet mellem guiden og dashboardet alt efter om enheden er tilmeldt.
type ui struct {
	fApp     fyne.App
	win      fyne.Window
	c        *cli
	set      settings
	activity *readOnlyEntry // aktivitets-/log-panel (read-only, fuld kontrast)
	statusLb *widget.Label  // status-tekst på dashboardet

	// Database-fanen: en hierarkisk liste hvor hver database står på sin egen
	// linje med inline-handlinger, og dens medlemmer vises lige under den.
	dbBox  *fyne.Container // beholder for database-kortene (genopbygges ved refresh)
	dbInfo *widget.Label   // linje over listen: antal / fejl / tom
	dbHint *widget.Label   // hover-hint-linje nederst (hvad ikon-knapper gør)

	// Enheder-fanen: kontoens tilmeldte enheder (`devices`, kontobredt) som en
	// liste med inline-ikoner pr. enhed.
	devBox  *fyne.Container
	devInfo *widget.Label
	devHint *widget.Label // hover-hint-linje nederst

	// Log-fanen: serverens audit-log (`keepass-deltasync log`) som en liste,
	// med en periode-vælger der styrer --since.
	logBox   *fyne.Container
	logInfo  *widget.Label
	logSince string // Go-duration til --since ("24h", "168h", "" = alle)

	// Administration-fanen: brugeradministration via `admin`-subkommandoer.
	// adminToken holdes KUN i hukommelsen (aldrig gemt på disk) af hensyn til
	// sikkerhed — det er en fuld admin-nøgle.
	adminBox   *fyne.Container
	adminInfo  *widget.Label
	adminHint  *widget.Label
	adminToken string

	// Hjælpe-panelet i bunden (wiki-agtig beskrivelse af den aktuelle fane).
	helpText  *widget.RichText
	helpIcon  *widget.Icon
	helpTitle *canvas.Text

	currentUser string // brugernavn fra status — default i "tilføj enhed"
}

func main() {
	set := loadSettings()
	setLang(lang(set.Language))

	fApp := app.NewWithID("dk.bjoerck-braun.deltasync.gui")
	applyTheme(fApp, set.Theme)
	helpEnabled = set.ShowHelpPanel
	win := fApp.NewWindow(L.AppTitle)

	u := &ui{
		fApp:     fApp,
		win:      win,
		c:        &cli{path: locateCLI(set.CLIPath)},
		set:      set,
		activity: newActivityLog(),
	}

	debugf("start: cli=%q lang=%s", u.c.path, set.Language)

	win.SetContent(centeredSpinner(L.Working))
	// Lidt højere når hjælpe-panelet er slået til, så det får plads i bunden.
	if helpEnabled {
		win.Resize(fyne.NewSize(820, 760))
	} else {
		win.Resize(fyne.NewSize(820, 560))
	}

	// VIGTIGT: opstartslogikken (inkl. "find CLI"-dialogen og det første
	// status-kald) skal køre FØRST når Fynes event-loop er i gang. Hvis vi gør
	// det før ShowAndRun, bliver en dialog aldrig tegnet, og vinduet ser ud til
	// at hænge på "Arbejder…". OnStarted kaldes én gang på UI-tråden efter
	// ShowAndRun og er det rette sted til opstart.
	fApp.Lifecycle().SetOnStarted(func() {
		debugf("onstarted: cli=%q", u.c.path)
		if u.c.path == "" {
			u.promptForCLI(func() { u.route() })
		} else {
			u.route()
		}
	})

	win.ShowAndRun()
}

// route beslutter hvad vinduet skal vise: guiden hvis enheden ikke er tilmeldt,
// ellers dashboardet. Status hentes asynkront så UI'en ikke fryser.
func (u *ui) route() {
	u.win.SetContent(centeredSpinner(L.Working))
	u.async(func() any {
		ctx, cancel := withTimeout(30 * time.Second)
		defer cancel()
		return u.c.status(ctx)
	}, func(v any) {
		s := v.(status)
		debugf("route: enrolled=%v raw=%q", s.Enrolled, s.Raw)
		if s.Enrolled {
			u.showDashboard()
		} else {
			u.showWizard()
		}
	})
}

// promptForCLI viser en lille dialog der beder brugeren udpege CLI'en, og
// gemmer valget. onDone kaldes når en gyldig sti er valgt.
func (u *ui) promptForCLI(onDone func()) {
	info := widget.NewLabel(L.CLINotFound + "\n\n" + L.CLILocateHint)
	info.Wrapping = fyne.TextWrapWord
	pathEntry := widget.NewEntry()
	pathEntry.SetPlaceHolder(binaryName())

	browse := widget.NewButton(L.Browse, func() {
		dialog.ShowFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			defer rc.Close()
			pathEntry.SetText(rc.URI().Path())
		}, u.win)
	})

	form := container.NewBorder(nil, nil, nil, browse, pathEntry)
	content := container.NewVBox(info, widget.NewLabel(L.CLIPathLabel), form)

	d := dialog.NewCustomConfirm(L.CLILocate, L.Save, L.Cancel, content, func(ok bool) {
		if !ok {
			return
		}
		p := pathEntry.Text
		if !fileExists(p) {
			dialog.ShowError(errSimple(L.CLINotFound), u.win)
			return
		}
		u.c.path = p
		u.set.CLIPath = p
		_ = saveSettings(u.set)
		onDone()
	}, u.win)
	d.Resize(fyne.NewSize(600, 320))
	d.Show()
}
