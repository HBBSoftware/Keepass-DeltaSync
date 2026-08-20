// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// showDashboard er hovedskærmen efter tilmelding: en faneblads-visning med
// databaser, aktivitetslog og indstillinger.
func (u *ui) showDashboard() {
	debugf("showDashboard: begin")
	u.statusLb = widget.NewLabel("")
	u.statusLb.Wrapping = fyne.TextWrapWord

	dbTab := u.databasesTab()
	debugf("showDashboard: databasesTab built")
	devTab := u.devicesTab()
	debugf("showDashboard: devicesTab built")
	actTab := u.activityTab()
	debugf("showDashboard: activityTab built")
	lgTab := u.logTab()
	debugf("showDashboard: logTab built")
	adTab := u.adminTab()
	debugf("showDashboard: adminTab built")
	setTab := u.settingsTab()
	debugf("showDashboard: settingsTab built")

	tabs := container.NewAppTabs(
		container.NewTabItem(L.TabDatabases, topPad(dbTab)),
		container.NewTabItem(L.TabDevices, topPad(devTab)),
		container.NewTabItem(L.TabActivity, topPad(actTab)),
		container.NewTabItem(L.TabLog, topPad(lgTab)),
		container.NewTabItem(L.TabAdmin, topPad(adTab)),
		container.NewTabItem(L.TabSettings, topPad(setTab)),
	)

	if helpEnabled {
		// Wiki-agtigt hjælpe-panel i bunden, der følger den valgte fane.
		panel := u.buildHelpPanel()
		u.updateHelp(tabs.SelectedIndex())
		tabs.OnSelected = func(*container.TabItem) { u.updateHelp(tabs.SelectedIndex()) }
		u.win.SetContent(container.NewBorder(nil, panel, nil, nil, tabs))
	} else {
		u.helpText = nil
		u.win.SetContent(tabs)
	}
	debugf("showDashboard: content set")

	u.refreshStatus()
	u.refreshDatabases()
	u.refreshDevices()
	debugf("showDashboard: refresh kicked off")
}

// databasesTab bygger fanen som en hierarkisk liste: hver database står på sin
// egen linje med sine handlinger (synk/del/glem) inline, og lige under den vises
// de brugere der er koblet til den, hver med en "fjern medlem"-knap. Mere
// integreret end at vælge en række og bruge en separat knapbjælke.
func (u *ui) databasesTab() fyne.CanvasObject {
	u.dbInfo = widget.NewLabel("")
	u.dbHint = widget.NewLabel(" ")
	u.dbBox = container.NewVBox()

	add := widget.NewButtonWithIcon(L.AddDatabase, theme.ContentAddIcon(), func() { u.showAddDBDialog() })
	add.Importance = widget.HighImportance
	syncAll := widget.NewButtonWithIcon(L.SyncAll, theme.ViewRefreshIcon(), func() { u.syncAll() })
	refresh := widget.NewButton(L.Refresh, func() { u.refreshDatabases() })
	toolbar := container.NewHBox(add, syncAll, layout.NewSpacer(), refresh)
	top := container.NewVBox(toolbar, u.dbInfo)

	return container.NewBorder(top, u.dbHint, nil, nil, container.NewVScroll(u.dbBox))
}

// setHint opdaterer hover-hint-linjen nederst på databasefanen.
func (u *ui) setHint(s string) {
	if u.dbHint == nil {
		return
	}
	if s == "" {
		u.dbHint.SetText(" ")
		return
	}
	u.dbHint.SetText("➤  " + s)
}

