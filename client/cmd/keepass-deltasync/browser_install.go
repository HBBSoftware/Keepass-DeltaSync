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
//
// Der er ikke nødvendigvis ÉT sted at skrive til. På Linux findes Firefox i
// mindst tre indpakninger med hver sin manifest-mappe, så platform-filerne
// leverer en liste af mål frem for én sti. Se hostTargets i hver af dem.

// nativeManifest er skemaet Firefox forventer.
type nativeManifest struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Path              string   `json:"path"`
	Type              string   `json:"type"`
	AllowedExtensions []string `json:"allowed_extensions"`
}

// hostTarget er ét sted et manifest skal ligge — i praksis én Firefox-variant.
// Launcheren følger med målet frem for at være fælles, fordi en sandkasset
// Firefox hverken kan læse den samme fil eller starte binæren på samme måde.
type hostTarget struct {
	Label    string // "Firefox", "Firefox (snap)", "Firefox (flatpak)"
	Manifest string // fuld sti til <hostName>.json
	Launcher string // fuld sti til launcher-scriptet
	Script   string // launcher-scriptets indhold
	Detected bool   // ser varianten ud til at være installeret?
	Hint     string // hvad brugeren selv skal gøre for netop denne variant
}

func runInstallBrowserHost(args []string) error {
	fs := flag.NewFlagSet("install-browser-host", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "print what would be written without touching anything")
	all := fs.Bool("all", false, "install for every known Firefox variant, not just the ones found on this machine")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync install-browser-host [--dry-run] [--all]")
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
	targets, err := hostTargets(exe)
	if err != nil {
		return err
	}

	manifest := nativeManifest{
		Name:        hostName,
		Description: "keepass-deltasync — search entries and open their URL",
		Type:        "stdio",
		AllowedExtensions: []string{
			browserExtensionID,
		},
	}

	if *dryRun {
		for _, t := range targets {
			mark := "  "
			if !t.Detected {
				mark = "- " // ikke fundet på denne maskine
			}
			fmt.Printf("%s%s\n", mark, t.Label)
			fmt.Printf("    launcher: %s\n", t.Launcher)
			fmt.Printf("      -> %s browser-host\n", exe)
			manifest.Path = t.Launcher
			body, err := json.MarshalIndent(manifest, "    ", "  ")
			if err != nil {
				return err
			}
			fmt.Printf("    manifest: %s\n    %s\n", t.Manifest, body)
			if hint := registrationHint(t); hint != "" {
				fmt.Printf("    %s\n", hint)
			}
			if t.Hint != "" {
				fmt.Printf("    note: %s\n", t.Hint)
			}
			fmt.Println()
		}
		return nil
	}

	written := 0
	for _, t := range targets {
		if !t.Detected && !*all {
			continue
		}
		manifest.Path = t.Launcher
		body, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(t.Launcher), 0o700); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(t.Launcher), err)
		}
		if err := os.WriteFile(t.Launcher, []byte(t.Script), 0o700); err != nil {
			return fmt.Errorf("write launcher %s: %w", t.Launcher, err)
		}
		if err := os.MkdirAll(filepath.Dir(t.Manifest), 0o700); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(t.Manifest), err)
		}
		if err := os.WriteFile(t.Manifest, append(body, '\n'), 0o600); err != nil {
			return fmt.Errorf("write manifest %s: %w", t.Manifest, err)
		}
		if err := registerManifest(t); err != nil {
			return err
		}
		written++

		fmt.Printf("%s\n", t.Label)
		fmt.Printf("  launcher: %s\n", t.Launcher)
		fmt.Printf("  manifest: %s\n", t.Manifest)
		if hint := registrationHint(t); hint != "" {
			fmt.Printf("  %s\n", hint)
		}
		if t.Hint != "" {
			fmt.Printf("  note: %s\n", t.Hint)
		}
	}
	if written == 0 {
		return errors.New("found no Firefox installation to register with — pass --all to install anyway")
	}

	fmt.Printf("\nInstalled browser host for %d Firefox variant(s), pointing at:\n  %s\n", written, exe)
	fmt.Printf("\nThe manifests hard-code the path above. Run this command again if you\n")
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

	// Bemærk: her ignoreres Detected. En variant der er afinstalleret siden
	// registreringen skal stadig ryddes op efter.
	exe, err := currentExecutable()
	if err != nil {
		return err
	}
	targets, err := hostTargets(exe)
	if err != nil {
		return err
	}

	var problems []error
	if err := unregisterManifest(); err != nil {
		problems = append(problems, err)
	}
	for _, t := range targets {
		for _, p := range []string{t.Manifest, t.Launcher} {
			if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
				problems = append(problems, fmt.Errorf("remove %s: %w", p, err))
			}
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
