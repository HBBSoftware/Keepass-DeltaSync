// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// settings er GUI'ens egne præferencer — IKKE klient-konfigurationen. Klientens
// config (server-token, database-bindinger, krypto-nøgler) ejes udelukkende af
// CLI'en og ligger i dens egen config.toml. Her gemmer vi kun to ting: hvor
// CLI'en ligger, og hvilket sprog UI'en skal vise.
type settings struct {
	CLIPath       string `json:"cli_path"`
	Language      string `json:"language"`
	Theme         string `json:"theme"`           // "system" (følg OS), "light" eller "dark"
	ShowHelpPanel bool   `json:"show_help_panel"` // vis wiki-agtigt hjælpe-panel i bunden

	// CheckUpdates styrer om GUI'en spørger GitLab efter en nyere udgivelse
	// ved opstart. nil betyder "ikke sat" og regnes som slået til, så
	// eksisterende installationer får funktionen uden at røre gui.json.
	CheckUpdates *bool `json:"check_updates,omitempty"`
}

// updateCheckEnabled er standard-til: kun et udtrykkeligt false slår det fra.
func (s settings) updateCheckEnabled() bool {
	return s.CheckUpdates == nil || *s.CheckUpdates
}

// settingsPath er <os-config-dir>/keepass-deltasync-gui/gui.json.
func settingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "keepass-deltasync-gui", "gui.json"), nil
}

func loadSettings() settings {
	// Intet gemt valg betyder "ikke valgt endnu", ikke "dansk". Vi slår derfor
	// operativsystemets sprog op hver gang i stedet for at skrive det i
	// gui.json: så følger GUI'en med, hvis brugeren flytter til en maskine med
	// et andet sprog, indtil de aktivt vælger et i Indstillinger.
	s := settings{}
	p, err := settingsPath()
	if err != nil {
		s.Language = string(detectLang())
		return s
	}
	data, err := os.ReadFile(p)
	if err != nil {
		s.Language = string(detectLang())
		return s
	}
	_ = json.Unmarshal(data, &s)
	if s.Language == "" {
		s.Language = string(detectLang())
	}
	return s
}

func saveSettings(s settings) error {
	p, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
