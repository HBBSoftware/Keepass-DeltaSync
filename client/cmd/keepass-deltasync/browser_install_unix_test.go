// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows && !darwin

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolate giver testen et tomt HOME og peger systemstierne ind i det, så
// hverken snapd eller flatpak på testmaskinen kan påvirke resultatet.
func isolate(t *testing.T) (home, sysRoot string) {
	t.Helper()
	home = t.TempDir()
	sysRoot = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	// Uden denne ville Chromium-målene blive slået op i testmaskinens egen
	// konfiguration, og testen ville sige noget forskelligt alt efter om der
	// tilfældigvis er en Chrome installeret.
	t.Setenv("XDG_CONFIG_HOME", "")

	oldSnap, oldFlatpak := snapRoots, flatpakSystemDir
	snapRoots = []string{filepath.Join(sysRoot, "snap")}
	flatpakSystemDir = filepath.Join(sysRoot, "flatpak")
	t.Cleanup(func() { snapRoots, flatpakSystemDir = oldSnap, oldFlatpak })
	return home, sysRoot
}

// wantLabels er hele listen af mål, i den rækkefølge hostTargets bygger den.
// Alle skal altid være med — uninstall rydder op efter en browser der er
// afinstalleret siden registreringen, og det kan kun lade sig gøre hvis målet
// stadig står på listen.
var wantLabels = []string{
	"Firefox (system)", "Firefox (snap)", "Firefox (flatpak)",
	"Chrome", "Chromium", "Edge", "Brave", "Vivaldi", "Opera",
}

func byLabel(t *testing.T, targets []hostTarget) map[string]hostTarget {
	t.Helper()
	if len(targets) != len(wantLabels) {
		t.Fatalf("expected all %d variants in the list, got %d: %+v", len(wantLabels), len(targets), targets)
	}
	m := make(map[string]hostTarget, len(targets))
	// To mål må gerne dele launcher — den er den samme uanset hvem der kalder
	// den — men aldrig manifest: Firefox og Chromium skriver hvem-må-kalde på
	// hver sin måde, så den ene ville overskrive den anden med en form
	// browseren ikke forstår.
	seen := make(map[string]string, len(targets))
	for _, tgt := range targets {
		m[tgt.Label] = tgt
		if other, dup := seen[tgt.Manifest]; dup {
			t.Fatalf("%s and %s share the manifest %s — one would overwrite the other", other, tgt.Label, tgt.Manifest)
		}
		seen[tgt.Manifest] = tgt.Label
	}
	for _, label := range wantLabels {
		if _, ok := m[label]; !ok {
			t.Fatalf("%s missing from the target list", label)
		}
	}
	return m
}

// TestHostTargets_PlainFirefox: uden snap eller flatpak på maskinen er alle tre
// mål stadig med — uninstall skal kunne rydde op efter dem — men kun
// systemvarianten er Detected, og den peger på den klassiske ~/.mozilla-mappe.
func TestHostTargets_PlainFirefox(t *testing.T) {
	home, _ := isolate(t)

	targets, err := hostTargets("/opt/kp/keepass-deltasync")
	if err != nil {
		t.Fatalf("hostTargets: %v", err)
	}
	m := byLabel(t, targets)

	sys := m["Firefox (system)"]
	want := filepath.Join(home, ".mozilla", "native-messaging-hosts", hostName+".json")
	if sys.Manifest != want {
		t.Fatalf("manifest path = %q, want %q", sys.Manifest, want)
	}
	if !sys.Detected {
		t.Fatal("the system variant must always be installed for")
	}
	for _, label := range []string{"Firefox (snap)", "Firefox (flatpak)"} {
		if m[label].Detected {
			t.Fatalf("%s reported as detected on a machine that has neither", label)
		}
	}
}

