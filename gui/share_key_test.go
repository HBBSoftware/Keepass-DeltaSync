// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"image/png"
	"os"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestParseShareKey(t *testing.T) {
	out := "user: bob\ndevice: Bobs laptop: work\nfingerprint: 3B2B-FDCF-2696-B134-0797\nstatus: new\nfuture: ignored\n"
	k := parseShareKey(out)
	want := shareKey{User: "bob", Device: "Bobs laptop: work", Fingerprint: "3B2B-FDCF-2696-B134-0797", Status: "new"}
	if k != want {
		t.Fatalf("got %+v, want %+v", k, want)
	}
}

// Del-knappen må ikke kunne trykkes før fluebenet er sat — ellers kan man
// klikke sig forbi aftrykket uden at have kigget på det, og så beskytter
// dialogen ikke mod noget.
func TestConfirmShareKeyNeedsTheCheckbox(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	setLang(langEN)
	w := test.NewWindow(nil)
	w.Resize(fyne.NewSize(760, 520))
	u := &ui{fApp: a, win: w}

	shared := false
	u.confirmShareKey(shareKey{User: "bob", Device: "bob-laptop", Fingerprint: "3B2B-FDCF-2696-B134-0797", Status: "changed"},
		func() { shared = true })

	var shareBtn *widget.Button
	var check *widget.Check
	walk(w.Canvas().Overlays().Top(), func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *widget.Button:
			if x.Text == L.ShareDatabase {
				shareBtn = x
			}
		case *widget.Check:
			check = x
		}
	})
	if shareBtn == nil || check == nil {
		t.Fatal("dialog has no share button or no checkbox")
	}
	if !shareBtn.Disabled() {
		t.Fatal("share button is enabled before the fingerprint was confirmed")
	}
	test.Tap(shareBtn)
	if shared {
		t.Fatal("tapping the disabled button shared anyway")
	}

	// KDS_SCREENSHOT_DIR gemmer dialogen som billede til manuel kontrol.
	if dir := os.Getenv("KDS_SCREENSHOT_DIR"); dir != "" {
		f, err := os.Create(dir + "/share-key-dialog.png")
		if err == nil {
			_ = png.Encode(f, w.Canvas().Capture())
			f.Close()
		}
	}

	check.SetChecked(true)
	if shareBtn.Disabled() {
		t.Fatal("share button still disabled after the checkbox")
	}
	test.Tap(shareBtn)
	if !shared {
		t.Fatal("confirming did not share")
	}
}

// walk besøger o og alle objekter under det: containere direkte, widgets via
// deres renderer.
func walk(o fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	if o == nil {
		return
	}
	visit(o)
	switch x := o.(type) {
	case *fyne.Container:
		for _, c := range x.Objects {
			walk(c, visit)
		}
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(x).Objects() {
			walk(c, visit)
		}
	}
}
