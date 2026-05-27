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
	Server    Server     `toml:"server"`
	Databases []Database `toml:"database"`
}

// Server holder server-URL og device-token (sat efter enrollment).
type Server struct {
	URL         string `toml:"url"`
	DeviceToken string `toml:"device_token,omitempty"`
}

// Database er bindingen mellem en lokal .kdbx-fil og en database registreret
// hos serveren. RemoteID er den server-genererede UUID — den UUID HKDF bruger
// til at derivere entry-nøgler, så samme masterpassword på forskellige
// databaser ikke giver samme nøglemateriale.
//
// EntryStates er per-entry mtime-tracking: en map fra entry-UUID til den
// senest pushede (eller pullede) mtime som ISO 8601-streng. Push-delta
// sammenligner pr. entry mod denne map, så hverken just-pullede entries
// re-pushes eller same-second-edits mistes. Hvis en entry ikke er i map'en
// betragtes den som "aldrig pushet" og pushes næste sync.
//
// LastPush er beholdt som deprecated felt for bagudkompatibilitet med ældre
// configs — feltet skrives ikke længere af klienten, men eksisterende værdier
// bevares så manuel inspektion stadig giver mening.
type Database struct {
	Name        string            `toml:"name"`
	LocalPath   string            `toml:"local_path"`
	RemoteID    string            `toml:"remote_id"`
	LastSeq     int64             `toml:"last_seq"`
	LastPush    string            `toml:"last_push,omitempty"`
	EntryStates map[string]string `toml:"entry_states,omitempty"`
}

// RecordEntryState gemmer den seneste sete mtime for en entry (eller deletion).
// Initialiserer map'en lazy. Bruges efter både push og pull.
func (d *Database) RecordEntryState(uuid, mtimeISO string) {
	if d.EntryStates == nil {
		d.EntryStates = make(map[string]string)
	}
	d.EntryStates[uuid] = mtimeISO
}

// FindDatabase returnerer en pointer til den lokale database med det navn,
// eller nil hvis ingen findes. Pointeren tillader caller'en at mutere
// LastSeq/LastPush efter en sync.
func (c *Config) FindDatabase(name string) *Database {
	for i := range c.Databases {
		if c.Databases[i].Name == name {
			return &c.Databases[i]
		}
	}
	return nil
}

// AddDatabase tilføjer en ny database-binding. Caller skal selv sørge for at
// navnet ikke kollidere — FindDatabase før dette kald.
func (c *Config) AddDatabase(db Database) {
	c.Databases = append(c.Databases, db)
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
