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

// hostTargets — på macOS er der kun ét sted. Firefox distribueres som .app og
// læser altid ~/Library/Application Support/Mozilla/NativeMessagingHosts.
func hostTargets(exe string) ([]hostTarget, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dataDir, err := hostDataDir()
	if err != nil {
		return nil, err
	}
	return []hostTarget{{
		Label:    "Firefox",
		Manifest: filepath.Join(home, "Library", "Application Support", "Mozilla", "NativeMessagingHosts", hostName+".json"),
		Launcher: filepath.Join(dataDir, launcherFileName),
		Script:   launcherScript(exe),
		Detected: true,
		// Gatekeeper sætter karantæne på alt hentet med en browser. Firefox
		// starter hosten uden en dialog, så en karantæneret binær dør tavst.
		Hint: "if the binary was downloaded with a browser, macOS quarantines it and\n" +
			"    Firefox cannot launch it. Clear the flag once:\n" +
			"    xattr -d com.apple.quarantine " + exe,
	}}, nil
}

// På macOS og Linux finder Firefox manifestet på stien alene — der er intet
// at registrere.
func registerManifest(hostTarget) error { return nil }

func unregisterManifest() error { return nil }

func registrationHint(hostTarget) string { return "" }
