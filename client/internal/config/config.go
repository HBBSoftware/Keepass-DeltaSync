// SPDX-License-Identifier: GPL-3.0-or-later

// Package config læser og skriver klient-konfigurationen i TOML.
// Filen ligger i OS-konventionel config-mappe under "keepass-deltasync/config.toml".
// Eksempel:
//
//	[server]
//	url          = "https://deltasync.example.dk"
//	device_token = "..."
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	appDir     = "keepass-deltasync"
	configFile = "config.toml"
	dirPerm    = 0o700
	filePerm   = 0o600
)

// Config repræsenterer hele config-filen.
type Config struct {
	Server Server `toml:"server"`
}

// Server holder server-URL og device-token (sat efter enrollment).
type Server struct {
	URL         string `toml:"url"`
	DeviceToken string `toml:"device_token,omitempty"`
}

// Path returnerer den fulde sti til config-filen. Mappen oprettes ikke her.
func Path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	return filepath.Join(base, appDir, configFile), nil
}

// Load læser config-filen. Hvis filen ikke findes returneres en tom Config
// uden fejl, så førstegangsbrugere kan køre `enroll` uden manuelt setup.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	var cfg Config
	_, err = toml.DecodeFile(path, &cfg)
	if errors.Is(err, fs.ErrNotExist) {
		return &cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return &cfg, nil
}

// Save skriver config atomisk: write til temp + rename. Mappen oprettes om
// nødvendigt med restriktive permissions (token er hemmelig).
func Save(cfg *Config) error {
	path, err := Path()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, configFile+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op hvis renamed

	if err := os.Chmod(tmpPath, filePerm); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}

	enc := toml.NewEncoder(tmp)
	if err := enc.Encode(cfg); err != nil {
		tmp.Close()
		return fmt.Errorf("encode toml: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}
