// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"image/color"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// async kører work() i en goroutine (så UI'en ikke fryser under et CLI-kald) og
// leverer resultatet tilbage på UI-tråden via fyne.Do, hvor done() trygt kan
// røre widgets.
func (u *ui) async(work func() any, done func(any)) {
	go func() {
		v := work()
		fyne.Do(func() { done(v) })
	}()
}

// newActivityLog laver det read-only tekstfelt der viser CLI-output. Det bruger
// readOnlyEntry (ikke Disable()), så teksten vises i fuld kontrast og er nem at
// læse, men stadig ikke kan redigeres.
func newActivityLog() *readOnlyEntry {
	return newReadOnlyEntry()
}

// log tilføjer en linje (med tidsstempel) til aktivitetsloggen.
func (u *ui) log(text string) {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return
	}
	stamp := time.Now().Format("15:04:05")
	cur := u.activity.Text
	if cur != "" {
		cur += "\n"
	}
	u.activity.SetText(cur + "[" + stamp + "] " + text)
	u.activity.CursorRow = strings.Count(u.activity.Text, "\n")
	u.activity.Refresh()
}

// centeredSpinner viser en uendelig progress-bar med en label — bruges mens
// vi venter på et CLI-kald.
func centeredSpinner(label string) fyne.CanvasObject {
	bar := widget.NewProgressBarInfinite()
	box := container.NewVBox(widget.NewLabel(label), bar)
	return container.New(layout.NewCenterLayout(), box)
}

// errSimple pakker en streng som en error til dialog.ShowError.
func errSimple(msg string) error { return errors.New(msg) }

// prettyTime parser et tidsstempel (ISO 8601 fra serveren, evt. med Z/offset) og
// formaterer det pænt i lokal tid. Kan ikke det parses, returneres originalen.
func prettyTime(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	layouts := []string{
		time.RFC3339,                       // 2026-06-16T12:36:16Z
		"2006-01-02T15:04:05.999999Z07:00", // med brøkdele
		"2006-01-02 15:04:05.999999-07",    // shares/AddedAt-format
		"2006-01-02 15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Local().Format("2006-01-02 15:04")
		}
	}
	return s
}

// prettyTimeSec er som prettyTime, men medtager sekunder — bruges i detalje-
// popups hvor det præcise tidspunkt er relevant (fx for at skelne log-poster
// inden for samme minut).
func prettyTimeSec(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.999999Z07:00",
		"2006-01-02 15:04:05.999999-07",
		"2006-01-02 15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Local().Format("2006-01-02 15:04:05")
		}
	}
	return s
}

// indented rykker et element ind med en fast venstre-margin, så medlemslinjer
// under en database fremstår visuelt underordnet den.
func indented(o fyne.CanvasObject) fyne.CanvasObject {
	pad := canvas.NewRectangle(color.Transparent)
	pad.SetMinSize(fyne.NewSize(28, 0))
	return container.NewBorder(nil, nil, pad, nil, o)
}

// topPad lægger en lille tom margin oven over et element — bruges til at give
// luft mellem fanebladene og fanens øverste knaprække.
func topPad(o fyne.CanvasObject) fyne.CanvasObject {
	pad := canvas.NewRectangle(color.Transparent)
	pad.SetMinSize(fyne.NewSize(0, 8))
	return container.NewBorder(pad, nil, nil, nil, o)
}

// uriToPath konverterer en Fyne-URI fra en fildialog til en OS-sti. På Windows
// kan Fyne give "/C:/sti" — den ledende skråstreg fjernes så Go's exec/os kan
// bruge stien.
func uriToPath(u fyne.URI) string {
	p := u.Path()
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return filepath.FromSlash(p)
}
