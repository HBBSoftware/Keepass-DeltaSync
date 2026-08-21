// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows && !darwin

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHostTargets_PlainFirefox: uden snap eller flatpak på maskinen er der ét
// mål, og det peger på den klassiske ~/.mozilla-mappe.
func TestHostTargets_PlainFirefox(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")

	targets, err := hostTargets("/opt/kp/keepass-deltasync")
	if err != nil {
		t.Fatalf("hostTargets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d: %+v", len(targets), targets)
	}
	want := filepath.Join(home, ".mozilla", "native-messaging-hosts", hostName+".json")
	if targets[0].Manifest != want {
		t.Fatalf("manifest path = %q, want %q", targets[0].Manifest, want)
	}
	if !targets[0].Detected {
		t.Fatal("the system variant must always be installed for")
	}
}

// TestHostTargets_SnapAndFlatpak er hele pointen med at målene er en liste:
// en snap- eller flatpak-Firefox læser IKKE ~/.mozilla, så et manifest kun
// dér ville lade `install-browser-host` melde succes uden at virke.
func TestHostTargets_SnapAndFlatpak(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")

	mustMkdir(t, filepath.Join(home, "snap", "firefox"))
	mustMkdir(t, filepath.Join(home, ".var", "app", "org.mozilla.firefox"))

	targets, err := hostTargets("/opt/kp/keepass-deltasync")
	if err != nil {
		t.Fatalf("hostTargets: %v", err)
	}
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d: %+v", len(targets), targets)
	}

	byLabel := make(map[string]hostTarget, len(targets))
	seen := make(map[string]bool, len(targets)*2)
	for _, tgt := range targets {
		byLabel[tgt.Label] = tgt
		for _, p := range []string{tgt.Manifest, tgt.Launcher} {
			if seen[p] {
				t.Fatalf("two variants share the path %s — one would overwrite the other", p)
			}
			seen[p] = true
		}
	}

	snap, ok := byLabel["Firefox (snap)"]
	if !ok {
		t.Fatal("snap Firefox was not detected")
	}
	wantSnap := filepath.Join(home, "snap", "firefox", "common", ".mozilla", "native-messaging-hosts", hostName+".json")
	if snap.Manifest != wantSnap {
		t.Fatalf("snap manifest = %q, want %q", snap.Manifest, wantSnap)
	}
	// Launcheren skal ligge inde i snap'ens egen mappe — ellers kan den
	// confinede proces ikke læse den.
	if !strings.HasPrefix(snap.Launcher, filepath.Join(home, "snap", "firefox")) {
		t.Fatalf("snap launcher %q is outside the snap's own directory", snap.Launcher)
	}

	flatpak, ok := byLabel["Firefox (flatpak)"]
	if !ok {
		t.Fatal("flatpak Firefox was not detected")
	}
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

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}
