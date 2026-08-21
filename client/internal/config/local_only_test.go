// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"os"
	"testing"
)

// TestDatabase_LocalOnly verificerer skellet mellem en synkroniseret binding
// og en fra `add-local`. Det er dét skel alle server-kommandoer hænger deres
// afvisning op på.
func TestDatabase_LocalOnly(t *testing.T) {
	synced := Database{Name: "work", RemoteID: "0e6b0d6c-1f4a-4f1e-9a6b-2b0f4d4c9f11"}
	local := Database{Name: "private", LocalID: "8f2c1b90-77a1-4e0c-9a3e-1d5c6f7a8b90"}

	if synced.LocalOnly() {
		t.Fatal("a database with a remote id must not count as local-only")
	}
	if !local.LocalOnly() {
		t.Fatal("a database without a remote id must count as local-only")
	}
}

// TestDatabase_SecretID sikrer at keyring-nøglen aldrig bliver den tomme
// streng. Gjorde den det, ville alle lokal-kun databaser dele ét slot og
// overskrive hinandens masterpassword.
func TestDatabase_SecretID(t *testing.T) {
	synced := Database{RemoteID: "remote-uuid", LocalID: "local-uuid"}
	if got := synced.SecretID(); got != "remote-uuid" {
		t.Fatalf("synced database should key on the remote id, got %q", got)
	}

	local := Database{LocalID: "local-uuid"}
	if got := local.SecretID(); got != "local-uuid" {
		t.Fatalf("local-only database should key on the local id, got %q", got)
	}
}

// TestLocalOnlyDatabase_RoundTrip verificerer at local_id overlever
// Save/Load, og at en gammel config uden feltet stadig loader.
func TestLocalOnlyDatabase_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)

	cfg := &Config{}
	cfg.AddDatabase(Database{
		Name:      "private",
		LocalPath: "/home/user/private.kdbx",
		LocalID:   "8f2c1b90-77a1-4e0c-9a3e-1d5c6f7a8b90",
	})
	cfg.AddDatabase(Database{
		Name:      "work",
		LocalPath: "/home/user/work.kdbx",
		RemoteID:  "0e6b0d6c-1f4a-4f1e-9a6b-2b0f4d4c9f11",
	})
	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	private := loaded.FindDatabase("private")
	if private == nil {
		t.Fatal("local-only database did not survive the round trip")
	}
	if !private.LocalOnly() {
		t.Fatalf("loaded database should still be local-only, remote_id=%q", private.RemoteID)
	}
	if private.SecretID() != "8f2c1b90-77a1-4e0c-9a3e-1d5c6f7a8b90" {
		t.Fatalf("local_id did not round-trip: %q", private.SecretID())
	}
	if work := loaded.FindDatabase("work"); work == nil || work.LocalOnly() {
		t.Fatal("the synced database must be unaffected by the local-only one")
	}

	// En config skrevet før add-local fandtes har ikke feltet overhovedet.
	path, err := Path()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	legacy := "[server]\n  url = \"https://example.test\"\n\n[[database]]\n  name = \"work\"\n  local_path = \"/tmp/work.kdbx\"\n  remote_id = \"0e6b0d6c-1f4a-4f1e-9a6b-2b0f4d4c9f11\"\n  last_seq = 7\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	old, err := Load()
	if err != nil {
		t.Fatalf("load legacy: %v", err)
	}
	db := old.FindDatabase("work")
	if db == nil {
		t.Fatal("legacy database not loaded")
	}
	if db.LocalID != "" {
		t.Fatalf("legacy config must not gain a local id, got %q", db.LocalID)
	}
	if db.LocalOnly() {
		t.Fatal("legacy synced database must not be treated as local-only")
	}
}
