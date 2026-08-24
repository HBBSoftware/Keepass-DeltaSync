// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"net/url"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Firefox-fanen. Udvidelsen står og falder på to kommandoer — `add-local`, der
// peger programmet på en .kdbx, og `install-browser-host`, der registrerer
// native-hosten hos Firefox — og indtil nu fandtes de kun på kommandolinjen.
// Det er den forkerte vej rundt: den bruger der helst vil undgå en terminal er
// præcis den bruger der installerer et program med vinduer og knapper, og på
// Windows lægger installeren ikke engang CLI'en på PATH. TUI'en fik sin
// Firefox-sektion af samme grund; det her er dens pendant.
//
// Fanen viser også hvor CLI'en ligger. Det lyder som en detalje, men det var
// den konkrete blindgyde: programmet var installeret, vejledningen sagde "kør
// keepass-deltasync add-local", og intet sted stod hvor den fil var.

// firefoxGuideURL er den samme side som udvidelsens opsætningsknapper peger på.
const firefoxGuideURL = "https://deltasync.bjoerck-braun.dk/firefox.html"

// firefoxTab bygger fanen: en kort forklaring, knapperne der udfører de to
// trin, og en liste over de databaser udvidelsen ville kunne søge i.
func (u *ui) firefoxTab() fyne.CanvasObject {
	u.ffInfo = widget.NewLabel("")
	u.ffInfo.Wrapping = fyne.TextWrapWord
	u.ffBox = container.NewVBox()

	intro := widget.NewLabel(L.FFIntro)
	intro.Wrapping = fyne.TextWrapWord

	addDB := widget.NewButtonWithIcon(L.FFAddLocal, theme.ContentAddIcon(), func() { u.showAddLocalDialog() })
	addDB.Importance = widget.HighImportance
	install := widget.NewButtonWithIcon(L.FFInstallHost, theme.ConfirmIcon(), func() { u.runInstallHost() })
	install.Importance = widget.HighImportance
	preview := widget.NewButton(L.FFPreview, func() { u.runPreviewHost() })
	guide := widget.NewButtonWithIcon(L.FFGuide, theme.HelpIcon(), func() { u.openGuide() })

	uninstall := widget.NewButton(L.FFUninstallHost, func() { u.runUninstallHost() })
	uninstall.Importance = widget.LowImportance
	refresh := widget.NewButton(L.Refresh, func() { u.refreshFirefox() })

	steps := container.NewHBox(addDB, install, layout.NewSpacer(), guide)
	extras := container.NewHBox(preview, uninstall, layout.NewSpacer(), refresh)

	top := container.NewVBox(intro, steps, extras, widget.NewSeparator(), u.ffInfo)
	return container.NewBorder(top, u.cliPathRow(), nil, nil, container.NewVScroll(u.ffBox))
}

// cliPathRow er linjen nederst der fortæller hvilket program fanen kalder, med
// en kopi-knap. Uden den er svaret på "hvor ligger det?" en søgning i
// Stifinder — og på Windows ligger det som standard i %LOCALAPPDATA%, ikke i
// Program Files, hvilket er nemt at gætte forkert.
func (u *ui) cliPathRow() fyne.CanvasObject {
	path := u.c.path
	if path == "" {
		path = L.CLINotFound
	}
	lbl := widget.NewLabel(path)
	lbl.Truncation = fyne.TextTruncateEllipsis
	copyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		u.fApp.Clipboard().SetContent(u.c.path)
	})
	return container.NewBorder(widget.NewSeparator(), nil,
		widget.NewLabelWithStyle(L.FFCLIPath, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		copyBtn, lbl)
}

// refreshFirefox henter databaselisten og viser hvad udvidelsen ville kunne
// søge i. Hosten indekserer hver database i config'en, både de synkroniserede
// og de lokal-kun, så begge hører hjemme på listen.
func (u *ui) refreshFirefox() {
	if u.ffBox == nil {
		return
	}
	u.ffInfo.SetText(L.Working)
	u.async(func() any {
		ctx, cancel := withTimeout(30 * time.Second)
		defer cancel()
		dbs, r := u.c.databases(ctx)
		return dbResult{rows: toDBRows(dbs), r: r}
	}, func(v any) {
		res := v.(dbResult)
		u.ffBox.RemoveAll()
		if res.r.Err != nil {
			// `databases` fejler kun hvis der hverken er konto eller lokale
			// databaser. Det er ikke en fejl at vise som en fejl — det er
			// tilstanden "du har ikke gjort trin 1 endnu".
			u.ffInfo.SetText(L.FFNone)
			u.ffBox.Refresh()
			return
		}
		if len(res.rows) == 0 {
			u.ffInfo.SetText(L.FFNone)
			u.ffBox.Refresh()
			return
		}
		for _, row := range res.rows {
			u.ffBox.Add(u.firefoxRow(row.db))
		}
		u.ffInfo.SetText(fmt.Sprintf(L.FFCount, len(res.rows)))
		u.ffBox.Refresh()
	})
}

