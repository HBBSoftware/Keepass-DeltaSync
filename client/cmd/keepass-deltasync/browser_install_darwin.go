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

func nativeManifestPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "Mozilla", "NativeMessagingHosts", hostName+".json"), nil
}

func launcherScript(exe string) string {
	return "#!/bin/sh\nexec \"" + exe + "\" browser-host \"$@\"\n"
}

// På macOS og Linux finder Firefox manifestet på stien alene — der er intet
// at registrere.
func registerManifest(string) error { return nil }

func unregisterManifest() error { return nil }

func registrationHint(string) string { return "" }
