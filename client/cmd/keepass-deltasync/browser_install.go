// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// nativeManifest er skemaet browseren forventer. De to familier er enige om
// alt undtagen hvem der må tale med hosten: Firefox lister udvidelses-id'er i
// allowed_extensions, Chromium lister hele oprindelser i allowed_origins.
// Skriver man den ene form til den anden browser, starter hosten aldrig — og
// browseren siger ikke hvorfor.
type nativeManifest struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Path              string   `json:"path"`
	Type              string   `json:"type"`
	AllowedExtensions []string `json:"allowed_extensions,omitempty"`
	AllowedOrigins    []string `json:"allowed_origins,omitempty"`
}

// chromiumExtensionIDs er de id'er Chrome og Edge kender udvidelsen under.
//
// Til forskel fra Firefox, hvor id'et er et navn vi selv vælger og skriver i
// manifestet, udleder Chromium id'et af den nøgle pakken er signeret med — og
// den nøgle laver butikken, første gang varen oprettes. Id'et findes altså
// ikke før udvidelsen er uploadet, og Chrome Web Store og Edge Add-ons giver
// hver sit.
//
// Derfor er listen tom her og fyldes med --extension-id indtil de to butikker
// har svaret. Et id under udvikling står på chrome://extensions når pakken er
// indlæst med "Load unpacked"; det ændrer sig ikke så længe mappen ligger det
// samme sted.
var chromiumExtensionIDs = []string{}

// stringList samler et flag der må gentages. flag-pakken kan ikke selv, og
// --extension-id skal kunne nævnes én gang per browser.
type stringList []string

func (l *stringList) String() string { return strings.Join(*l, ",") }

func (l *stringList) Set(v string) error {
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*l = append(*l, part)
		}
	}
	return nil
}

// manifestFor bygger manifestet til ét mål. Den skal kaldes per mål og ikke
// én gang for alle: både stien og listen over hvem der må kalde er forskellig
// fra browser til browser.
func manifestFor(t hostTarget, chromiumIDs []string) nativeManifest {
	m := nativeManifest{
		Name:        hostName,
		Description: "keepass-deltasync — search entries and open their URL",
		Path:        t.Launcher,
		Type:        "stdio",
	}
	if t.Chromium {
		for _, id := range chromiumIDs {
			m.AllowedOrigins = append(m.AllowedOrigins, "chrome-extension://"+id+"/")
		}
		return m
	}
	m.AllowedExtensions = []string{browserExtensionID}
	return m
}

// hostTarget er ét sted et manifest skal ligge — i praksis én Firefox-variant.
// Launcheren følger med målet frem for at være fælles, fordi en sandkasset
// Firefox hverken kan læse den samme fil eller starte binæren på samme måde.
type hostTarget struct {
	Label    string // "Firefox", "Firefox (snap)", "Chrome", "Edge"
	Manifest string // fuld sti til <hostName>.json
	Launcher string // fuld sti til launcher-scriptet
	Script   string // launcher-scriptets indhold
	Detected bool   // ser varianten ud til at være installeret?
	Hint     string // hvad brugeren selv skal gøre for netop denne variant

	// Chromium vælger manifest-formen. Se nativeManifest.
	Chromium bool

	// RegistryKey bruges kun på Windows, hvor manifestet findes gennem en
	// nøgle under HKCU frem for gennem sin placering. Tom på alle andre
	// platforme, og hver browser har sin egen.
	RegistryKey string
}

// Flere mål må gerne dele launcher — den er den samme uanset hvem der kalder
// den — men aldrig manifest, for indholdet er forskelligt.

func runInstallBrowserHost(args []string) error {
	fs := flag.NewFlagSet("install-browser-host", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "print what would be written without touching anything")
	all := fs.Bool("all", false, "install for every known browser variant, not just the ones found on this machine")
	var extraIDs stringList
	fs.Var(&extraIDs, "extension-id", "Chrome/Edge extension ID to allow (repeatable; see chrome://extensions)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync install-browser-host [--dry-run] [--all] [--extension-id ID]")
		fmt.Fprintln(fs.Output(), "\nRegisters the browser host with Firefox, Chrome and Edge so the extension")
		fmt.Fprintln(fs.Output(), "can reach it. Chrome and Edge identify an extension by an ID the store")
		fmt.Fprintln(fs.Output(), "assigns, so those two are skipped until one is passed with --extension-id.")
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

	chromiumIDs := append(append([]string{}, chromiumExtensionIDs...), extraIDs...)

	if *dryRun {
		for _, t := range targets {
			mark := "  "
			if !t.Detected {
				mark = "- " // ikke fundet på denne maskine
			}
			fmt.Printf("%s%s\n", mark, t.Label)
			if t.Chromium && len(chromiumIDs) == 0 {
				fmt.Printf("    skipped: no extension ID — pass --extension-id\n\n")
				continue
			}
			fmt.Printf("    launcher: %s\n", t.Launcher)
			fmt.Printf("      -> %s browser-host\n", exe)
			body, err := json.MarshalIndent(manifestFor(t, chromiumIDs), "    ", "  ")
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

	written, skipped := 0, 0
	for _, t := range targets {
		if !t.Detected && !*all {
			continue
		}
		// En Chromium-browser uden id ville få et manifest hvor
		// allowed_origins er tom. Det er ikke "alle må" men "ingen må", og
		// hosten ville så afvise udvidelsen uden at nogen af delene siger
		// hvorfor. Bedre at lade være og sige det.
		if t.Chromium && len(chromiumIDs) == 0 {
			skipped++
			continue
		}
		body, err := json.MarshalIndent(manifestFor(t, chromiumIDs), "", "  ")
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
		if skipped > 0 {
			return errors.New("the only browsers found were Chrome or Edge, and neither has an extension ID yet — pass --extension-id")
		}
		return errors.New("found no browser to register with — pass --all to install anyway")
	}

	fmt.Printf("\nInstalled browser host for %d browser(s), pointing at:\n  %s\n", written, exe)
	if skipped > 0 {
		fmt.Printf("\nSkipped %d Chromium browser(s): pass --extension-id with the ID from\n", skipped)
		fmt.Printf("chrome://extensions (or edge://extensions) to register those too.\n")
	}
	fmt.Printf("\nThe manifests hard-code the path above. Run this command again if you\n")
	fmt.Printf("move or reinstall the binary, and restart the browser afterwards.\n")
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
	for _, t := range targets {
		if err := unregisterManifest(t); err != nil {
			problems = append(problems, err)
		}
		for _, p := range []string{t.Manifest, t.Launcher} {
			if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
				problems = append(problems, fmt.Errorf("remove %s: %w", p, err))
			}
		}
	}
	if len(problems) > 0 {
		return errors.Join(problems...)
	}

	fmt.Println("Browser host removed. Restart the browser for the change to take effect.")
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
