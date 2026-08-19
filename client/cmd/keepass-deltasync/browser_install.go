// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// Installation af native messaging-manifestet, så Firefox kan finde
// browser-host'en. Se docs/browser-extension.md.
//
// Firefox' manifest peger på ét eksekverbart program og understøtter ikke
// argumenter. Vores host er en subkommando, så vi skriver et lille
// launcher-script ved siden af manifestet, som kalder binæren med
// `browser-host`. Det er samme mønster som Mozillas egen dokumentation
// bruger til at pege på et Python-script under Windows.

// nativeManifest er skemaet Firefox forventer.
type nativeManifest struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Path              string   `json:"path"`
	Type              string   `json:"type"`
	AllowedExtensions []string `json:"allowed_extensions"`
}

func runInstallBrowserHost(args []string) error {
	fs := flag.NewFlagSet("install-browser-host", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "print what would be written without touching anything")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync install-browser-host [--dry-run]")
		fmt.Fprintln(fs.Output(), "\nRegisters the browser host with Firefox so the extension can reach it.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	exe, err := currentExecutable()
	if err != nil {
		return err
	}
	dataDir, err := hostDataDir()
	if err != nil {
		return err
	}
	manifestPath, err := nativeManifestPath()
	if err != nil {
		return err
	}
	launcherPath := filepath.Join(dataDir, launcherFileName)

	manifest := nativeManifest{
		Name:              hostName,
		Description:       "keepass-deltasync — search entries and open their URL",
		Path:              launcherPath,
		Type:              "stdio",
		AllowedExtensions: []string{browserExtensionID},
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	if *dryRun {
		fmt.Printf("launcher: %s\n  -> %s browser-host\n\n", launcherPath, exe)
		fmt.Printf("manifest: %s\n%s\n", manifestPath, body)
		if hint := registrationHint(manifestPath); hint != "" {
			fmt.Printf("\n%s\n", hint)
		}
		return nil
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dataDir, err)
	}
	if err := os.WriteFile(launcherPath, []byte(launcherScript(exe)), 0o700); err != nil {
		return fmt.Errorf("write launcher %s: %w", launcherPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(manifestPath), err)
	}
	if err := os.WriteFile(manifestPath, append(body, '\n'), 0o600); err != nil {
		return fmt.Errorf("write manifest %s: %w", manifestPath, err)
	}
	if err := registerManifest(manifestPath); err != nil {
		return err
	}

	fmt.Printf("Installed browser host for Firefox.\n")
	fmt.Printf("  launcher: %s\n", launcherPath)
	fmt.Printf("  manifest: %s\n", manifestPath)
	fmt.Printf("  binary:   %s\n", exe)
	fmt.Printf("\nThe manifest hard-codes the path above. Run this command again if you\n")
	fmt.Printf("move or reinstall the binary, and restart Firefox afterwards.\n")
	return nil
}

func runUninstallBrowserHost(args []string) error {
	fs := flag.NewFlagSet("uninstall-browser-host", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync uninstall-browser-host")
		fmt.Fprintln(fs.Output(), "\nRemoves the native messaging manifest and launcher.")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	manifestPath, err := nativeManifestPath()
	if err != nil {
		return err
	}
	dataDir, err := hostDataDir()
	if err != nil {
		return err
	}
	launcherPath := filepath.Join(dataDir, launcherFileName)

	var problems []error
	if err := unregisterManifest(); err != nil {
		problems = append(problems, err)
	}
	for _, p := range []string{manifestPath, launcherPath} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			problems = append(problems, fmt.Errorf("remove %s: %w", p, err))
		}
	}
	if len(problems) > 0 {
		return errors.Join(problems...)
	}

	fmt.Println("Browser host removed. Restart Firefox for the change to take effect.")
	return nil
}

// currentExecutable returnerer den absolutte, symlink-opløste sti til
// binæren. Manifestet skal pege på en fast sti, ikke på hvad PATH tilfældigvis
// finder når Firefox starter hosten.
func currentExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate own binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Abs(exe)
}
