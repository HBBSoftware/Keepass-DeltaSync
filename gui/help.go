// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// helpEnabled styrer om det wiki-agtige hjælpe-panel i bunden vises. Pakke-global
// (som L for sprog), sættes ved opstart fra settings.ShowHelpPanel og kan slås
// til/fra i Indstillinger.
var helpEnabled bool

// helpTexts er den wiki-agtige hjælpetekst (markdown) for hver fane, i SAMME
// rækkefølge som fanerne oprettes i showDashboard.
func helpTexts() []string {
	return []string{
		L.HelpDatabases,
		L.HelpDevices,
		L.HelpActivity,
		L.HelpLog,
		L.HelpAdmin,
		L.HelpSettings,
	}
}

// helpTitles er fanetitlen der vises (farvet) i panel-headeren — samme tekster
// som selve fanerne, i samme rækkefølge.
func helpTitles() []string {
	return []string{L.TabDatabases, L.TabDevices, L.TabActivity, L.TabLog, L.TabAdmin, L.TabSettings}
}

// helpIcons er et repræsentativt ikon pr. fane, i samme rækkefølge.
func helpIcons() []fyne.Resource {
	return []fyne.Resource{
		theme.StorageIcon(),  // Databaser
		theme.ComputerIcon(), // Enheder
		theme.ListIcon(),     // Aktivitet
		theme.HistoryIcon(),  // Log
		theme.AccountIcon(),  // Administration
		theme.SettingsIcon(), // Indstillinger
	}
}

// buildHelpPanel laver hjælpe-panelet til bunden af vinduet: en header med et
// fane-ikon og en farvet titel, og under den et rullbart markdown-felt med fast
// højde, der opdateres når man skifter fane.
func (u *ui) buildHelpPanel() fyne.CanvasObject {
	u.helpText = widget.NewRichTextFromMarkdown("")
	u.helpText.Wrapping = fyne.TextWrapWord

	u.helpIcon = widget.NewIcon(theme.InfoIcon())
	u.helpTitle = canvas.NewText("", theme.PrimaryColor())
	u.helpTitle.TextStyle = fyne.TextStyle{Bold: true}
	u.helpTitle.TextSize = 18

	header := container.NewHBox(u.helpIcon, u.helpTitle)
	scroll := container.NewVScroll(u.helpText)
	scroll.SetMinSize(fyne.NewSize(0, 168))

	body := container.NewBorder(header, nil, nil, nil, scroll)
	// Adskil panelet fra fanerne med en streg foroven.
	return container.NewBorder(widget.NewSeparator(), nil, nil, nil, body)
}

// updateHelp sætter ikon, titel og tekst til den fane med indeks idx (fanernes
// rækkefølge i showDashboard).
func (u *ui) updateHelp(idx int) {
	if u.helpText == nil {
		return
	}
	texts, titles, icons := helpTexts(), helpTitles(), helpIcons()
	if idx < 0 || idx >= len(texts) {
		return
	}
	u.helpIcon.SetResource(icons[idx])
	u.helpTitle.Text = titles[idx]
	u.helpTitle.Refresh()

	// Titlen vises i headeren, så fjern den ledende "## Overskrift"-linje fra
	// selve teksten for at undgå dobbelt overskrift.
	md := texts[idx]
	if strings.HasPrefix(md, "## ") {
		if i := strings.Index(md, "\n"); i >= 0 {
			md = strings.TrimLeft(md[i+1:], "\n")
		}
	}
	u.helpText.ParseMarkdown(md)
	colorizeHelp(u.helpText)
}

// colorizeHelp farver alle inline-kode-segmenter (dvs. CLI-kommandoerne) med
// temaets accentfarve, så kommandoerne træder tydeligt frem i teksten.
func colorizeHelp(rt *widget.RichText) {
	for _, seg := range rt.Segments {
		ts, ok := seg.(*widget.TextSegment)
		if !ok {
			continue
		}
		if ts.Style.TextStyle.Monospace {
			ts.Style.ColorName = theme.ColorNamePrimary
		}
	}
	rt.Refresh()
}
