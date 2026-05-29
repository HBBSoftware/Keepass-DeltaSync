// SPDX-License-Identifier: GPL-3.0-or-later

//go:build emit_fixture

// Køres manuelt for at re-generere Kotlin-test-fixture'en når canonical-skemaet
// ændres. Den producerede JSON tjekkes ind under android/sync/src/test/resources/
// så Kotlin-testen validerer at den parser præcis det Go-emitterer.
//
// Kør med:
//
//	go test -tags=emit_fixture -run TestEmitFixture ./internal/kdbx/canonical/

package canonical

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEmitFixture(t *testing.T) {
	qc := true
	mtime := time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC)
	e := &Entry{
		V:    SchemaVersion,
		UUID: "00010203-0405-0607-0809-0a0b0c0d0e0f",
		Times: Times{
			Created:         time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			Modified:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			Accessed:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			LocationChanged: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			Expires:         false,
			UsageCount:      5,
		},
		Strings: map[string]String{
			"Title":     {V: "GitLab"},
			"UserName":  {V: "hans"},
			"Password":  {V: "s3cr3t", Protected: true},
			"URL":       {V: "https://gitlab.com"},
			"Notes":     {V: ""},
			"API-Token": {V: "tok-12345", Protected: true},
		},
		Binaries: []Binary{
			{Name: "key.pem", Data: []byte{0x00, 0xff, 0x42}},
		},
		Tags:         []string{"work", "important"},
		IconID:       0,
		QualityCheck: &qc,
		CustomData: map[string]CustomDataItem{
			"KPXC_ext": {V: "browser-state", Modified: &mtime},
		},
		AutoType: &AutoType{
			Enabled:         true,
			DefaultSequence: "{USERNAME}{TAB}{PASSWORD}{ENTER}",
		},
	}

	raw, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Find repo-roden via go.mod's placering: client/go.mod ligger to mapper op.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(wd))))
	dest := filepath.Join(repoRoot, "android", "sync", "src", "test", "resources", "canonical-entry-fixture.json")

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(dest, raw, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Logf("wrote %d bytes to %s", len(raw), dest)
}