// TestHostTargets_ResidueIsNotAnInstall er fundet fra Linux-testen 2026-08-22:
// ~/.var/app/<id> og ~/snap/<navn> er brugerdata og overlever afinstallation —
// `flatpak uninstall` rører dem ikke uden --delete-data, og på KDE opretter
// plasma-browser-integration den første uopfordret. Detekterer vi på dem,
// melder `install-browser-host` en variant installeret som brugeren ikke har,
// og beder om en `flatpak override` for en app der ikke findes.
func TestHostTargets_ResidueIsNotAnInstall(t *testing.T) {
	home, _ := isolate(t)

	// Præcis den tilstand maskinen stod i: datamapperne findes, pakkerne ikke.
	mustMkdir(t, filepath.Join(home, "snap", "firefox"))
	mustMkdir(t, filepath.Join(home, ".var", "app", "org.mozilla.firefox", ".mozilla"))

	targets, err := hostTargets("/opt/kp/keepass-deltasync")
	if err != nil {
		t.Fatalf("hostTargets: %v", err)
	}
	m := byLabel(t, targets)
	for _, label := range []string{"Firefox (snap)", "Firefox (flatpak)"} {
		if m[label].Detected {
			t.Fatalf("%s detected from leftover user data alone", label)
		}
	}
}

// TestHostTargets_FlatpakUserInstall: en flatpak installeret --user ligger
// under XDG_DATA_HOME, ikke i systemmappen.
func TestHostTargets_FlatpakUserInstall(t *testing.T) {
	home, _ := isolate(t)
	dataHome := filepath.Join(home, ".local", "share")
	t.Setenv("XDG_DATA_HOME", dataHome)
	mustMkdir(t, filepath.Join(dataHome, "flatpak", "app", "org.mozilla.firefox"))

	targets, err := hostTargets("/opt/kp/keepass-deltasync")
	if err != nil {
		t.Fatalf("hostTargets: %v", err)
	}
	if !byLabel(t, targets)["Firefox (flatpak)"].Detected {
		t.Fatal("a --user flatpak install was not detected")
	}
}

// TestHostTargets_SnapAndFlatpak er hele pointen med at målene er en liste:
// en snap- eller flatpak-Firefox læser IKKE ~/.mozilla, så et manifest kun
// dér ville lade `install-browser-host` melde succes uden at virke.
func TestHostTargets_SnapAndFlatpak(t *testing.T) {
	home, sysRoot := isolate(t)

	// Pakkerne er reelt installeret: deploy-mapperne findes.
	mustMkdir(t, filepath.Join(sysRoot, "snap", "firefox"))
	mustMkdir(t, filepath.Join(sysRoot, "flatpak", "app", "org.mozilla.firefox"))

	targets, err := hostTargets("/opt/kp/keepass-deltasync")
	if err != nil {
		t.Fatalf("hostTargets: %v", err)
	}
	m := byLabel(t, targets)
	for _, label := range []string{"Firefox (snap)", "Firefox (flatpak)"} {
		if !m[label].Detected {
			t.Fatalf("%s is installed but was not detected", label)
		}
	}

	snap := m["Firefox (snap)"]
	wantSnap := filepath.Join(home, "snap", "firefox", "common", ".mozilla", "native-messaging-hosts", hostName+".json")
	if snap.Manifest != wantSnap {
		t.Fatalf("snap manifest = %q, want %q", snap.Manifest, wantSnap)
	}
	// Launcheren skal ligge inde i snap'ens egen mappe — ellers kan den
	// confinede proces ikke læse den.
	if !strings.HasPrefix(snap.Launcher, filepath.Join(home, "snap", "firefox")) {
		t.Fatalf("snap launcher %q is outside the snap's own directory", snap.Launcher)
	}

	// Manifestet skrives fortsat i datamappen, selv om detektionen nu ser på
	// deploy-mappen — det er dér Firefox læser det.
	flatpak := m["Firefox (flatpak)"]
	wantFlatpak := filepath.Join(home, ".var", "app", "org.mozilla.firefox", ".mozilla", "native-messaging-hosts", hostName+".json")
	if flatpak.Manifest != wantFlatpak {
		t.Fatalf("flatpak manifest = %q, want %q", flatpak.Manifest, wantFlatpak)
	}
	// Inde i sandkassen kan hosten ikke eksekveres direkte.
	if !strings.Contains(flatpak.Script, "flatpak-spawn --host") {
		t.Fatalf("flatpak launcher must go through flatpak-spawn, got:\n%s", flatpak.Script)
	}
	if flatpak.Hint == "" {
		t.Fatal("flatpak needs a hint about the required override — it cannot work without it")
	}
}

