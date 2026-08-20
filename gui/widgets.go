// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// Fyne v2.7 har ingen indbygget tooltip. hintIconButton er en ikon-knap der i
// stedet viser sin hjælpetekst i en delt "hint"-linje, så længe musen er over
// den. setHint kaldes med teksten ved MouseIn og med "" ved MouseOut.
type hintIconButton struct {
	widget.Button
	hint    string
	setHint func(string)
}

func newHintIconButton(hint string, icon fyne.Resource, tapped func(), setHint func(string)) *hintIconButton {
	b := &hintIconButton{hint: hint, setHint: setHint}
	b.ExtendBaseWidget(b)
	b.SetIcon(icon)
	b.OnTapped = tapped
	return b
}

func (b *hintIconButton) MouseIn(e *desktop.MouseEvent) {
	b.Button.MouseIn(e) // bevar hover-highlight
	if b.setHint != nil {
		b.setHint(b.hint)
	}
}

func (b *hintIconButton) MouseOut() {
	b.Button.MouseOut()
	if b.setHint != nil {
		b.setHint("")
	}
}

// readOnlyEntry er et multiline-tekstfelt der viser tekst i FULD kontrast (i
// modsætning til et Disable()'t felt, hvis tekst tegnes nedtonet og er svær at
// læse), men som ignorerer redigering. Markering og kopiering (Ctrl+C) virker
// stadig, så aktivitetsloggen kan kopieres.
type readOnlyEntry struct {
	widget.Entry
}

func newReadOnlyEntry() *readOnlyEntry {
	e := &readOnlyEntry{}
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapWord
	e.ExtendBaseWidget(e)
	return e
}

// TypedRune ignoreres — man kan ikke skrive tegn i loggen.
func (e *readOnlyEntry) TypedRune(rune) {}

// TypedKey blokerer redigerings-taster men lader piletaster/markering passere.
func (e *readOnlyEntry) TypedKey(k *fyne.KeyEvent) {
	switch k.Name {
	case fyne.KeyBackspace, fyne.KeyDelete, fyne.KeyReturn, fyne.KeyEnter:
		return
	}
	e.Entry.TypedKey(k)
}

// columnsLayout lægger børn vandret ud med en fast bredde per kolonne, så
// indholdet (fx navn og dato) flugter på tværs af rækker — uden en egentlig
// grid-widget. Det sidste element (uden tildelt bredde) får resten af pladsen.
type columnsLayout struct {
	widths []float32
}

func (c *columnsLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for i, o := range objs {
		if i < len(c.widths) {
			w += c.widths[i]
		} else if ms := o.MinSize(); ms.Width > 0 {
			w += ms.Width
		}
		if ms := o.MinSize(); ms.Height > h {
			h = ms.Height
		}
	}
	return fyne.NewSize(w, h)
}

func (c *columnsLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	var x float32
	for i, o := range objs {
		w := o.MinSize().Width
		if i < len(c.widths) {
			w = c.widths[i]
		}
		o.Resize(fyne.NewSize(w, size.Height))
		o.Move(fyne.NewPos(x, 0))
		x += w
	}
}
