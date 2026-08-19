// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows && !darwin

package main

import (
	"os"
	"path/filepath"
)

const launcherFileName = "browser-host.sh"

// hostDataDir følger XDG. Firefox' manifest-mappe gør ikke — den ligger fast
// under ~/.mozilla uanset XDG_DATA_HOME.
func hostDataDir() (string, error) {
	if base := os.Getenv("XDG_DATA_HOME"); base != "" {
		return filepath.Join(base, "keepass-deltasync"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "keepass-deltasync"), nil
}

func nativeManifestPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".mozilla", "native-messaging-hosts", hostName+".json"), nil
}

func launcherScript(exe string) string {
	return "#!/bin/sh\nexec \"" + exe + "\" browser-host \"$@\"\n"
}

// På Linux finder Firefox manifestet på stien alene — der er intet at
// registrere.
func registerManifest(string) error { return nil }

func unregisterManifest() error { return nil }

func registrationHint(string) string { return "" }
