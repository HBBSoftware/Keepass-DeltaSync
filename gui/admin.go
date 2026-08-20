// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// adminResult bærer brugerlisten tilbage fra goroutinen.
type adminResult struct {
	us []adminUser
	r  result
}

// adminTab er brugeradministration oven på `admin`-subkommandoerne. Admin-tokenet
// indtastes her og holdes kun i hukommelsen (u.adminToken) — det gemmes aldrig i
// gui.json. Uden token vises blot en vejledning.
func (u *ui) adminTab() fyne.CanvasObject {
	u.adminInfo = widget.NewLabel("")
	u.adminHint = widget.NewLabel(" ")
	u.adminBox = container.NewVBox()

	tokenEntry := widget.NewPasswordEntry()
	tokenEntry.SetPlaceHolder(L.AdminToken)
	tokenEntry.SetText(u.adminToken)
	tokenEntry.OnChanged = func(s string) { u.adminToken = s }

	load := widget.NewButtonWithIcon(L.AdminLoadUsers, theme.ViewRefreshIcon(), func() { u.refreshAdminUsers() })
	load.Importance = widget.HighImportance
	tokenRow := container.NewBorder(nil, nil, widget.NewLabel(L.AdminToken), load, tokenEntry)

	create := widget.NewButtonWithIcon(L.AdminCreateUser, theme.ContentAddIcon(), func() { u.adminCreateUser() })
	sqlHelp := widget.NewButton(L.AdminTokenHelp, func() { u.adminShowTokenSQL() })
	toolbar := container.NewHBox(create, layout.NewSpacer(), sqlHelp)

	top := container.NewVBox(tokenRow, toolbar, widget.NewLabel(L.AdminHint), u.adminInfo)

	// Auto-hent hvis vi allerede har en token i hukommelsen (fx efter et sprog-
	// eller tema-skift, der genopbygger hele dashboardet).
	if strings.TrimSpace(u.adminToken) != "" {
		u.refreshAdminUsers()
	} else {
		u.adminInfo.SetText(L.AdminNeedToken)
	}

	return container.NewBorder(top, u.adminHint, nil, nil, container.NewVScroll(u.adminBox))
}

// setAdminHint opdaterer hover-hint-linjen nederst på Administration-fanen.
func (u *ui) setAdminHint(s string) {
	if u.adminHint == nil {
		return
	}
	if s == "" {
		u.adminHint.SetText(" ")
		return
	}
	u.adminHint.SetText("➤  " + s)
}

