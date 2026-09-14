// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// Under Windows ligger manifestet et vilkårligt sted på disken, og Firefox
// finder det via en registry-nøgle. Vi holder både manifest og launcher i
// %LOCALAPPDATA%, så intet kræver administratorrettigheder.

const launcherFileName = "browser-host.bat"

// HKCU-nøglerne browserne slår op i. Hver browser har sin egen gren, og
// Chromium-grenene ligner hinanden nok til at de er værd at skrive ud:
// Chrome ser kun under Google, Edge kun under Microsoft.
//
// Opera har ingen gren. Deres egen dokumentation henviser til Chromes nøgle,
// så en Opera bliver betjent af Chrome-målet og har ikke sit eget.
const (
	firefoxRegistryKey  = `Software\Mozilla\NativeMessagingHosts\` + hostName
	chromeRegistryKey   = `Software\Google\Chrome\NativeMessagingHosts\` + hostName
	chromiumRegistryKey = `Software\Chromium\NativeMessagingHosts\` + hostName
	edgeRegistryKey     = `Software\Microsoft\Edge\NativeMessagingHosts\` + hostName
	braveRegistryKey    = `Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\` + hostName
	vivaldiRegistryKey  = `Software\Vivaldi\NativeMessagingHosts\` + hostName
)

func hostDataDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", errors.New("LOCALAPPDATA is not set — cannot decide where to install the browser host")
	}
	return filepath.Join(base, "keepass-deltasync"), nil
}

// launcherScript er en .bat, fordi Firefox' manifest ikke kan sende
// argumenter med til programmet — og vores host er en subkommando.
// %* videresender manifest-stien Firefox selv tilføjer.
//
// `@echo off` er IKKE kosmetik: uden den skriver cmd.exe kommandolinjen ud på
// stdout, og den tekst lander midt i den længdeprefixede beskedstrøm og
// ødelægger framingen permanent. Målt: 332 bytes output i stedet for 132.
//
// Bemærk også at cmd.exe returnerer exit 1 når pipen lukkes, selvom binæren
// selv returnerer 0. Det er en egenskab ved wrapperen, ikke en fejl hos os,
// og hverken `exit /b %errorlevel%` eller `call` ændrer det.
func launcherScript(exe string) string {
	return "@echo off\r\n" +
		"\"" + exe + "\" browser-host %*\r\n"
}

// hostTargets — på Windows er manifestets placering ligegyldig, for
// registry-nøglen peger på den. Alle tre manifester ligger derfor sammen med
// launcheren, som de deler: launcheren er den samme uanset hvem der kalder
// den, mens manifesterne skal holdes adskilt, fordi Firefox og Chromium ikke
// skriver hvem-må-kalde på samme måde.
//
// Firefox er altid Detected. Den er den browser udvidelsen har været udgivet
// til længst, og manifestet koster to filer selv hvis den ikke er der endnu.
// Chrome og Edge tælles kun med hvis de faktisk er installeret — ellers ville
// hver eneste installation efterlade registry-nøgler for browsere maskinen
// aldrig har haft.
func hostTargets(exe string) ([]hostTarget, error) {
	dir, err := hostDataDir()
	if err != nil {
		return nil, err
	}
	launcher := filepath.Join(dir, launcherFileName)

	// Opera læser Chromes nøgle, så den tæller med i om Chrome-målet skal
	// skrives — ellers ville en maskine med Opera og uden Chrome ikke få
	// noget skrevet overhovedet.
	opera := installed(operaExecutables)
	chrome := hostTarget{
		Label:       "Chrome",
		Manifest:    filepath.Join(dir, hostName+".chrome.json"),
		Launcher:    launcher,
		Script:      launcherScript(exe),
		Detected:    installed(chromeExecutables) || opera,
		Chromium:    true,
		RegistryKey: chromeRegistryKey,
	}
	if opera {
		chrome.Hint = "Opera is installed, and it reads Chrome's registration rather than\n" +
			"    keeping its own — so this entry covers Opera too."
	}

	chromium := func(label, manifest, key string, present bool) hostTarget {
		return hostTarget{
			Label:       label,
			Manifest:    filepath.Join(dir, hostName+"."+manifest+".json"),
			Launcher:    launcher,
			Script:      launcherScript(exe),
			Detected:    present,
			Chromium:    true,
			RegistryKey: key,
		}
	}

	return []hostTarget{
		{
			Label:       "Firefox",
			Manifest:    filepath.Join(dir, hostName+".json"),
			Launcher:    launcher,
			Script:      launcherScript(exe),
			Detected:    true,
			RegistryKey: firefoxRegistryKey,
		},
		chrome,
		chromium("Chromium", "chromium", chromiumRegistryKey, installed(chromiumExecutables)),
		chromium("Edge", "edge", edgeRegistryKey, installed(edgeExecutables)),
		chromium("Brave", "brave", braveRegistryKey, installed(braveExecutables)),
		chromium("Vivaldi", "vivaldi", vivaldiRegistryKey, installed(vivaldiExecutables)),
	}, nil
}

// Detektionen ser efter selve programmet, ikke efter brugerdata. En
// afinstalleret browser efterlader sin profil under %LOCALAPPDATA%, så et
// kig derpå ville melde en browser installeret som maskinen ikke har.
//
// Edge findes i praksis under Program Files (x86) også på 64-bit Windows;
// det er ikke en fejl i listen.
var (
	chromeExecutables = []string{
		`Google\Chrome\Application\chrome.exe`,
	}
	chromiumExecutables = []string{
		`Chromium\Application\chrome.exe`,
	}
	edgeExecutables = []string{
		`Microsoft\Edge\Application\msedge.exe`,
	}
	braveExecutables = []string{
		`BraveSoftware\Brave-Browser\Application\brave.exe`,
	}
	vivaldiExecutables = []string{
		`Vivaldi\Application\vivaldi.exe`,
	}
	// Opera installerer normalt per bruger under %LOCALAPPDATA%\Programs, og
	// starteren hedder launcher.exe; opera.exe ligger i en versioneret
	// undermappe. Begge navne er med, for det har ikke altid været sådan.
	operaExecutables = []string{
		`Programs\Opera\launcher.exe`,
		`Programs\Opera\opera.exe`,
		`Opera\launcher.exe`,
		`Opera\opera.exe`,
	}
	// programRoots gennemsøges for de relative stier ovenfor.
	programRoots = []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA"}
)

func installed(relatives []string) bool {
	for _, env := range programRoots {
		root := os.Getenv(env)
		if root == "" {
			continue
		}
		for _, rel := range relatives {
			if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
				return true
			}
		}
	}
	return false
}

func registerManifest(t hostTarget) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, t.RegistryKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create registry key HKCU\\%s: %w", t.RegistryKey, err)
	}
	defer key.Close()

	// Browseren læser manifest-stien fra nøglens default-værdi (tomt navn).
	if err := key.SetStringValue("", t.Manifest); err != nil {
		return fmt.Errorf("set registry value: %w", err)
	}
	return nil
}

func unregisterManifest(t hostTarget) error {
	err := registry.DeleteKey(registry.CURRENT_USER, t.RegistryKey)
	if err != nil && !errors.Is(err, registry.ErrNotExist) && !os.IsNotExist(err) {
		return fmt.Errorf("delete registry key HKCU\\%s: %w", t.RegistryKey, err)
	}
	return nil
}

func registrationHint(t hostTarget) string {
	return fmt.Sprintf("registry: HKCU\\%s (default) = %s", t.RegistryKey, t.Manifest)
}
