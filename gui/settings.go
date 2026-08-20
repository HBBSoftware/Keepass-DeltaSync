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
	s := settings{Language: string(langDA)}
	p, err := settingsPath()
	if err != nil {
		return s
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s)
	if s.Language == "" {
		s.Language = string(langDA)
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
