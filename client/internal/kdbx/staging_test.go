// SPDX-License-Identifier: GPL-3.0-or-later

package kdbx

import (
	"testing"
	"time"
)

// fragmentFor bygger et minimalt entry-InnerXML-fragment med given UUID (hex),
// som ParseExport kan parse igen.
func fragmentFor(t *testing.T, uuidHex string) []byte {
	t.Helper()
	b64, err := uuidHexToBase64(uuidHex)
	if err != nil {
		t.Fatalf("uuidHexToBase64(%s): %v", uuidHex, err)
	}
	return []byte("<UUID>" + b64 + "</UUID><Times><LastModificationTime>2026-05-28T10:00:00Z</LastModificationTime></Times>")
}

// TestBuildStagingXMLWithGroups_RoundTrip bygger et gruppetræ, kører det gennem
// BuildStagingXMLWithGroups og parser resultatet med ParseExport igen. Det
// verificerer at entries lander i rigtige grupper, at gruppe-hierarkiet
// (parent) bevares, og at gruppe-navne XML-escapes korrekt.
func TestBuildStagingXMLWithGroups_RoundTrip(t *testing.T) {
	const (
		rootHex = "f0f1f2f3-f4f5-f6f7-f8f9-fafbfcfdfeff"
		workHex = "10111213-1415-1617-1819-1a1b1c1d1e1f"
		subHex  = "30313233-3435-3637-3839-3a3b3c3d3e3f"
		e1Hex   = "00010203-0405-0607-0809-0a0b0c0d0e0f" // i Root
		e2Hex   = "20212223-2425-2627-2829-2a2b2c2d2e2f" // i Work
		e3Hex   = "40414243-4445-4647-4849-4a4b4c4d4e4f" // i Sub
		orphHex = "50515253-5455-5657-5859-5a5b5c5d5e5f" // entry m. ukendt parent → Root
	)
	rootB64, err := uuidHexToBase64(rootHex)
	if err != nil {
		t.Fatalf("root b64: %v", err)
	}

	entries := []StagingEntry{
		{Fragment: fragmentFor(t, e1Hex), ParentGroup: ""},
		{Fragment: fragmentFor(t, e2Hex), ParentGroup: workHex},
		{Fragment: fragmentFor(t, e3Hex), ParentGroup: subHex},
		{Fragment: fragmentFor(t, orphHex), ParentGroup: "99999999-9999-9999-9999-999999999999"},
	}
	mod := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	groups := []StagingGroup{
		{UUID: workHex, ParentUUID: "", Name: "Work", IconID: 48, ModifiedAt: mod},
		{UUID: subHex, ParentUUID: workHex, Name: "Sub & <Co>", IconID: 0, ModifiedAt: mod},
	}

	xmlBytes, err := BuildStagingXMLWithGroups(entries, groups, nil, rootB64)
	if err != nil {
		t.Fatalf("BuildStagingXMLWithGroups: %v", err)
	}

	gotEntries, gotGroups, _, err := ParseExport(xmlBytes)
	if err != nil {
		t.Fatalf("ParseExport(staging output): %v\n%s", err, xmlBytes)
	}

	if len(gotEntries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(gotEntries))
	}
	if len(gotGroups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(gotGroups))
	}

	parentOf := map[string]string{}
	for _, e := range gotEntries {
		parentOf[e.UUID] = e.ParentGroupUUID
	}
	if parentOf[e1Hex] != "" {
		t.Errorf("e1 parent = %q, want \"\" (Root)", parentOf[e1Hex])
	}
	if parentOf[e2Hex] != workHex {
		t.Errorf("e2 parent = %q, want %q (Work)", parentOf[e2Hex], workHex)
	}
	if parentOf[e3Hex] != subHex {
		t.Errorf("e3 parent = %q, want %q (Sub)", parentOf[e3Hex], subHex)
	}
	// Orphan: ukendt parent → placeret i Root.
	if parentOf[orphHex] != "" {
		t.Errorf("orphan parent = %q, want \"\" (redirected to Root)", parentOf[orphHex])
	}

	byUUID := map[string]Group{}
	for _, g := range gotGroups {
		byUUID[g.UUID] = g
	}
	if byUUID[workHex].ParentUUID != "" {
		t.Errorf("Work parent = %q, want \"\" (Root)", byUUID[workHex].ParentUUID)
	}
	if byUUID[subHex].ParentUUID != workHex {
		t.Errorf("Sub parent = %q, want %q (Work)", byUUID[subHex].ParentUUID, workHex)
	}
	// XML-escaping af gruppe-navn skal round-trippe.
	if byUUID[subHex].Name != "Sub & <Co>" {
		t.Errorf("Sub name = %q, want %q (XML escaping round-trip)", byUUID[subHex].Name, "Sub & <Co>")
	}
}
