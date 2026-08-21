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

// registryKeyPath er HKCU-nøglen Firefox slår op i.
const registryKeyPath = `Software\Mozilla\NativeMessagingHosts\` + hostName

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

// hostTargets — Windows har kun ét mål. Manifestets placering er ligegyldig
// (registry-nøglen peger på den), så den ligger sammen med launcheren.
func hostTargets(exe string) ([]hostTarget, error) {
	dir, err := hostDataDir()
	if err != nil {
		return nil, err
	}
	return []hostTarget{{
		Label:    "Firefox",
		Manifest: filepath.Join(dir, hostName+".json"),
		Launcher: filepath.Join(dir, launcherFileName),
		Script:   launcherScript(exe),
		Detected: true,
	}}, nil
}

func registerManifest(t hostTarget) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create registry key HKCU\\%s: %w", registryKeyPath, err)
	}
	defer key.Close()

	// Firefox læser manifest-stien fra nøglens default-værdi (tomt navn).
	if err := key.SetStringValue("", t.Manifest); err != nil {
		return fmt.Errorf("set registry value: %w", err)
	}
	return nil
}

func unregisterManifest() error {
	err := registry.DeleteKey(registry.CURRENT_USER, registryKeyPath)
	if err != nil && !errors.Is(err, registry.ErrNotExist) && !os.IsNotExist(err) {
		return fmt.Errorf("delete registry key HKCU\\%s: %w", registryKeyPath, err)
	}
	return nil
}

func registrationHint(t hostTarget) string {
	return fmt.Sprintf("registry: HKCU\\%s (default) = %s", registryKeyPath, t.Manifest)
}
