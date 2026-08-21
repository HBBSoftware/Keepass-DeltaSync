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

func launcherScript(exe string) string {
	return "#!/bin/sh\nexec \"" + exe + "\" browser-host \"$@\"\n"
}

// hostTargets returnerer ét mål per Firefox-variant vi kender.
//
// Det er DEN vigtigste forskel på Linux og de to andre platforme: der findes
// ikke én Firefox. En pakke fra distroen læser ~/.mozilla, en snap læser sin
// egen SNAP_USER_COMMON, og en flatpak læser sin egen ~/.var/app-mappe.
// Skriver vi kun det første sted — som vi gjorde indtil nu — melder
// `install-browser-host` succes, mens Firefox aldrig ser manifestet. Brugeren
// står så med en kommando der siger det lykkedes og en udvidelse der siger
// "cannot start the native host", uden noget at forbinde de to med.
//
// Alle tre mål returneres altid; Detected afgør om der bliver skrevet til dem
// uden --all. Det er det listen er bygget til — dry-run markerer de ikke-fundne
// med "- ", og uninstall rydder op efter en variant der er afinstalleret siden
// registreringen, hvilket kun kan lade sig gøre hvis målet stadig er med.
//
// Systemvarianten er derudover altid Detected: den koster to filer, og en
// Firefox installeret efter at hosten blev registreret skal også kunne finde
// den.
func hostTargets(exe string) ([]hostTarget, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dataDir, err := hostDataDir()
	if err != nil {
		return nil, err
	}

	targets := []hostTarget{{
		Label:    "Firefox (system)",
		Manifest: filepath.Join(home, ".mozilla", "native-messaging-hosts", hostName+".json"),
		Launcher: filepath.Join(dataDir, launcherFileName),
		Script:   launcherScript(exe),
		Detected: true,
	}}

	// Snap. Launcheren skal ligge et sted snap'ens egen AppArmor-profil kan
	// læse og eksekvere: dens SNAP_USER_COMMON. Binæren udenfor er en anden
	// sag — home-interfacet giver ikke adgang til punktum-mapper i $HOME, så
	// en binær i ~/.local/bin er uden for rækkevidde. Derfor hint'et.
	snapRoot := filepath.Join(home, "snap", "firefox")
	targets = append(targets, hostTarget{
		Label:    "Firefox (snap)",
		Manifest: filepath.Join(snapRoot, "common", ".mozilla", "native-messaging-hosts", hostName+".json"),
		Launcher: filepath.Join(snapRoot, "common", "keepass-deltasync", launcherFileName),
		Script:   launcherScript(exe),
		Detected: snapFirefoxInstalled(),
		Hint: "snap-confined Firefox can only reach files outside dot-directories in your\n" +
			"    home. If the extension still cannot start the host, move the binary to\n" +
			"    a plain directory (~/bin works) and run this command again.",
	})

	// Flatpak. Her rækker det ikke at ramme den rigtige mappe: processen
	// starter INDE i sandkassen og kan ikke eksekvere en binær på værten. Den
	// vej går gennem flatpak-spawn, som til gengæld kræver at brugeren giver
	// Firefox lov til at tale med portalen.
	flatpakRoot := filepath.Join(home, ".var", "app", "org.mozilla.firefox")
	targets = append(targets, hostTarget{
		Label:    "Firefox (flatpak)",
		Manifest: filepath.Join(flatpakRoot, ".mozilla", "native-messaging-hosts", hostName+".json"),
		Launcher: filepath.Join(flatpakRoot, "data", "keepass-deltasync", launcherFileName),
		Script:   "#!/bin/sh\nexec flatpak-spawn --host \"" + exe + "\" browser-host \"$@\"\n",
		Detected: flatpakFirefoxInstalled(home),
		Hint: "flatpak Firefox runs sandboxed and needs permission to reach the host:\n" +
			"    flatpak override --user --talk-name=org.freedesktop.Flatpak org.mozilla.firefox",
	})

	return targets, nil
}

// Detektionen må IKKE se på de mapper vi selv skriver i. Både ~/.var/app/<id>
// og ~/snap/<navn> er brugerdata, og de overlever afinstallation: `flatpak
// uninstall` rører dem ikke uden --delete-data, og `snap remove` efterlader
// ~/snap/<navn>. På KDE opretter plasma-browser-integration oven i købet
// ~/.var/app/org.mozilla.firefox uopfordret, på en maskine der aldrig har haft
// en flatpak-Firefox. Så vi spurgte reelt "har der engang ligget en Firefox
// her?" og meldte derefter en variant installeret som brugeren ikke har — med
// et hint om at køre `flatpak override` for en app der ikke findes.
//
// Deploy-mappen er derimod flatpak's og snapd's egen, og forsvinder med
// pakken. Manifestet skrives fortsat i datamappen; det er kun spørgsmålet der
// flytter sig. Rammer detektionen ved siden af på en usædvanlig opsætning, er
// --all stadig vejen udenom.

// Systemstierne er variabler alene for at testene kan pege dem et andet sted
// hen; i drift ændres de ikke.
var (
	// /snap er normalt et symlink til /var/lib/snapd/snap, men ikke alle
	// distroer laver det — Arch/Manjaro og Fedora overlader det til brugeren.
	snapRoots        = []string{"/snap", "/var/lib/snapd/snap"}
	flatpakSystemDir = "/var/lib/flatpak"
)

func snapFirefoxInstalled() bool {
	for _, root := range snapRoots {
		if dirExists(filepath.Join(root, "firefox")) {
			return true
		}
	}
	return false
}

func flatpakFirefoxInstalled(home string) bool {
	userDir := filepath.Join(home, ".local", "share", "flatpak")
	if base := os.Getenv("XDG_DATA_HOME"); base != "" {
		userDir = filepath.Join(base, "flatpak")
	}
	for _, root := range []string{flatpakSystemDir, userDir} {
		if dirExists(filepath.Join(root, "app", "org.mozilla.firefox")) {
			return true
		}
	}
	return false
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// På Linux finder Firefox manifestet på stien alene — der er intet at
// registrere.
func registerManifest(hostTarget) error { return nil }

func unregisterManifest() error { return nil }

func registrationHint(hostTarget) string { return "" }
