// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package main

import (
	"os"
	"path/filepath"
)

const launcherFileName = "browser-host.sh"

func hostDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "keepass-deltasync"), nil
}

func launcherScript(exe string) string {
	return "#!/bin/sh\nexec \"" + exe + "\" browser-host \"$@\"\n"
}

// hostTargets — på macOS har hver browser ét sted, og det ligger fast.
// Programmerne distribueres som .app og lægger deres manifest-mappe under
// ~/Library/Application Support med browserens eget navn.
//
// Alle mål deler launcher; kun manifestet er forskelligt fra browser til
// browser. Karantæne-hintet gælder dem alle, for Gatekeeper ser på binæren og
// ikke på hvem der starter den.
func hostTargets(exe string) ([]hostTarget, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dataDir, err := hostDataDir()
	if err != nil {
		return nil, err
	}
	support := filepath.Join(home, "Library", "Application Support")
	launcher := filepath.Join(dataDir, launcherFileName)

	// Gatekeeper sætter karantæne på alt hentet med en browser. En browser
	// starter hosten uden en dialog, så en karantæneret binær dør tavst.
	quarantine := "if the binary was downloaded with a browser, macOS quarantines it and\n" +
		"    the browser cannot launch it. Clear the flag once:\n" +
		"    xattr -d com.apple.quarantine " + exe

	targets := []hostTarget{{
		Label:    "Firefox",
		Manifest: filepath.Join(support, "Mozilla", "NativeMessagingHosts", hostName+".json"),
		Launcher: launcher,
		Script:   launcherScript(exe),
		Detected: true,
		Hint:     quarantine,
	}}

	// Chromium-browserne. Profilmappen er det eneste vi kan gå efter, og den
	// findes fra første gang browseren har kørt — hvilket også er første gang
	// der er en profil at registrere manifestet i.
	for _, b := range []struct{ label, dir, app string }{
		{"Chrome", filepath.Join("Google", "Chrome"), "Google Chrome.app"},
		{"Chromium", "Chromium", "Chromium.app"},
		{"Edge", filepath.Join("Microsoft Edge"), "Microsoft Edge.app"},
	} {
		root := filepath.Join(support, b.dir)
		_, appErr := os.Stat(filepath.Join("/Applications", b.app))
		info, dirErr := os.Stat(root)
		targets = append(targets, hostTarget{
			Label:    b.label,
			Manifest: filepath.Join(root, "NativeMessagingHosts", hostName+".json"),
			Launcher: launcher,
			Script:   launcherScript(exe),
			Detected: appErr == nil || (dirErr == nil && info.IsDir()),
			Chromium: true,
			Hint:     quarantine,
		})
	}

	return targets, nil
}

// På macOS og Linux finder Firefox manifestet på stien alene — der er intet
// at registrere.
func registerManifest(hostTarget) error { return nil }

func unregisterManifest(hostTarget) error { return nil }

func registrationHint(hostTarget) string { return "" }