// toDBRows pakker databaserne i den række-type resten af dashboardet bruger, så
// listen kan bygges med samme mønster.
func toDBRows(dbs []database) []dbWithMembers {
	out := make([]dbWithMembers, 0, len(dbs))
	for _, db := range dbs {
		if db.Name == "" {
			continue
		}
		// En database der kun findes på serveren har ingen lokal fil, og så er
		// der intet at indeksere for hosten.
		if !db.Bound && !db.LocalOnly {
			continue
		}
		out = append(out, dbWithMembers{db: db})
	}
	return out
}

// firefoxRow er én database på listen: navn, hvordan den er registreret, stien,
// og en knap der viser præcis hvad browseren ville få at se.
func (u *ui) firefoxRow(db database) fyne.CanvasObject {
	kind := L.FFKindSynced
	if db.LocalOnly {
		kind = L.FFKindLocal
	}
	left := container.NewHBox(
		widget.NewLabelWithStyle(db.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(kind),
	)
	path := widget.NewLabel(db.LocalPath)
	path.Truncation = fyne.TextTruncateEllipsis
	probe := widget.NewButtonWithIcon(L.FFProbe, theme.SearchIcon(), func() { u.showProbeDialog(db.Name) })
	row := container.NewBorder(nil, nil, left, probe, path)
	return container.NewVBox(row, widget.NewSeparator())
}

// showAddLocalDialog er trin 1: peg programmet på en .kdbx. Ingen konto, ingen
// server — filen bliver ikke uploadet nogen steder, og det siger dialogen selv,
// fordi det er det spørgsmål enhver stiller inden de trykker.
func (u *ui) showAddLocalDialog() {
	name := widget.NewEntry()
	name.SetPlaceHolder(L.DBNameHint)
	path := widget.NewEntry()
	path.SetPlaceHolder(L.KdbxFileHint)
	browse := widget.NewButton(L.Browse, func() {
		dialog.ShowFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			defer rc.Close()
			path.SetText(uriToPath(rc.URI()))
		}, u.win)
	})
	pathRow := container.NewBorder(nil, nil, nil, browse, path)

	pw := widget.NewPasswordEntry()
	pw.SetPlaceHolder(L.MasterPwd)
	pw.Disable()
	save := widget.NewCheck(L.FFSavePassword, func(on bool) {
		if on {
			pw.Enable()
		} else {
			pw.SetText("")
			pw.Disable()
		}
	})

	items := []*widget.FormItem{
		widget.NewFormItem(L.DBName, name),
		widget.NewFormItem(L.KdbxFile, pathRow),
		widget.NewFormItem("", save),
		widget.NewFormItem(L.MasterPwd, pw),
	}
	u.showFormDialog(L.FFAddLocalTitle, L.FFAddLocal, items, func(ok bool) {
		if !ok || name.Text == "" || path.Text == "" {
			return
		}
		if save.Checked && pw.Text == "" {
			dialog.ShowError(errSimple(L.FFPasswordMissing), u.win)
			return
		}
		u.log("add-local " + name.Text + " …")
		u.async(func() any {
			// Med --save-password åbnes databasen for at verificere
			// masterpasswordet, før det lægges i nøgleringen. Det tager
			// Argon2-tid, så timeouten er rundhåndet.
			ctx, cancel := withTimeout(5 * time.Minute)
			defer cancel()
			return u.c.addLocal(ctx, name.Text, path.Text, pw.Text)
		}, func(v any) {
			r := v.(result)
			u.log(r.Combined())
			if r.Err != nil {
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			u.refreshFirefox()
			u.refreshDatabases()
			dialog.ShowInformation(L.FFAddLocalTitle, L.FFAddLocalDone, u.win)
		})
	})
}

