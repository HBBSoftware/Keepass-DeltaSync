// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TOML skriver [known_keys] efter [[database]]-tabellerne. Det er lovligt,
// men en forkert feltrækkefølge ville få nøglerne læst ind i den sidste
// database — eller tabt i stilhed. Kører derfor hele vejen gennem Save/Load.
func TestKnownKeys_RoundTripAfterDatabases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("KEEPASS_DELTASYNC_CONFIG", path)

	cfg := &Config{Server: Server{URL: "https://example.org"}}
	cfg.AddDatabase(Database{Name: "a", LocalPath: "/a.kdbx", RemoteID: "id-a"})
	cfg.AddDatabase(Database{Name: "b", LocalPath: "/b.kdbx", RemoteID: "id-b"})
	if cfg.KnownKey("bob") != "" {
		t.Fatal("empty config should know no keys")
	}
	cfg.RememberKey("bob", "aa11")
	cfg.RememberKey("carol", "bb22")
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.KnownKey("bob") != "aa11" || got.KnownKey("carol") != "bb22" {
		t.Fatalf("known keys lost: %#v", got.KnownKeys)
	}
	if len(got.Databases) != 2 || got.Databases[1].RemoteID != "id-b" {
		t.Fatalf("databases changed: %#v", got.Databases)
	}

	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "[known_keys]") {
		t.Fatalf("expected a [known_keys] table:\n%s", raw)
	}
}