// dbCard bygger ét database-"kort": en header-linje med navn, status, sti og
// inline-handlinger, og derunder de brugere (medlemmer) der er koblet til den —
// hver med en fjern-knap. memErr != "" vises i stedet for medlemmer (fx hvis man
// ikke er ejer og derfor ikke må se medlemslisten).
func (u *ui) dbCard(db database, members []member, memErr string) fyne.CanvasObject {
	// Statusmarkør + navn + oprettet til venstre, sti i midten (afkortes), og
	// ikon-knapper til højre — på én linje.
	status := "○"
	if db.Bound {
		status = "●"
	}
	left := container.NewHBox(
		widget.NewLabel(status),
		widget.NewLabelWithStyle(db.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(db.Created),
	)
	path := widget.NewLabel(db.LocalPath)
	path.Truncation = fyne.TextTruncateEllipsis

	info := newHintIconButton(L.DBInfoTitle, theme.InfoIcon(), func() { u.showDBInfo(db) }, u.setHint)
	more := u.dbMoreButton(db)
	var actions *fyne.Container
	if db.Bound {
		sync := newHintIconButton(L.Sync, theme.ViewRefreshIcon(), func() { u.promptSync(db.Name) }, u.setHint)
		sync.Importance = widget.HighImportance
		share := newHintIconButton(L.ShareDatabase, theme.MailForwardIcon(), func() { u.shareDatabase(db) }, u.setHint)
		forget := newHintIconButton(L.ForgetDatabase, theme.CancelIcon(), func() { u.forgetDatabase(db) }, u.setHint)
		actions = container.NewHBox(sync, share, forget, more, info)
	} else {
		// Server-only: enten forbinde din EGEN eksisterende database (ny enhed,
		// init --bind) eller sætte en database op som er DELT med dig (init-shared).
		bind := newHintIconButton(L.BindExisting, theme.FolderOpenIcon(), func() { u.bindDatabase(db) }, u.setHint)
		bind.Importance = widget.HighImportance
		setup := newHintIconButton(L.SetupShared, theme.DownloadIcon(), func() { u.setupSharedDB(db) }, u.setHint)
		actions = container.NewHBox(bind, setup, more, info)
	}

	header := container.NewBorder(nil, nil, left, actions, path)

	rows := []fyne.CanvasObject{header}
	switch {
	case !db.Bound:
		// server-only: intet medlems-opslag muligt (kræver lokal binding)
	case memErr != "":
		rows = append(rows, indented(widget.NewLabel(L.MembersUnavailable+" "+memErr)))
	case len(members) == 0:
		rows = append(rows, indented(widget.NewLabel(L.NoMembers)))
	default:
		for _, m := range members {
			rows = append(rows, u.memberRow(db, m))
		}
	}
	rows = append(rows, widget.NewSeparator())
	return container.NewVBox(rows...)
}

// showDBInfo viser detaljer om databasen — navn, server-UUID, oprettet og sti —
// med mulighed for at kopiere UUID'en til udklipsholderen.
func (u *ui) showDBInfo(db database) {
	idEntry := widget.NewEntry()
	idEntry.SetText(db.ID)
	copyBtn := widget.NewButtonWithIcon(L.Copy, theme.ContentCopyIcon(), func() {
		u.fApp.Clipboard().SetContent(db.ID)
	})
	idRow := container.NewBorder(nil, nil, nil, copyBtn, idEntry)

	form := widget.NewForm(
		widget.NewFormItem(L.ColName, widget.NewLabel(db.Name)),
		widget.NewFormItem(L.ColID, idRow),
		widget.NewFormItem(L.ColCreated, widget.NewLabel(db.Created)),
		widget.NewFormItem(L.ColPath, widget.NewLabel(db.LocalPath)),
	)
	d := dialog.NewCustom(L.DBInfoTitle, L.Close, container.NewPadded(form), u.win)
	d.Resize(fyne.NewSize(600, 260))
	d.Show()
}

// memberRow bygger én indrykket medlemslinje med en fjern-knap (ikke for ejeren).
func (u *ui) memberRow(db database, m member) fyne.CanvasObject {
	text := m.Role + "   " + m.Username
	if m.DisplayName != "" {
		text += "   (" + m.DisplayName + ")"
	}
	lbl := widget.NewLabel(text)

	var right fyne.CanvasObject
	if m.Role != "owner" {
		right = widget.NewButton(L.RemoveMember, func() { u.unshareMember(db, m) })
	}
	return indented(container.NewBorder(nil, nil, nil, right, lbl))
}

// shareDatabase deler databasen med en anden bruger via
// `share --password-stdin <db> <username>`. Masterpasswordet bruges lokalt til
// at wrappe database-nøglen til modtagerens enhed (serveren ser det aldrig).
func (u *ui) shareDatabase(db database) {
	username := widget.NewEntry()
	username.SetPlaceHolder(L.Username)
	pw := widget.NewPasswordEntry()
	items := []*widget.FormItem{
		widget.NewFormItem(L.ShareWith, username),
		widget.NewFormItem(L.MasterPwd, pw),
	}
	u.showFormDialog(fmt.Sprintf(L.ShareTitle, db.Name), L.ShareDatabase, items, func(ok bool) {
		if !ok || username.Text == "" || pw.Text == "" {
			return
		}
		name := db.Name
		u.async(func() any {
			ctx, cancel := withTimeout(60 * time.Second)
			defer cancel()
			return u.c.share(ctx, name, username.Text, pw.Text)
		}, func(v any) {
			r := v.(result)
			u.log(r.Combined())
			if r.Err != nil {
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			u.refreshDatabases()
		})
	})
}

// unshareMember fjerner et medlem fra databasen via `unshare <db> <username>`.
func (u *ui) unshareMember(db database, m member) {
	msg := fmt.Sprintf(L.ConfirmUnshare, m.Username, db.Name)
	dialog.ShowConfirm(L.RemoveMember, msg, func(ok bool) {
		if !ok {
			return
		}
		dbName, user := db.Name, m.Username
		u.async(func() any {
			ctx, cancel := withTimeout(30 * time.Second)
			defer cancel()
			return u.c.unshare(ctx, dbName, user)
		}, func(v any) {
			r := v.(result)
			u.log(r.Combined())
			if r.Err != nil {
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			u.refreshDatabases()
		})
	}, u.win)
}

// bindDatabase forbinder en server-only database, du SELV ejer (typisk på en ny
// enhed), til en eksisterende lokal .kdbx via `init --bind <uuid> <name> <path>`.
// Filen skal allerede findes (init --bind opretter den ikke). Bagefter skal man
// synkronisere for at hente entries.
func (u *ui) bindDatabase(db database) {
	dialog.ShowFileOpen(func(rc fyne.URIReadCloser, err error) {
		if err != nil || rc == nil {
			return
		}
		defer rc.Close()
		path := uriToPath(rc.URI())
		name, uuid := db.Name, db.ID
		u.async(func() any {
			ctx, cancel := withTimeout(60 * time.Second)
			defer cancel()
			return u.c.initBind(ctx, name, path, uuid)
		}, func(v any) {
			r := v.(result)
			u.log(r.Combined())
			if r.Err != nil {
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			u.refreshDatabases()
			dialog.ShowInformation(L.BindExisting, L.BoundNowSync, u.win)
		})
	}, u.win)
}

// setupSharedDB sætter en database, der er delt med dig (vises som "kun på
// server"), op lokalt via `init-shared --password-stdin <remote> <path>`. Du
// vælger en lokal .kdbx-sti og et NYT lokalt password til din egen kopi.
func (u *ui) setupSharedDB(db database) {
	path := widget.NewEntry()
	path.SetPlaceHolder(L.KdbxFileHint)
	browse := widget.NewButton(L.Browse, func() {
		dialog.ShowFileSave(func(wc fyne.URIWriteCloser, err error) {
			if err != nil || wc == nil {
				return
			}
			defer wc.Close()
			path.SetText(uriToPath(wc.URI()))
		}, u.win)
	})
	pathRow := container.NewBorder(nil, nil, nil, browse, path)
	pw := widget.NewPasswordEntry()
	items := []*widget.FormItem{
		widget.NewFormItem(L.KdbxFile, pathRow),
		widget.NewFormItem(L.NewLocalPassword, pw),
	}
	u.showFormDialog(fmt.Sprintf(L.SetupSharedTitle, db.Name), L.SetupShared, items, func(ok bool) {
		if !ok || path.Text == "" || pw.Text == "" {
			return
		}
		remote := db.Name
		u.async(func() any {
			ctx, cancel := withTimeout(2 * time.Minute)
			defer cancel()
			return u.c.initShared(ctx, remote, path.Text, pw.Text)
		}, func(v any) {
			r := v.(result)
			u.log(r.Combined())
			if r.Err != nil {
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			u.refreshDatabases()
		})
	})
}

// forgetDatabase fjerner den lokale binding. Det rører hverken .kdbx-filen eller
// databasen på serveren — kun koblingen i config.
//
// Den prøver FØRST med GUID (db.ID) — symmetrisk med `init --bind <uuid>` og
// robust hvis bindingen blev oprettet forkert og mangler/har et forkert navn.
// Falder tilbage til navn hvis CLI'ens `forget` ikke kan opløse GUID'et (f.eks.
// en ældre CLI der kun tager navn). Begge tilfælde rammer dermed samme knap.
func (u *ui) forgetDatabase(db database) {
	msg := fmt.Sprintf(L.ConfirmForget, db.Name)
	dialog.ShowConfirm(L.ForgetDatabase, msg, func(ok bool) {
		if !ok {
			return
		}
		name, id := db.Name, db.ID
		u.async(func() any {
			ctx, cancel := withTimeout(30 * time.Second)
			defer cancel()
			if id != "" {
				if r := u.c.run(ctx, "", "forget", id); r.Err == nil || name == "" {
					return r
				}
				// GUID kunne ikke opløses — prøv navn som fallback.
			}
			return u.c.run(ctx, "", "forget", name)
		}, u.afterDBOp)
	}, u.win)
}

// dbMoreButton laver en "flere handlinger"-knap (⋮) der åbner en popup-menu med
// de mindre brugte database-operationer, så rækken ikke fyldes med ikoner: for
// bundne databaser push/pull (ensrettet sync) + slet-på-server; for server-only
// kun slet-på-server.
func (u *ui) dbMoreButton(db database) *hintIconButton {
	more := newHintIconButton(L.MoreActions, theme.MoreVerticalIcon(), nil, u.setHint)
	more.OnTapped = func() {
		var items []*fyne.MenuItem
		if db.Bound {
			items = append(items,
				fyne.NewMenuItem(L.PushNow, func() { u.pushDatabase(db) }),
				fyne.NewMenuItem(L.PullNow, func() { u.pullDatabase(db) }),
				fyne.NewMenuItem(L.VersionsMenu, func() { u.showVersionsDialog(db) }),
				fyne.NewMenuItemSeparator(),
			)
		}
		del := fyne.NewMenuItem(L.DeleteOnServer, func() { u.deleteDatabaseServer(db) })
		items = append(items, del)

		menu := fyne.NewMenu("", items...)
		pos := u.fApp.Driver().AbsolutePositionForObject(more)
		widget.ShowPopUpMenuAtPosition(menu, u.win.Canvas(), fyne.NewPos(pos.X, pos.Y+more.Size().Height))
	}
	return more
}

// afterDBOp er den fælles done-callback for database-operationer: log output,
// vis evt. fejl, og genopbyg listen ved succes.
func (u *ui) afterDBOp(v any) {
	r := v.(result)
	u.log(r.Combined())
	if r.Err != nil {
		dialog.ShowError(errSimple(describeErr(r)), u.win)
		return
	}
	u.refreshDatabases()
}

// pushDatabase kører `push` (kun upload) for databasen efter at have bedt om
// masterpasswordet.
func (u *ui) pushDatabase(db database) {
	u.promptDBPassword(L.PushNow+" — "+db.Name, L.PushNow, db.Name, func(name, pw string) {
		u.log("push " + name + " …")
		u.async(func() any {
			ctx, cancel := withTimeout(10 * time.Minute)
			defer cancel()
			return u.c.push(ctx, name, pw)
		}, u.afterDBOp)
	})
}

// pullDatabase kører `pull` (kun download) for databasen efter at have bedt om
// masterpasswordet.
func (u *ui) pullDatabase(db database) {
	u.promptDBPassword(L.PullNow+" — "+db.Name, L.PullNow, db.Name, func(name, pw string) {
		u.log("pull " + name + " …")
		u.async(func() any {
			ctx, cancel := withTimeout(10 * time.Minute)
			defer cancel()
			return u.c.pull(ctx, name, pw)
		}, u.afterDBOp)
	})
}

// promptDBPassword viser en password-dialog og kalder run(name, pw) hvis brugeren
// bekræfter med et udfyldt felt.
func (u *ui) promptDBPassword(title, confirm, name string, run func(name, pw string)) {
	pw := widget.NewPasswordEntry()
	items := []*widget.FormItem{widget.NewFormItem(L.MasterPwd, pw)}
	u.showFormDialog(title, confirm, items, func(ok bool) {
		if !ok || pw.Text == "" {
			return
		}
		run(name, pw.Text)
	})
}

// deleteDatabaseServer sletter databasen PERMANENT på serveren via
// `delete-database`. Til forskel fra forget (lokal binding) fjerner dette alt
// server-side for alle brugere — derfor en kraftig bekræftelse. UUID foretrækkes
// som mål (virker også for ikke-bundne databaser), med navn som fallback.
func (u *ui) deleteDatabaseServer(db database) {
	msg := fmt.Sprintf(L.ConfirmDeleteServer, db.Name)
	dialog.ShowConfirm(L.DeleteOnServer, msg, func(ok bool) {
		if !ok {
			return
		}
		target := db.ID
		if target == "" {
			target = db.Name
		}
		u.async(func() any {
			ctx, cancel := withTimeout(60 * time.Second)
			defer cancel()
			return u.c.deleteDatabase(ctx, target)
		}, u.afterDBOp)
	}, u.win)
}

// devicesTab viser kontoens tilmeldte enheder som en liste (kontobredt — enheder
// hører til kontoen, ikke til en enkelt database). Hver enhed har inline-ikoner
// til fjern og info.
func (u *ui) devicesTab() fyne.CanvasObject {
	u.devInfo = widget.NewLabel("")
	u.devHint = widget.NewLabel(" ")
	u.devBox = container.NewVBox()

	add := widget.NewButtonWithIcon(L.AddDevice, theme.ContentAddIcon(), func() { u.showAddDeviceDialog() })
	add.Importance = widget.HighImportance
	refresh := widget.NewButton(L.Refresh, func() { u.refreshDevices() })
	toolbar := container.NewHBox(add, layout.NewSpacer(), refresh)
	top := container.NewVBox(toolbar, u.devInfo)

	return container.NewBorder(top, u.devHint, nil, nil, container.NewVScroll(u.devBox))
}

// setDevHint opdaterer hover-hint-linjen nederst på enheds-fanen.
func (u *ui) setDevHint(s string) {
	if u.devHint == nil {
		return
	}
	if s == "" {
		u.devHint.SetText(" ")
		return
	}
	u.devHint.SetText("➤  " + s)
}

// deviceRow bygger én enheds-linje: markør (● for den aktuelle enhed) + navn +
// "sidst set" (pænt formateret), og ikon-knapper til fjern og info.
func (u *ui) deviceRow(d device) fyne.CanvasObject {
	marker := "   "
	if d.Current {
		marker = "●"
	}
	markerLbl := widget.NewLabel(marker)
	nameLbl := widget.NewLabelWithStyle(d.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	nameLbl.Truncation = fyne.TextTruncateEllipsis
	seenLbl := widget.NewLabel(L.ColLastSeen + ": " + prettyTime(d.LastSeen))

	// Faste kolonnebredder, så navn og dato flugter på tværs af rækkerne.
	cols := container.New(&columnsLayout{widths: []float32{26, 230}}, markerLbl, nameLbl, seenLbl)

	info := newHintIconButton(L.DeviceInfoTitle, theme.InfoIcon(), func() { u.showDeviceInfo(d) }, u.setDevHint)
	remove := newHintIconButton(L.RemoveDevice, theme.CancelIcon(), func() { u.removeDevice(d) }, u.setDevHint)
	actions := container.NewHBox(remove, info)

	header := container.NewBorder(nil, nil, cols, actions, nil)
	return container.NewVBox(header, widget.NewSeparator())
}

// showFormDialog viser en form-dialog der er markant bredere end Fynes default
// (som ellers krymper til indholdets bredde og bliver for smal til at indtaste
// stier, tokens og navne). Bredden er fast; højden skaleres med antal felter.
func (u *ui) showFormDialog(title, confirm string, items []*widget.FormItem, cb func(bool)) {
	d := dialog.NewForm(title, confirm, L.Cancel, items, cb, u.win)
	d.Resize(fyne.NewSize(620, float32(200+len(items)*48)))
	d.Show()
}

// showAddDeviceDialog (A) genererer en enrollment-token til en ny enhed via
// `admin user-enrollment <bruger> --admin-token <token>`. Enheder self-enroller,
// så GUI'en kan ikke tilmelde en fjern-maskine direkte — den udsteder en token
// som så bruges med `enroll` PÅ den nye enhed. Kræver en admin-token.
func (u *ui) showAddDeviceDialog() {
	username := widget.NewEntry()
	username.SetText(u.currentUser)
	username.SetPlaceHolder(L.Username)
	adminTok := widget.NewPasswordEntry()
	adminTok.SetPlaceHolder(L.AdminToken)

	items := []*widget.FormItem{
		widget.NewFormItem(L.Username, username),
		widget.NewFormItem(L.AdminToken, adminTok),
	}
	u.showFormDialog(L.AddDevice, L.AddDeviceCreate, items, func(ok bool) {
		if !ok {
			return
		}
		if username.Text == "" || adminTok.Text == "" {
			dialog.ShowError(errSimple(L.Username+" / "+L.AdminToken), u.win)
			return
		}
		u.async(func() any {
			ctx, cancel := withTimeout(30 * time.Second)
			defer cancel()
			return u.c.run(ctx, "", "admin", "user-enrollment", username.Text, "--admin-token", adminTok.Text)
		}, func(v any) {
			r := v.(result)
			u.log(r.Combined())
			if r.Err != nil {
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			u.showEnrollTokenResult(r.Stdout)
		})
	})
}

// showEnrollTokenResult viser den genererede enrollment-token i et felt man kan
// kopiere fra.
func (u *ui) showEnrollTokenResult(output string) {
	out := widget.NewMultiLineEntry()
	out.SetText(output)
	out.Wrapping = fyne.TextWrapWord
	d := dialog.NewCustom(L.EnrollTokenCreated, L.Close, container.NewVScroll(out), u.win)
	d.Resize(fyne.NewSize(560, 320))
	d.Show()
}

// showDeviceInfo viser alle data om enheden — navn, ID, tilmeldt og sidst set —
// med mulighed for at kopiere ID'et til udklipsholderen.
func (u *ui) showDeviceInfo(d device) {
	idEntry := widget.NewEntry()
	idEntry.SetText(d.ID)
	copyBtn := widget.NewButtonWithIcon(L.Copy, theme.ContentCopyIcon(), func() {
		u.fApp.Clipboard().SetContent(d.ID)
	})
	idRow := container.NewBorder(nil, nil, nil, copyBtn, idEntry)

	current := "—"
	if d.Current {
		current = L.ThisDevice
	}
	form := widget.NewForm(
		widget.NewFormItem(L.ColName, widget.NewLabel(d.Name)),
		widget.NewFormItem(L.ColID, idRow),
		widget.NewFormItem(L.ColStatus, widget.NewLabel(current)),
		widget.NewFormItem(L.ColEnrolled, widget.NewLabel(prettyTime(d.Enrolled))),
		widget.NewFormItem(L.ColLastSeen, widget.NewLabel(prettyTime(d.LastSeen))),
	)
	dlg := dialog.NewCustom(L.DeviceInfoTitle, L.Close, container.NewPadded(form), u.win)
	dlg.Resize(fyne.NewSize(600, 300))
	dlg.Show()
}

// removeDevice tilbagekalder en enhed via `devices remove <id>`. Den aktuelle
// enhed kan ikke fjernes herfra (det ville gøre den lokale token ugyldig).
func (u *ui) removeDevice(d device) {
	if d.Current {
		dialog.ShowInformation(L.RemoveDevice, L.CannotRemoveCurrent, u.win)
		return
	}
	msg := fmt.Sprintf(L.ConfirmRemoveDevice, d.Name)
	dialog.ShowConfirm(L.RemoveDevice, msg, func(ok bool) {
		if !ok {
			return
		}
		id := d.ID
		u.async(func() any {
			ctx, cancel := withTimeout(30 * time.Second)
			defer cancel()
			return u.c.run(ctx, "", "devices", "remove", id)
		}, func(v any) {
			r := v.(result)
			u.log(r.Combined())
			if r.Err != nil {
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			u.refreshDevices()
		})
	}, u.win)
}

// refreshDevices henter `devices` og genopbygger enheds-listen.
func (u *ui) refreshDevices() {
	if u.devBox == nil {
		return
	}
	u.devInfo.SetText(L.Working)
	u.async(func() any {
		ctx, cancel := withTimeout(30 * time.Second)
		defer cancel()
		ds, r := u.c.devices(ctx)
		return devResult{ds: ds, r: r}
	}, func(v any) {
		res := v.(devResult)
		u.devBox.RemoveAll()
		if res.r.Err != nil {
			u.log(res.r.Combined())
			u.devInfo.SetText(L.Error + ": " + describeErr(res.r))
			u.devBox.Add(widget.NewLabel(L.Error + ": " + describeErr(res.r)))
			u.devBox.Refresh()
			return
		}
		u.devInfo.SetText(fmt.Sprintf(L.DevCount, len(res.ds)))
		u.log(fmt.Sprintf(L.DevCount, len(res.ds)))
		for _, d := range res.ds {
			u.devBox.Add(u.deviceRow(d))
		}
		u.devBox.Refresh()
	})
}

// devResult bærer enhedslisten tilbage fra goroutinen.
type devResult struct {
	ds []device
	r  result
}

// activityTab viser CLI-output-loggen. Log-feltet fylder hele fanen (et
// multiline-Entry har sin egen scrollbar), med en header og en Ryd-knap.
func (u *ui) activityTab() fyne.CanvasObject {
	clear := widget.NewButton(L.Clear, func() {
		u.activity.SetText("")
		u.activity.Refresh()
	})
	header := container.NewBorder(nil, nil, widget.NewLabel(L.ActivityHint), clear, nil)
	return container.NewBorder(header, nil, nil, nil, u.activity)
}

// logResult bærer log-listen tilbage fra goroutinen.
type logResult struct {
	es []logEntry
	r  result
}

// logTab viser serverens audit-log (`keepass-deltasync log`). I modsætning til
// Aktivitet-fanen — der kun spejler CLI-output siden app-start — er dette den
// VEDVARENDE historik fra serveren (op til 30 dage, på tværs af enheder). En
// periode-vælger styrer --since; Opdatér henter på ny.
func (u *ui) logTab() fyne.CanvasObject {
	u.logInfo = widget.NewLabel("")
	u.logBox = container.NewVBox()

	// Vis-tekst → Go-duration til `log --since`. Tom = intet filter (alle).
	sinceFor := map[string]string{
		L.LogPeriod24h: "24h",
		L.LogPeriod7d:  "168h",
		L.LogPeriod30d: "720h",
		L.LogPeriodAll: "",
	}
	sel := widget.NewSelect(
		[]string{L.LogPeriod24h, L.LogPeriod7d, L.LogPeriod30d, L.LogPeriodAll},
		func(s string) {
			u.logSince = sinceFor[s]
			u.refreshLog()
		},
	)
	sel.SetSelected(L.LogPeriod7d) // udløser første hentning

	refresh := widget.NewButton(L.Refresh, func() { u.refreshLog() })
	toolbar := container.NewHBox(widget.NewLabel(L.LogPeriodLabel), sel, layout.NewSpacer(), refresh)
	top := container.NewVBox(toolbar, widget.NewLabel(L.LogHint), u.logInfo)

	return container.NewBorder(top, nil, nil, nil, container.NewVScroll(u.logBox))
}

// logRow bygger én audit-linje: OK/fejl-markør + tidspunkt + event (fremhævet) +
// niveau + IP, med faste kolonnebredder så felterne flugter på tværs af rækker.
// Yderst til højre en info-knap der åbner en popup med alle felter i fuld
// længde — så intet er skjult selvom en kolonne afkortes.
func (u *ui) logRow(e logEntry) fyne.CanvasObject {
	mark := "✓"
	if !e.OK {
		mark = "✗"
	}
	markLbl := widget.NewLabel(mark)
	timeLbl := widget.NewLabel(prettyTime(e.Time))
	eventLbl := widget.NewLabelWithStyle(e.Event, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	eventLbl.Truncation = fyne.TextTruncateEllipsis
	levelLbl := widget.NewLabel(e.Level)
	ipLbl := widget.NewLabel(e.IP) // ingen afkortning: en IP er kort og skal vises helt

	cols := container.New(&columnsLayout{widths: []float32{26, 140, 180, 70}},
		markLbl, timeLbl, eventLbl, levelLbl, ipLbl)

	info := newHintIconButton(L.LogDetailsTitle, theme.InfoIcon(), func() { u.showLogDetails(e) }, func(string) {})
	header := container.NewBorder(nil, nil, cols, info, nil)
	return container.NewVBox(header, widget.NewSeparator())
}

// showLogDetails viser alle felter for én log-post i en popup — med fuldt
// tidsstempel (sekunder), hele event-navnet og IP'en, så intet afkortes væk.
func (u *ui) showLogDetails(e logEntry) {
	status := L.LogOK
	if !e.OK {
		status = L.LogFail
	}
	ip := e.IP
	if ip == "" {
		ip = "—"
	}
	form := widget.NewForm(
		widget.NewFormItem(L.LogColTime, widget.NewLabel(prettyTimeSec(e.Time))),
		widget.NewFormItem(L.LogColEvent, widget.NewLabel(e.Event)),
		widget.NewFormItem(L.ColStatus, widget.NewLabel(status)),
		widget.NewFormItem(L.LogColLevel, widget.NewLabel(e.Level)),
		widget.NewFormItem(L.LogColIP, widget.NewLabel(ip)),
	)
	dlg := dialog.NewCustom(L.LogDetailsTitle, L.Close, container.NewPadded(form), u.win)
	dlg.Resize(fyne.NewSize(520, 280))
	dlg.Show()
}

// refreshLog henter `log --since=<periode>` og genopbygger log-listen.
func (u *ui) refreshLog() {
	if u.logBox == nil {
		return
	}
	u.logInfo.SetText(L.Working)
	since := u.logSince
	u.async(func() any {
		ctx, cancel := withTimeout(30 * time.Second)
		defer cancel()
		es, r := u.c.logEntries(ctx, since, 200)
		return logResult{es: es, r: r}
	}, func(v any) {
		res := v.(logResult)
		u.logBox.RemoveAll()
		if res.r.Err != nil {
			u.log(res.r.Combined())
			u.logInfo.SetText(L.Error + ": " + describeErr(res.r))
			u.logBox.Add(widget.NewLabel(L.Error + ": " + describeErr(res.r)))
			u.logBox.Refresh()
			return
		}
		u.logInfo.SetText(fmt.Sprintf(L.LogCount, len(res.es)))
		if len(res.es) == 0 {
			u.logBox.Add(widget.NewLabel(L.LogEmpty))
		}
		for _, e := range res.es {
			u.logBox.Add(u.logRow(e))
		}
		u.logBox.Refresh()
	})
}

// settingsTab samler status, sprog, CLI-sti og nulstilling.
func (u *ui) settingsTab() fyne.CanvasObject {
	statusCard := widget.NewCard(L.StatusBox, "", u.statusLb)

	// VIGTIGT: sæt .Selected som felt FØR vi tildeler OnChanged. SetSelected()
	// ville udløse callbacken — og da callbacken genopbygger hele dashboardet,
	// gav den indledende værdisætning en uendelig løkke (dashboard byggede sig
	// selv igen og igen). Vi sætter derfor startværdien stille og reagerer kun
	// på reelle ændringer.
	langSel := widget.NewSelect([]string{"Dansk", "English"}, nil)
	if u.set.Language == string(langEN) {
		langSel.Selected = "English"
	} else {
		langSel.Selected = "Dansk"
	}
	langSel.OnChanged = func(choice string) {
		l := langDA
		if choice == "English" {
			l = langEN
		}
		if string(l) == u.set.Language {
			return // ingen reel ændring
		}
		setLang(l)
		u.set.Language = string(l)
		_ = saveSettings(u.set)
		u.showDashboard() // genopbyg med nye tekster
	}

	// Tema-vælger: System (følg OS), Lyst eller Mørkt. Samme mønster som sproget
	// — sæt .Selected stille før OnChanged, så den indledende værdi ikke udløser
	// callbacken. Temaet anvendes live (SetTheme genmaler UI'en), så her er ingen
	// genopbygning af dashboardet nødvendig.
	themeLabels := []string{L.ThemeSystem, L.ThemeLight, L.ThemeDark}
	themeFor := map[string]string{L.ThemeSystem: themeSystem, L.ThemeLight: themeLight, L.ThemeDark: themeDark}
	labelFor := map[string]string{themeSystem: L.ThemeSystem, themeLight: L.ThemeLight, themeDark: L.ThemeDark}
	themeSel := widget.NewSelect(themeLabels, nil)
	if lbl, ok := labelFor[u.set.Theme]; ok {
		themeSel.Selected = lbl
	} else {
		themeSel.Selected = L.ThemeSystem
	}
	themeSel.OnChanged = func(choice string) {
		mode := themeFor[choice]
		if mode == u.set.Theme {
			return // ingen reel ændring
		}
		u.set.Theme = mode
		_ = saveSettings(u.set)
		applyTheme(u.fApp, mode)
	}

	cliPath := widget.NewEntry()
	cliPath.SetText(u.c.path)
	cliBrowse := widget.NewButton(L.Browse, func() {
		dialog.ShowFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			defer rc.Close()
			p := uriToPath(rc.URI())
			cliPath.SetText(p)
			u.c.path = p
			u.set.CLIPath = p
			_ = saveSettings(u.set)
		}, u.win)
	})
	cliRow := container.NewBorder(nil, nil, nil, cliBrowse, cliPath)

	form := widget.NewForm(
		widget.NewFormItem(L.Language, langSel),
		widget.NewFormItem(L.ThemeLabel, themeSel),
		widget.NewFormItem(L.CLIPathLabel, cliRow),
	)

	// Hjælpe-panel: vis et wiki-agtigt felt i bunden med en beskrivelse af den
	// aktuelle fane. Sæt .Checked stille (ikke SetChecked, der ville udløse
	// OnChanged under opbygningen). Skiftet genopbygger dashboardet, så panelet
	// vises/skjules og vinduet justeres med det samme.
	helpCheck := widget.NewCheck(L.HelpPanelLabel, nil)
	helpCheck.Checked = u.set.ShowHelpPanel
	helpCheck.OnChanged = func(on bool) {
		if on == helpEnabled {
			return
		}
		helpEnabled = on
		u.set.ShowHelpPanel = on
		_ = saveSettings(u.set)
		if on {
			u.win.Resize(fyne.NewSize(820, 760))
		} else {
			u.win.Resize(fyne.NewSize(820, 560))
		}
		u.showDashboard() // genopbyg med/uden panelet
	}
	helpDesc := widget.NewLabel(L.HelpPanelDesc)
	helpDesc.Wrapping = fyne.TextWrapWord

	return container.NewVScroll(container.NewVBox(
		statusCard,
		widget.NewSeparator(),
		form,
		widget.NewSeparator(),
		helpCheck,
		helpDesc,
		widget.NewSeparator(),
		u.autostartSection(),
	))
}

// autostartSection bygger Autostart-blokken til Indstillinger: en status-linje og
// en knap der slår autostart til/fra (`keepass-deltasync daemon` ved login).
// Statussen hentes asynkront, og knappen skifter rolle efter hvad der er sat op.
func (u *ui) autostartSection() fyne.CanvasObject {
	title := widget.NewLabelWithStyle(L.AutostartTitle, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	desc := widget.NewLabel(L.AutostartDesc)
	desc.Wrapping = fyne.TextWrapWord

	if !autostartSupported() {
		return container.NewVBox(title, widget.NewLabel(L.AutostartUnsupported), desc)
	}

	info := widget.NewLabel(L.Working)
	btn := widget.NewButton("", nil)
	btn.Disable()

	var refresh func()
	apply := func(installed bool) {
		if installed {
			info.SetText(L.AutostartOn)
			btn.SetText(L.AutostartDisable)
			btn.Importance = widget.MediumImportance
			btn.OnTapped = func() { u.autostartToggle(false, refresh) }
		} else {
			info.SetText(L.AutostartOff)
			btn.SetText(L.AutostartEnable)
			btn.Importance = widget.HighImportance
			btn.OnTapped = func() { u.autostartToggle(true, refresh) }
		}
		btn.Enable()
		btn.Refresh()
	}
	refresh = func() {
		info.SetText(L.Working)
		btn.Disable()
		u.async(func() any {
			inst, _ := autostartInstalled()
			return inst
		}, func(v any) { apply(v.(bool)) })
	}
	refresh()

	return container.NewVBox(title, info, btn, desc)
}

// autostartToggle slår autostart til (enable=true) eller fra og opdaterer status.
// Kræver en kendt CLI-sti ved aktivering — den indgår i opgaven/servicen.
func (u *ui) autostartToggle(enable bool, refresh func()) {
	if enable && u.c.path == "" {
		dialog.ShowError(errSimple(L.AutostartNoCLI), u.win)
		return
	}
	path := u.c.path
	u.async(func() any {
		if enable {
			return autostartEnable(path)
		}
		return autostartDisable()
	}, func(v any) {
		if err, ok := v.(error); ok && err != nil {
			dialog.ShowError(err, u.win)
		} else if enable {
			u.log(L.AutostartEnabled)
		} else {
			u.log(L.AutostartDisabled)
		}
		refresh()
	})
}

// refreshStatus henter `status` og opdaterer status-labelen.
func (u *ui) refreshStatus() {
	u.async(func() any {
		ctx, cancel := withTimeout(30 * time.Second)
		defer cancel()
		return u.c.status(ctx)
	}, func(v any) {
		s := v.(status)
		u.currentUser = usernameOnly(s.User)
		if u.statusLb == nil {
			return
		}
		if !s.Enrolled {
			u.statusLb.SetText(L.NotEnrolled)
			return
		}
		u.statusLb.SetText(
			"Server: " + s.Server + "\n" +
				"User: " + s.User + "\n" +
				"Device: " + s.Device + "\n" +
				"Last seen: " + s.LastSeen,
		)
	})
}

// refreshDatabases henter `databases` og — for hver lokalt bundet database —
// dens medlemmer via `shares`, og genopbygger kort-listen. Medlems-opslagene
// kører sekventielt i baggrunds-goroutinen, så UI'en ikke fryser.
func (u *ui) refreshDatabases() {
	if u.dbBox == nil {
		return
	}
	u.dbInfo.SetText(L.Working)
	u.dbBox.RemoveAll()
	u.dbBox.Add(widget.NewLabel(L.Working))
	u.dbBox.Refresh()

	u.async(func() any {
		ctx, cancel := withTimeout(2 * time.Minute)
		defer cancel()
		dbs, r := u.c.databases(ctx)
		debugf("databases: n=%d err=%v", len(dbs), r.Err)
		if r.Err != nil {
			return dbResult{r: r}
		}
		rows := make([]dbWithMembers, 0, len(dbs))
		for _, db := range dbs {
			dm := dbWithMembers{db: db}
			if db.Bound {
				ms, mr := u.c.shares(ctx, db.Name)
				if mr.Err != nil {
					dm.memErr = describeErr(mr)
				} else {
					dm.members = ms
				}
			}
			rows = append(rows, dm)
		}
		return dbResult{rows: rows, r: r}
	}, func(v any) {
		res := v.(dbResult)
		debugf("databases done: n=%d", len(res.rows))
		u.dbBox.RemoveAll()
		if res.r.Err != nil {
			u.log(res.r.Combined())
			u.dbInfo.SetText(L.Error + ": " + describeErr(res.r))
			u.dbBox.Add(widget.NewLabel(L.Error + ": " + describeErr(res.r)))
			u.dbBox.Refresh()
			return
		}
		if len(res.rows) == 0 {
			u.dbInfo.SetText(L.NoDatabases)
			u.dbBox.Add(widget.NewLabel(L.NoDatabases))
			u.dbBox.Refresh()
			return
		}
		u.dbInfo.SetText(fmt.Sprintf(L.DBCount, len(res.rows)))
		u.log(fmt.Sprintf(L.DBCount, len(res.rows)))
		for _, dm := range res.rows {
			u.dbBox.Add(u.dbCard(dm.db, dm.members, dm.memErr))
		}
		u.dbBox.Refresh()
	})
}

// dbWithMembers samler en database med dens medlemmer (eller en fejltekst hvis
// medlems-opslaget fejlede, fx fordi man ikke er ejer).
type dbWithMembers struct {
	db      database
	members []member
	memErr  string
}

// dbResult bærer de samlede rækker tilbage fra goroutinen.
type dbResult struct {
	rows []dbWithMembers
	r    result
}

// promptSync beder om masterpassword og kører derefter sync for databasen.
func (u *ui) promptSync(name string) {
	pw := widget.NewPasswordEntry()
	items := []*widget.FormItem{
		widget.NewFormItem(L.MasterPwd, pw),
	}
	u.showFormDialog(L.MasterPwdFor+" "+name, L.Sync, items, func(ok bool) {
		if !ok || pw.Text == "" {
			return
		}
		u.runSync(name, pw.Text)
	})
}

// runSync udfører selve sync-kaldet og logger resultatet.
func (u *ui) runSync(name, masterPassword string) {
	u.log("sync " + name + " …")
	u.async(func() any {
		ctx, cancel := withTimeout(10 * time.Minute)
		defer cancel()
		return u.c.sync(ctx, name, masterPassword)
	}, func(v any) {
		r := v.(result)
		u.log(r.Combined())
		if r.Err != nil {
			dialog.ShowError(errSimple(describeErr(r)), u.win)
			return
		}
		u.refreshDatabases()
	})
}

// syncAll spørger om ét masterpassword og synkroniserer alle bundne databaser.
// Forudsætter at de deler password — ellers kan brugeren synkronisere én ad
// gangen via rækkens egen knap.
func (u *ui) syncAll() {
	pw := widget.NewPasswordEntry()
	items := []*widget.FormItem{widget.NewFormItem(L.MasterPwd, pw)}
	u.showFormDialog(L.SyncAll, L.Sync, items, func(ok bool) {
		if !ok || pw.Text == "" {
			return
		}
		u.async(func() any {
			ctx, cancel := withTimeout(30 * time.Second)
			defer cancel()
			dbs, _ := u.c.databases(ctx)
			return dbs
		}, func(v any) {
			for _, db := range v.([]database) {
				if db.Bound {
					u.runSync(db.Name, pw.Text)
				}
			}
		})
	})
}

// showAddDBDialog registrerer en ny lokal database fra dashboardet.
func (u *ui) showAddDBDialog() {
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

	items := []*widget.FormItem{
		widget.NewFormItem(L.DBName, name),
		widget.NewFormItem(L.KdbxFile, pathRow),
	}
	u.showFormDialog(L.AddDatabase, L.CreateDBButton, items, func(ok bool) {
		if !ok || name.Text == "" || path.Text == "" {
			return
		}
		u.async(func() any {
			ctx, cancel := withTimeout(60 * time.Second)
			defer cancel()
			return u.c.initDB(ctx, name.Text, path.Text)
		}, func(v any) {
			r := v.(result)
			u.log(r.Combined())
			if r.Err != nil {
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			u.refreshDatabases()
		})
	})
}
