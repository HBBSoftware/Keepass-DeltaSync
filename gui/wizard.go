// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// showWizard er onboarding-guiden for en bruger der endnu ikke har tilmeldt sin
// enhed. Den fører gennem to trin: (1) tilmeld enhed med server-adresse + token,
// (2) tilføj den første lokale database. Begge trin udfører arbejdet ved at
// kalde CLI'en.

// showWizard viser velkomstskærmen.
func (u *ui) showWizard() {
	title := widget.NewLabelWithStyle(L.WizardWelcomeTitle, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	body := widget.NewLabel(L.WizardWelcomeBody)
	body.Wrapping = fyne.TextWrapWord

	start := widget.NewButton(L.WizardStart, func() { u.showEnrollStep() })
	start.Importance = widget.HighImportance

	// Sekundær vej for administratorer: udsted token + tilmeld PC'en i ét hug.
	advanced := widget.NewButton(L.WizardAdvanced, func() { u.showAdvancedEnroll() })
	advanced.Importance = widget.LowImportance

	card := container.NewVBox(
		title,
		widget.NewSeparator(),
		body,
		layout.NewSpacer(),
		container.NewHBox(layout.NewSpacer(), start, layout.NewSpacer()),
		container.NewHBox(layout.NewSpacer(), advanced, layout.NewSpacer()),
	)
	u.win.SetContent(container.NewPadded(card))
}

// showAdvancedEnroll er en alternativ trin 1 for administratorer: i stedet for at
// indtaste et færdigt enrollment-token udfyldes server-adresse + admin-token +
// brugernavn, hvorefter GUI'en bag kulisserne (1) udsteder et enrollment-token
// via `admin user-create`/`user-enrollment` og (2) kører `enroll` med det. Bagefter
// fortsætter den til trin 2 (tilføj database) som det normale flow.
func (u *ui) showAdvancedEnroll() {
	server := widget.NewEntry()
	server.SetPlaceHolder(L.ServerURLHint)
	adminTok := widget.NewPasswordEntry()
	adminTok.SetPlaceHolder(L.AdminToken)
	adminTok.SetText(u.adminToken) // genbrug et token der allerede ligger i hukommelsen
	username := widget.NewEntry()
	username.SetPlaceHolder(L.Username)
	display := widget.NewEntry()
	display.SetPlaceHolder(L.AdminDisplayName)
	device := widget.NewEntry()
	device.SetPlaceHolder(L.DeviceNameHint)

	// Visningsnavn er kun relevant når vi opretter en ny bruger.
	mode := widget.NewRadioGroup([]string{L.AdvExistingUser, L.AdvNewUser}, nil)
	mode.SetSelected(L.AdvExistingUser)
	syncDisplay := func(sel string) {
		if sel == L.AdvNewUser {
			display.Enable()
		} else {
			display.Disable()
		}
	}
	mode.OnChanged = syncDisplay
	syncDisplay(mode.Selected)

	intro := widget.NewLabel(L.AdvancedIntro)
	intro.Wrapping = fyne.TextWrapWord

	form := widget.NewForm(
		widget.NewFormItem(L.ServerURL, server),
		widget.NewFormItem(L.AdminToken, adminTok),
		widget.NewFormItem(L.AdvUserMode, mode),
		widget.NewFormItem(L.Username, username),
		widget.NewFormItem(L.AdminDisplayName, display),
		widget.NewFormItem(L.DeviceName, device),
	)

	submit := widget.NewButton(L.AdvEnrollButton, func() {
		if server.Text == "" || adminTok.Text == "" || username.Text == "" {
			dialog.ShowError(errSimple(L.ServerURL+" / "+L.AdminToken+" / "+L.Username), u.win)
			return
		}
		u.adminToken = adminTok.Text // husk tokenet som Administration-fanen gør
		srv, tok, usr := server.Text, adminTok.Text, username.Text
		disp, dev := display.Text, device.Text
		newUser := mode.Selected == L.AdvNewUser

		// Trin A: udsted enrollment-token via admin-kommandoen.
		u.win.SetContent(centeredSpinner(L.AdvIssuingToken))
		u.async(func() any {
			ctx, cancel := withTimeout(60 * time.Second)
			defer cancel()
			if newUser {
				return u.c.adminUserCreate(ctx, srv, tok, usr, disp)
			}
			return u.c.adminUserEnrollment(ctx, srv, tok, usr)
		}, func(v any) {
			r := v.(result)
			u.log(r.Combined())
			if r.Err != nil {
				u.showAdvancedEnroll()
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			enrollTok := parseEnrollToken(r.Stdout)
			if enrollTok == "" {
				u.showAdvancedEnroll()
				dialog.ShowError(errSimple(L.AdvNoTokenErr), u.win)
				return
			}
			// Trin B: tilmeld denne PC med det netop udstedte token.
			u.win.SetContent(centeredSpinner(L.AdvEnrolling))
			u.async(func() any {
				ctx, cancel := withTimeout(60 * time.Second)
				defer cancel()
				return u.c.enroll(ctx, srv, dev, enrollTok)
			}, func(v2 any) {
				er := v2.(result)
				u.log(er.Combined())
				if er.Err != nil {
					u.showAdvancedEnroll()
					dialog.ShowError(errSimple(describeErr(er)), u.win)
					return
				}
				u.showAddDBStep()
			})
		})
	})
	submit.Importance = widget.HighImportance

	header := widget.NewLabelWithStyle(L.WizardStepAdvanced, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	back := widget.NewButton(L.Back, func() { u.showWizard() })

	content := container.NewVBox(
		header,
		widget.NewSeparator(),
		intro,
		form,
		layout.NewSpacer(),
		container.NewHBox(back, layout.NewSpacer(), submit),
	)
	u.win.SetContent(container.NewPadded(content))
}

// showEnrollStep er trin 1: tilmeld enheden hos serveren.
func (u *ui) showEnrollStep() {
	server := widget.NewEntry()
	server.SetPlaceHolder(L.ServerURLHint)
	token := widget.NewEntry()
	token.SetPlaceHolder(L.EnrollTokenHint)
	device := widget.NewEntry()
	device.SetPlaceHolder(L.DeviceNameHint)

	form := widget.NewForm(
		widget.NewFormItem(L.ServerURL, server),
		widget.NewFormItem(L.EnrollToken, token),
		widget.NewFormItem(L.DeviceName, device),
	)

	submit := widget.NewButton(L.EnrollButton, func() {
		if server.Text == "" || token.Text == "" {
			dialog.ShowError(errSimple(L.ServerURL+" / "+L.EnrollToken), u.win)
			return
		}
		u.win.SetContent(centeredSpinner(L.Working))
		u.async(func() any {
			ctx, cancel := withTimeout(60 * time.Second)
			defer cancel()
			return u.c.enroll(ctx, server.Text, device.Text, token.Text)
		}, func(v any) {
			r := v.(result)
			u.log(r.Combined())
			if r.Err != nil {
				u.showEnrollStep()
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			u.showAddDBStep()
		})
	})
	submit.Importance = widget.HighImportance

	header := widget.NewLabelWithStyle(L.WizardStepEnroll, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	back := widget.NewButton(L.Back, func() { u.showWizard() })

	content := container.NewVBox(
		header,
		widget.NewSeparator(),
		form,
		layout.NewSpacer(),
		container.NewHBox(back, layout.NewSpacer(), submit),
	)
	u.win.SetContent(container.NewPadded(content))
}

// showAddDBStep er trin 2: registrér den første lokale .kdbx-fil.
func (u *ui) showAddDBStep() {
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

	body := widget.NewLabel(L.WizardAddDBBody)
	body.Wrapping = fyne.TextWrapWord

	form := widget.NewForm(
		widget.NewFormItem(L.DBName, name),
		widget.NewFormItem(L.KdbxFile, pathRow),
	)

	create := widget.NewButton(L.CreateDBButton, func() {
		if name.Text == "" || path.Text == "" {
			dialog.ShowError(errSimple(L.DBName+" / "+L.KdbxFile), u.win)
			return
		}
		u.win.SetContent(centeredSpinner(L.Working))
		u.async(func() any {
			ctx, cancel := withTimeout(60 * time.Second)
			defer cancel()
			return u.c.initDB(ctx, name.Text, path.Text)
		}, func(v any) {
			r := v.(result)
			u.log(r.Combined())
			if r.Err != nil {
				u.showAddDBStep()
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			u.showWizardDone()
		})
	})
	create.Importance = widget.HighImportance

	skip := widget.NewButton(L.SkipForNow, func() { u.showDashboard() })

	header := widget.NewLabelWithStyle(L.WizardStepDB, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	content := container.NewVBox(
		header,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(L.WizardAddDB, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		body,
		form,
		layout.NewSpacer(),
		container.NewHBox(skip, layout.NewSpacer(), create),
	)
	u.win.SetContent(container.NewPadded(content))
}

// showWizardDone er afslutningsskærmen.
func (u *ui) showWizardDone() {
	title := widget.NewLabelWithStyle(L.WizardDone, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	body := widget.NewLabel(L.WizardDoneBody)
	body.Wrapping = fyne.TextWrapWord
	finish := widget.NewButton(L.Finish, func() { u.showDashboard() })
	finish.Importance = widget.HighImportance

	content := container.NewVBox(
		title,
		widget.NewSeparator(),
		body,
		layout.NewSpacer(),
		container.NewHBox(layout.NewSpacer(), finish, layout.NewSpacer()),
	)
	u.win.SetContent(container.NewPadded(content))
}