// adminUserRow bygger én bruger-linje: navn + visningsnavn + status + tæller
// (enheder/databaser) + oprettet, med ikon-knapper til enrollment-token,
// aktivér/deaktivér og slet.
func (u *ui) adminUserRow(usr adminUser) fyne.CanvasObject {
	nameLbl := widget.NewLabelWithStyle(usr.Username, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	nameLbl.Truncation = fyne.TextTruncateEllipsis
	dispLbl := widget.NewLabel(usr.Display)
	dispLbl.Truncation = fyne.TextTruncateEllipsis
	statusLbl := widget.NewLabel(usr.Status)
	countsLbl := widget.NewLabel(usr.Devices + " / " + usr.Databases)
	createdLbl := widget.NewLabel(usr.Created)
	cols := container.New(&columnsLayout{widths: []float32{150, 160, 80, 70}},
		nameLbl, dispLbl, statusLbl, countsLbl, createdLbl)

	enroll := newHintIconButton(L.AdminNewEnroll, theme.MailForwardIcon(), func() { u.adminNewEnrollment(usr) }, u.setAdminHint)
	var toggle *hintIconButton
	if usr.Disabled {
		toggle = newHintIconButton(L.AdminEnable, theme.MediaPlayIcon(), func() { u.adminToggle(usr) }, u.setAdminHint)
	} else {
		toggle = newHintIconButton(L.AdminDisable, theme.MediaPauseIcon(), func() { u.adminToggle(usr) }, u.setAdminHint)
	}
	del := newHintIconButton(L.AdminDeleteUser, theme.DeleteIcon(), func() { u.adminDeleteUser(usr) }, u.setAdminHint)
	actions := container.NewHBox(enroll, toggle, del)

	header := container.NewBorder(nil, nil, cols, actions, nil)
	return container.NewVBox(header, widget.NewSeparator())
}

// refreshAdminUsers henter `admin user-list` og genopbygger bruger-listen. Uden
// en admin-token vises blot vejledningen (intet kald).
func (u *ui) refreshAdminUsers() {
	if u.adminBox == nil {
		return
	}
	if strings.TrimSpace(u.adminToken) == "" {
		u.adminInfo.SetText(L.AdminNeedToken)
		u.adminBox.RemoveAll()
		u.adminBox.Refresh()
		return
	}
	u.adminInfo.SetText(L.Working)
	tok := u.adminToken
	u.async(func() any {
		ctx, cancel := withTimeout(30 * time.Second)
		defer cancel()
		us, r := u.c.adminUserList(ctx, tok)
		return adminResult{us: us, r: r}
	}, func(v any) {
		res := v.(adminResult)
		u.adminBox.RemoveAll()
		if res.r.Err != nil {
			u.log(res.r.Combined())
			u.adminInfo.SetText(L.Error + ": " + describeErr(res.r))
			u.adminBox.Add(widget.NewLabel(L.Error + ": " + describeErr(res.r)))
			u.adminBox.Refresh()
			return
		}
		u.adminInfo.SetText(fmt.Sprintf(L.AdminUserCount, len(res.us)))
		for _, usr := range res.us {
			u.adminBox.Add(u.adminUserRow(usr))
		}
		u.adminBox.Refresh()
	})
}

// afterAdmin er den fælles done-callback for admin-handlinger uden egen output:
// log, vis evt. fejl, og genindlæs listen.
func (u *ui) afterAdmin(v any) {
	r := v.(result)
	u.log(r.Combined())
	if r.Err != nil {
		dialog.ShowError(errSimple(describeErr(r)), u.win)
		return
	}
	u.refreshAdminUsers()
}

// adminToggle aktiverer/deaktiverer en bruger (modsat af nuværende status).
func (u *ui) adminToggle(usr adminUser) {
	tok, name, disable := u.adminToken, usr.Username, !usr.Disabled
	u.async(func() any {
		ctx, cancel := withTimeout(30 * time.Second)
		defer cancel()
		return u.c.adminSetDisabled(ctx, tok, name, disable)
	}, u.afterAdmin)
}

// adminDeleteUser sletter en bruger permanent efter en kraftig bekræftelse.
func (u *ui) adminDeleteUser(usr adminUser) {
	msg := fmt.Sprintf(L.ConfirmDeleteUser, usr.Username)
	dialog.ShowConfirm(L.AdminDeleteUser, msg, func(ok bool) {
		if !ok {
			return
		}
		tok, name := u.adminToken, usr.Username
		u.async(func() any {
			ctx, cancel := withTimeout(30 * time.Second)
			defer cancel()
			return u.c.adminUserDelete(ctx, tok, name)
		}, u.afterAdmin)
	}, u.win)
}

// adminNewEnrollment genererer en ny enrollment-token til en eksisterende bruger
// og viser den i en kopierbar dialog.
func (u *ui) adminNewEnrollment(usr adminUser) {
	tok, name := u.adminToken, usr.Username
	u.async(func() any {
		ctx, cancel := withTimeout(30 * time.Second)
		defer cancel()
		return u.c.adminUserEnrollment(ctx, "", tok, name)
	}, func(v any) {
		r := v.(result)
		u.log(r.Combined())
		if r.Err != nil {
			dialog.ShowError(errSimple(describeErr(r)), u.win)
			return
		}
		u.showOutputDialog(L.EnrollTokenCreated, r.Stdout)
	})
}

// adminCreateUser opretter en ny bruger og viser den returnerede enrollment-token.
func (u *ui) adminCreateUser() {
	if strings.TrimSpace(u.adminToken) == "" {
		dialog.ShowInformation(L.AdminCreateUser, L.AdminNeedToken, u.win)
		return
	}
	username := widget.NewEntry()
	username.SetPlaceHolder(L.Username)
	display := widget.NewEntry()
	items := []*widget.FormItem{
		widget.NewFormItem(L.Username, username),
		widget.NewFormItem(L.AdminDisplayName, display),
	}
	u.showFormDialog(L.AdminCreateUser, L.AdminCreateUser, items, func(ok bool) {
		if !ok || username.Text == "" {
			return
		}
		tok, name, disp := u.adminToken, username.Text, display.Text
		u.async(func() any {
			ctx, cancel := withTimeout(30 * time.Second)
			defer cancel()
			return u.c.adminUserCreate(ctx, "", tok, name, disp)
		}, func(v any) {
			r := v.(result)
			u.log(r.Combined())
			if r.Err != nil {
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			u.refreshAdminUsers()
			u.showOutputDialog(L.EnrollTokenCreated, r.Stdout)
		})
	})
}

// adminShowTokenSQL henter `admin token-sql` (kræver ingen token) og viser SQL'en
// i en kopierbar dialog, så man kan bootstrappe en admin-token via DBeaver.
func (u *ui) adminShowTokenSQL() {
	u.async(func() any {
		ctx, cancel := withTimeout(15 * time.Second)
		defer cancel()
		return u.c.adminTokenSQL(ctx)
	}, func(v any) {
		r := v.(result)
		if r.Err != nil {
			dialog.ShowError(errSimple(describeErr(r)), u.win)
			return
		}
		u.showOutputDialog(L.AdminTokenHelp, r.Stdout)
	})
}

// showOutputDialog viser vilkårlig tekst-output i et kopierbart, read-only felt.
func (u *ui) showOutputDialog(title, output string) {
	out := widget.NewMultiLineEntry()
	out.SetText(output)
	out.Wrapping = fyne.TextWrapWord
	d := dialog.NewCustom(title, L.Close, container.NewVScroll(out), u.win)
	d.Resize(fyne.NewSize(640, 380))
	d.Show()
}
