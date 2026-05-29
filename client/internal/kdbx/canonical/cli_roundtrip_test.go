// SPDX-License-Identifier: GPL-3.0-or-later

//go:build cli

// Build-tagget `cli` afgrænser denne integration-test til kørsler hvor
// keepassxc-cli faktisk er tilgængelig. Kør med:
//
//	go test -tags=cli ./internal/kdbx/canonical/
//
// Default `go test` springer den over. CI uden KeePassXC-installeret behøver
// ikke gøre noget særligt.

package canonical_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx"
	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx/canonical"
)

// testPassword er det password vi bruger på alle test-kdbx'er. Vi krypterer
// reelt rigtige .kdbx-filer på disk i en t.TempDir, så vi har brug for et
// password; men da vi laver dem fra scratch i testen er værdien irrelevant.
const testPassword = "cli-roundtrip-test-password"

// TestCLIRoundTrip validerer at canonical-pakkens emit-stien faktisk produerer
// XML som keepassxc-cli kan importere og round-trippe uden at tabe felter.
// Det er den højest-risk del af Phase B — vores enhedstests round-tripper
// canonical→XML→canonical inden i samme pakke, men en faktisk keepassxc-cli
// kan sagtens have validerings-regler vi mangler at adlyde.
//
// Flow:
//
//	1. Byg in-memory canonical.Entry → ToInnerXML
//	2. Wrap som staging-kdbx-XML (samme path som pull-pipeline'en bruger)
//	3. keepassxc-cli import → frisk .kdbx
//	4. keepassxc-cli export → ny XML
//	5. Parse, kør første entry's fragment gennem FromInnerXML
//	6. Sammenlign kritiske felter med original entry
//
// Hvis (3) eller (4) fejler er det stærkt signal om at ToInnerXML emitterer
// noget keepassxc-cli ikke accepterer.
func TestCLIRoundTrip(t *testing.T) {
	cli, err := kdbx.NewCLI("")
	if err != nil {
		t.Skipf("keepassxc-cli not available: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dir := t.TempDir()

	origEntry := &canonical.Entry{
		V:    canonical.SchemaVersion,
		UUID: "00010203-0405-0607-0809-0a0b0c0d0e0f",
		Times: canonical.Times{
			Created:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Modified:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			Accessed:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			LocationChanged: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		Strings: map[string]canonical.String{
			"Title":     {V: "GitLab"},
			"UserName":  {V: "hans"},
			"Password":  {V: "s3cr3t", Protected: true},
			"URL":       {V: "https://gitlab.com"},
			"Notes":     {V: "with & ampersand"},
			"API-Token": {V: "tok-12345", Protected: true},
		},
		Tags: []string{"work", "important"},
	}

	fragment, err := canonical.ToInnerXML(origEntry)
	if err != nil {
		t.Fatalf("ToInnerXML: %v", err)
	}

	stagingXML, err := kdbx.BuildStagingXML([]kdbx.StagingEntry{{
		UUID:       origEntry.UUID,
		Fragment:   fragment,
		ModifiedAt: origEntry.Times.Modified,
	}}, nil, "")
	if err != nil {
		t.Fatalf("BuildStagingXML: %v", err)
	}

	xmlPath := filepath.Join(dir, "staging.xml")
	kdbxPath := filepath.Join(dir, "test.kdbx")
	if err := os.WriteFile(xmlPath, stagingXML, 0o600); err != nil {
		t.Fatalf("write staging xml: %v", err)
	}
	if err := cli.Import(ctx, xmlPath, kdbxPath, []byte(testPassword)); err != nil {
		t.Fatalf("keepassxc-cli rejected our generated XML — ToInnerXML output is malformed:\n%v\nstaging XML:\n%s", err, stagingXML)
	}

	exported, err := cli.Export(ctx, kdbxPath, []byte(testPassword))
	if err != nil {
		t.Fatalf("cli.Export: %v", err)
	}

	entries, _, err := kdbx.ParseExport(exported)
	if err != nil {
		t.Fatalf("ParseExport: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after CLI round-trip, got %d", len(entries))
	}

	parsed, err := canonical.FromInnerXML(entries[0].Fragment)
	if err != nil {
		t.Fatalf("FromInnerXML on cli-exported fragment: %v\nfragment:\n%s", err, entries[0].Fragment)
	}

	// Sammenlign kritiske, brugersynlige felter. Vi sammenligner IKKE hele
	// strukturen — keepassxc-cli kan have tilføjet AutoType-skelet, sat
	// IconID, eller normaliseret andre felter vi ikke explicitly satte.
	// Det er forventet og ikke en regression.
	if parsed.UUID != origEntry.UUID {
		t.Errorf("UUID drift: %q vs %q", parsed.UUID, origEntry.UUID)
	}
	wantStrings := map[string]canonical.String{
		"Title":     {V: "GitLab"},
		"UserName":  {V: "hans"},
		"Password":  {V: "s3cr3t", Protected: true},
		"URL":       {V: "https://gitlab.com"},
		"Notes":     {V: "with & ampersand"},
		"API-Token": {V: "tok-12345", Protected: true},
	}
	for k, want := range wantStrings {
		got, ok := parsed.Strings[k]
		if !ok {
			t.Errorf("String %q missing after CLI round-trip", k)
			continue
		}
		if got.V != want.V {
			t.Errorf("String %q value drift: %q vs %q", k, got.V, want.V)
		}
		if got.Protected != want.Protected {
			t.Errorf("String %q Protected drift: %v vs %v", k, got.Protected, want.Protected)
		}
	}

	wantTags := map[string]bool{"work": true, "important": true}
	for _, tag := range parsed.Tags {
		delete(wantTags, tag)
	}
	if len(wantTags) > 0 {
		t.Errorf("Tags lost after CLI round-trip: %v", wantTags)
	}
}

// TestCLIRoundTrip_MergeIntoExisting verificerer det fulde pull-flow:
// vores canonical-emitterede fragment kan merges ind i en eksisterende kdbx
// uden at det breaker keepassxc-cli's merge-engine.
func TestCLIRoundTrip_MergeIntoExisting(t *testing.T) {
	cli, err := kdbx.NewCLI("")
	if err != nil {
		t.Skipf("keepassxc-cli not available: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dir := t.TempDir()

	// 1. Byg en "target"-kdbx med én eksisterende entry.
	targetEntry := &canonical.Entry{
		V:    canonical.SchemaVersion,
		UUID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Times: canonical.Times{
			Created:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Modified:        time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			Accessed:        time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			LocationChanged: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		Strings: map[string]canonical.String{
			"Title": {V: "ExistingEntry"},
		},
	}
	targetFragment, err := canonical.ToInnerXML(targetEntry)
	if err != nil {
		t.Fatalf("emit target: %v", err)
	}
	targetXML, err := kdbx.BuildStagingXML([]kdbx.StagingEntry{{
		UUID:       targetEntry.UUID,
		Fragment:   targetFragment,
		ModifiedAt: targetEntry.Times.Modified,
	}}, nil, "")
	if err != nil {
		t.Fatalf("BuildStagingXML target: %v", err)
	}
	targetXMLPath := filepath.Join(dir, "target.xml")
	targetKDBXPath := filepath.Join(dir, "target.kdbx")
	if err := os.WriteFile(targetXMLPath, targetXML, 0o600); err != nil {
		t.Fatalf("write target xml: %v", err)
	}
	if err := cli.Import(ctx, targetXMLPath, targetKDBXPath, []byte(testPassword)); err != nil {
		t.Fatalf("import target: %v", err)
	}

	// 2. Byg en "source"-staging-kdbx med en NY entry (simulerer pull's
	//    new-from-server entry).
	newEntry := &canonical.Entry{
		V:    canonical.SchemaVersion,
		UUID: "11111111-2222-3333-4444-555555555555",
		Times: canonical.Times{
			Created:         time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC),
			Modified:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			Accessed:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			LocationChanged: time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC),
		},
		Strings: map[string]canonical.String{
			"Title":    {V: "NewFromServer"},
			"Password": {V: "freshpass", Protected: true},
		},
	}
	newFragment, err := canonical.ToInnerXML(newEntry)
	if err != nil {
		t.Fatalf("emit new: %v", err)
	}
	sourceXML, err := kdbx.BuildStagingXML([]kdbx.StagingEntry{{
		UUID:       newEntry.UUID,
		Fragment:   newFragment,
		ModifiedAt: newEntry.Times.Modified,
	}}, nil, "")
	if err != nil {
		t.Fatalf("BuildStagingXML source: %v", err)
	}
	sourceXMLPath := filepath.Join(dir, "source.xml")
	sourceKDBXPath := filepath.Join(dir, "source.kdbx")
	if err := os.WriteFile(sourceXMLPath, sourceXML, 0o600); err != nil {
		t.Fatalf("write source xml: %v", err)
	}
	if err := cli.Import(ctx, sourceXMLPath, sourceKDBXPath, []byte(testPassword)); err != nil {
		t.Fatalf("import source: %v", err)
	}

	// 3. Merge source → target.
	if err := cli.Merge(ctx, targetKDBXPath, sourceKDBXPath, []byte(testPassword)); err != nil {
		t.Fatalf("cli.Merge: %v", err)
	}

	// 4. Exportér og verificer at begge entries er der.
	merged, err := cli.Export(ctx, targetKDBXPath, []byte(testPassword))
	if err != nil {
		t.Fatalf("cli.Export merged: %v", err)
	}
	entries, _, err := kdbx.ParseExport(merged)
	if err != nil {
		t.Fatalf("ParseExport merged: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after merge, got %d", len(entries))
	}

	titles := make(map[string]bool)
	for _, e := range entries {
		parsed, err := canonical.FromInnerXML(e.Fragment)
		if err != nil {
			t.Fatalf("FromInnerXML merged entry: %v", err)
		}
		titles[parsed.Strings["Title"].V] = true
	}
	if !titles["ExistingEntry"] {
		t.Error("ExistingEntry lost in merge")
	}
	if !titles["NewFromServer"] {
		t.Error("NewFromServer not merged in")
	}
}