// TestHostTargets_Chromium: de tre Chromium-browsere findes på profilmappen,
// hver sit sted, og de er ikke "fundet" på en maskine hvor ingen af dem har
// kørt.
func TestHostTargets_Chromium(t *testing.T) {
	home, _ := isolate(t)
	mustMkdir(t, filepath.Join(home, ".config", "google-chrome"))

	targets, err := hostTargets("/opt/kp/keepass-deltasync")
	if err != nil {
		t.Fatalf("hostTargets: %v", err)
	}
	m := byLabel(t, targets)

	chrome := m["Chrome"]
	if !chrome.Detected {
		t.Fatal("a profile directory is there, but Chrome was not detected")
	}
	if !chrome.Chromium {
		t.Fatal("Chrome must be marked as Chromium — it decides the manifest format")
	}
	want := filepath.Join(home, ".config", "google-chrome", "NativeMessagingHosts", hostName+".json")
	if chrome.Manifest != want {
		t.Fatalf("Chrome manifest = %q, want %q", chrome.Manifest, want)
	}
	for _, label := range []string{"Chromium", "Edge", "Brave", "Vivaldi", "Opera"} {
		if m[label].Detected {
			t.Fatalf("%s reported as detected on a machine that never ran it", label)
		}
	}

	// Brave er den eneste hvis mappe ligger to niveauer nede. Skrives den
	// fladt ud, lander manifestet et sted Brave aldrig kigger, og kommandoen
	// melder succes alligevel.
	brave := filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser", "NativeMessagingHosts", hostName+".json")
	if m["Brave"].Manifest != brave {
		t.Fatalf("Brave manifest = %q, want %q", m["Brave"].Manifest, brave)
	}
}

// TestManifestFor_TwoFormats er hele grunden til at målene bærer et flag frem
// for at dele ét manifest: Firefox læser allowed_extensions og Chromium
// allowed_origins. Får en browser den anden forms felt, starter hosten aldrig,
// og hverken browseren eller hosten siger hvorfor.
func TestManifestFor_TwoFormats(t *testing.T) {
	gecko := manifestFor(hostTarget{Launcher: "/x/launch.sh"}, []string{"abc"})
	if len(gecko.AllowedExtensions) != 1 || gecko.AllowedExtensions[0] != browserExtensionID {
		t.Fatalf("gecko manifest must allow %s, got %+v", browserExtensionID, gecko.AllowedExtensions)
	}
	if gecko.AllowedOrigins != nil {
		t.Fatalf("gecko manifest must not carry allowed_origins, got %+v", gecko.AllowedOrigins)
	}

	chromium := manifestFor(hostTarget{Launcher: "/x/launch.sh", Chromium: true}, []string{"abc", "def"})
	want := []string{"chrome-extension://abc/", "chrome-extension://def/"}
	if len(chromium.AllowedOrigins) != len(want) {
		t.Fatalf("chromium origins = %+v, want %+v", chromium.AllowedOrigins, want)
	}
	for i, origin := range want {
		if chromium.AllowedOrigins[i] != origin {
			t.Fatalf("chromium origin %d = %q, want %q", i, chromium.AllowedOrigins[i], origin)
		}
	}
	if chromium.AllowedExtensions != nil {
		t.Fatalf("chromium manifest must not carry allowed_extensions, got %+v", chromium.AllowedExtensions)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}
