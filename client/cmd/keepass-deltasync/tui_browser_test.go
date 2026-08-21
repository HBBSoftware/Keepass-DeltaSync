// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rivo/tview"
)

// newTestTui bygger en TUI uden at starte app'en. tview tegner først noget
// når Run() kaldes, så menuerne kan bygges og inspiceres uden en terminal.
func newTestTui(t *testing.T) *tui {
	t.Helper()
	m, lang := messagesFor("en")
	return &tui{app: tview.NewApplication(), self: "keepass-deltasync", m: m, lang: lang}
}

// itemLabels trækker hovedteksten ud af hvert punkt i listen.
func itemLabels(list *tview.List) []string {
	out := make([]string, 0, list.GetItemCount())
	for i := 0; i < list.GetItemCount(); i++ {
		main, _ := list.GetItemText(i)
		out = append(out, main)
	}
	return out
}

// focusedList henter menuen frem. setRoot pakker den ind i en Flex sammen med
// statushovedet, men det er listen der får fokus — og den vej udenom sparer
// os for at kende layoutets form.
func focusedList(t *testing.T, app *tview.Application) *tview.List {
	t.Helper()
	list, ok := app.GetFocus().(*tview.List)
	if !ok {
		t.Fatalf("expected a list to have focus, got %T", app.GetFocus())
	}
	return list
}

// TestBrowserMenuIsReachableWithoutEnrollment er hele pointen med sektionen:
// søgning kræver ingen konto, så en uenrolleret bruger må ikke mødes af en
// menu hvor det eneste valg er at skaffe sig en.
func TestBrowserMenuIsReachableWithoutEnrollment(t *testing.T) {
	// Peg config'en på en tom mappe, så der hverken er server eller databaser.
	t.Setenv("KEEPASS_DELTASYNC_CONFIG", filepath.Join(t.TempDir(), "config.toml"))

	tu := newTestTui(t)
	tu.showMain()

	labels := itemLabels(focusedList(t, tu.app))
	if !contains(labels, tu.m.secBrowser) {
		t.Fatalf("the Firefox section is missing from the unenrolled main menu: %v", labels)
	}
}

// TestBrowserMenuItems tjekker at undermenuen kan bygges, og at den bærer de
// to kommandoer opsætningen står og falder med.
func TestBrowserMenuItems(t *testing.T) {
	t.Setenv("KEEPASS_DELTASYNC_CONFIG", filepath.Join(t.TempDir(), "config.toml"))

	tu := newTestTui(t)
	tu.showBrowserMenu()

	labels := itemLabels(focusedList(t, tu.app))
	for _, want := range []string{tu.m.miAddLocal, tu.m.miInstallHost, tu.m.miUninstallHost} {
		if !contains(labels, want) {
			t.Fatalf("browser menu is missing %q: %v", want, labels)
		}
	}
}

// TestBrowserMenuStringsAreTranslated fanger den klassiske fejl ved en ny
// sektion: feltet tilføjes til struct'en og udfyldes kun i det ene sprog, så
// menupunktet står tomt for den anden halvdel af brugerne.
func TestBrowserMenuStringsAreTranslated(t *testing.T) {
	for _, lang := range []string{"en", "da"} {
		m, _ := messagesFor(lang)
		fields := map[string]string{
			"secBrowser":       m.secBrowser,
			"secBrowserDesc":   m.secBrowserDesc,
			"browserMenuTitle": m.browserMenuTitle,
			"miAddLocal":       m.miAddLocal,
			"miAddLocalDesc":   m.miAddLocalDesc,
			"fldAddLocalName":  m.fldAddLocalName,
			"fldAddLocalPath":  m.fldAddLocalPath,
			"fldAddLocalSave":  m.fldAddLocalSave,
			"miInstallHost":    m.miInstallHost,
			"miInstallDesc":    m.miInstallDesc,
			"miUninstallHost":  m.miUninstallHost,
			"miUninstDesc":     m.miUninstDesc,
			"miProbe":          m.miProbe,
			"miProbeDesc":      m.miProbeDesc,
			"pkProbe":          m.pkProbe,
		}
		for name, value := range fields {
			if strings.TrimSpace(value) == "" {
				t.Errorf("%s is empty in %q", name, lang)
			}
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
