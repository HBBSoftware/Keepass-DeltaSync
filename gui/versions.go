// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// versionsResult bærer versionslisten tilbage fra goroutinen.
type versionsResult struct {
	vs []entryVersion
	r  result
}

// showVersionsDialog åbner en dialog hvor man kan se og gendanne tidligere
// versioner af en entry (op til 3). GUI'en kender ikke entries' indhold (den
// rører aldrig .kdbx-filen), så brugeren indsætter selv entry-UUID'et — fx
// kopieret fra KeePassXC eller fra en linje i Log-fanen.
func (u *ui) showVersionsDialog(db database) {
	uuidEntry := widget.NewEntry()
	uuidEntry.SetPlaceHolder(L.EntryUUIDHint)
	info := widget.NewLabel(L.VersionsHint)
	resultBox := container.NewVBox()

	show := widget.NewButton(L.VersionsShow, nil)
	show.OnTapped = func() {
		uuid := strings.TrimSpace(uuidEntry.Text)
		if uuid == "" {
			return
		}
		info.SetText(L.Working)
		resultBox.RemoveAll()
		resultBox.Refresh()
		name := db.Name
		u.async(func() any {
			ctx, cancel := withTimeout(30 * time.Second)
			defer cancel()
			vs, r := u.c.versions(ctx, name, uuid)
			return versionsResult{vs: vs, r: r}
		}, func(v any) {
			res := v.(versionsResult)
			resultBox.RemoveAll()
			if res.r.Err != nil {
				u.log(res.r.Combined())
				info.SetText(L.Error + ": " + describeErr(res.r))
				resultBox.Refresh()
				return
			}
			if len(res.vs) == 0 {
				info.SetText(L.VersionsNone)
				resultBox.Refresh()
				return
			}
			info.SetText(fmt.Sprintf(L.VersionsCount, len(res.vs)))
			for _, ver := range res.vs {
				resultBox.Add(u.versionRow(db, uuid, ver))
			}
			resultBox.Refresh()
		})
	}

	uuidRow := container.NewBorder(nil, nil, nil, show, uuidEntry)
	top := container.NewVBox(widget.NewLabel(L.EntryUUID), uuidRow, info)
	content := container.NewBorder(top, nil, nil, nil, container.NewVScroll(resultBox))

	d := dialog.NewCustom(fmt.Sprintf(L.VersionsTitle, db.Name), L.Close, content, u.win)
	d.Resize(fyne.NewSize(660, 460))
	d.Show()
}

// versionRow bygger én versions-linje: "v3 — current" + ændret-tidspunkt, med en
// Gendan-knap til højre.
func (u *ui) versionRow(db database, uuid string, ver entryVersion) fyne.CanvasObject {
	label := widget.NewLabel("v" + ver.Num + " — " + ver.State)
	modified := widget.NewLabel(prettyTime(ver.Modified))
	cols := container.New(&columnsLayout{widths: []float32{170}}, label, modified)
	restore := widget.NewButton(L.VersionsRestore, func() { u.restoreVersion(db, uuid, ver) })
	return container.NewBorder(nil, widget.NewSeparator(), nil, restore, cols)
}

// restoreVersion gendanner en valgt version server-side via `restore` efter en
// bekræftelse, og minder om at man skal synkronisere for at hente ændringen ned.
func (u *ui) restoreVersion(db database, uuid string, ver entryVersion) {
	msg := fmt.Sprintf(L.ConfirmRestore, ver.Num, db.Name)
	dialog.ShowConfirm(L.VersionsRestore, msg, func(ok bool) {
		if !ok {
			return
		}
		name, num := db.Name, ver.Num
		u.async(func() any {
			ctx, cancel := withTimeout(30 * time.Second)
			defer cancel()
			return u.c.restore(ctx, name, uuid, num)
		}, func(v any) {
			r := v.(result)
			u.log(r.Combined())
			if r.Err != nil {
				dialog.ShowError(errSimple(describeErr(r)), u.win)
				return
			}
			dialog.ShowInformation(L.VersionsRestore, L.RestoreDoneSync, u.win)
		})
	}, u.win)
}