// runInstallHost er trin 2: registrér native-hosten hos hver Firefox på
// maskinen. Output'et listes, fordi det er svaret på "fandt den min Firefox?".
func (u *ui) runInstallHost() {
	u.log("install-browser-host …")
	u.async(func() any {
		ctx, cancel := withTimeout(60 * time.Second)
		defer cancel()
		return u.c.installBrowserHost(ctx, false)
	}, func(v any) {
		r := v.(result)
		u.log(r.Combined())
		if r.Err != nil {
			dialog.ShowError(errSimple(describeErr(r)), u.win)
			return
		}
		u.showOutputDialog(L.FFHostInstalled, L.FFRestartFirefox+"\n\n"+r.Combined())
	})
}

// runPreviewHost kører --dry-run: samme arbejde, men skriver ingenting. Det er
// også den eneste måde at se hvilke Firefox-varianter maskinen faktisk har.
func (u *ui) runPreviewHost() {
	u.async(func() any {
		ctx, cancel := withTimeout(60 * time.Second)
		defer cancel()
		return u.c.installBrowserHost(ctx, true)
	}, func(v any) {
		r := v.(result)
		if r.Err != nil {
			dialog.ShowError(errSimple(describeErr(r)), u.win)
			return
		}
		u.showOutputDialog(L.FFPreviewTitle, r.Combined())
	})
}

// runUninstallHost fjerner registreringen igen — for hver variant, også dem der
// er afinstalleret siden.
func (u *ui) runUninstallHost() {
	dialog.ShowConfirm(L.FFUninstallHost, L.FFUninstallConfirm, func(ok bool) {
		if !ok {
			return
		}
		u.log("uninstall-browser-host …")
		u.async(func() any {
			ctx, cancel := withTimeout(60 * time.Second)
			defer cancel()
			return u.c.uninstallBrowserHost(ctx)
		}, func(v any) {
			r := v.(result)
			u.log(r.Combined())
			if r.Err != nil {
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			u.showOutputDialog(L.FFUninstallHost, r.Combined())
		})
	}, u.win)
}

// showProbeDialog spørger om masterpasswordet — valgfrit, for ligger det i
// nøgleringen henter hosten det selv — og viser derefter det rå indeks.
func (u *ui) showProbeDialog(name string) {
	pw := widget.NewPasswordEntry()
	pw.SetPlaceHolder(L.FFProbePwdHint)
	items := []*widget.FormItem{widget.NewFormItem(L.MasterPwd, pw)}
	u.showFormDialog(fmt.Sprintf(L.FFProbeTitle, name), L.FFProbe, items, func(ok bool) {
		if !ok {
			return
		}
		u.log("browser-host --probe " + name + " …")
		u.async(func() any {
			ctx, cancel := withTimeout(5 * time.Minute)
			defer cancel()
			return u.c.browserProbe(ctx, name, pw.Text)
		}, func(v any) {
			r := v.(result)
			if r.Err != nil {
				u.log(r.Combined())
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			out := r.Stdout
			if out == "" {
				out = L.FFProbeEmpty
			}
			u.showOutputDialog(fmt.Sprintf(L.FFProbeTitle, name), out)
		})
	})
}

// openGuide åbner opsætningsvejledningen i brugerens browser — den samme side
// udvidelsens egne knapper peger på, så de to steder ikke kan sige forskellige
// ting.
func (u *ui) openGuide() {
	link, err := url.Parse(firefoxGuideURL)
	if err != nil {
		dialog.ShowError(err, u.win)
		return
	}
	if err := u.fApp.OpenURL(link); err != nil {
		dialog.ShowError(err, u.win)
	}
}

// showFirefoxStandalone er Firefox-fanen UDEN dashboard omkring sig, til den
// bruger der ikke har en konto. Søgning kræver ingen server, så guiden må ikke
// være den eneste vej videre — det var netop den fælde TUI'ens menu havde, hvor
// en uden enrollment kun blev tilbudt måder at få en konto på.
func (u *ui) showFirefoxStandalone() {
	back := widget.NewButtonWithIcon(L.Back, theme.NavigateBackIcon(), func() { u.showWizard() })
	title := widget.NewLabelWithStyle(L.TabFirefox, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	header := container.NewBorder(nil, nil, back, nil, title)

	u.win.SetContent(container.NewBorder(header, nil, nil, nil, topPad(u.firefoxTab())))
	u.refreshFirefox()
}
