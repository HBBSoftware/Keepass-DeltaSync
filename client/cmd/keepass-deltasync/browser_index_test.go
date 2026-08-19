// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// browserFixtureXML er en KeePassFile-eksport der rammer alle de kanter
// indekseringen skal håndtere: papirkurv, en søge-deaktiveret gruppe, en
// undergruppe der arver deaktiveringen, ekstra URL-felter, et protected
// custom-felt, og URL-værdier der ikke kan navigeres til.
const browserFixtureXML = `<?xml version="1.0" encoding="UTF-8"?>
<KeePassFile>
  <Meta>
    <RecycleBinEnabled>True</RecycleBinEnabled>
    <RecycleBinUUID>MTIzNDU2Nzg5Ojs8PT4/QA==</RecycleBinUUID>
  </Meta>
  <Root>
    <Group>
      <UUID>AAECAwQFBgcICQoLDA0ODw==</UUID>
      <Name>Root</Name>
      <Entry>
        <UUID>QUJDREVGR0hJSktMTU5PUA==</UUID>
        <Times><CreationTime>2026-08-01T09:00:00Z</CreationTime><LastModificationTime>2026-08-01T10:00:00Z</LastModificationTime><LastAccessTime>2026-08-01T10:00:00Z</LastAccessTime><LocationChanged>2026-08-01T09:00:00Z</LocationChanged></Times>
        <String><Key>Title</Key><Value>Root entry</Value></String>
        <String><Key>URL</Key><Value>https://root.example/</Value></String>
        <String><Key>Password</Key><Value>hunter2</Value></String>
        <String><Key>UserName</Key><Value>hans</Value></String>
      </Entry>
      <Entry>
        <UUID>gYKDhIWGh4iJiouMjY6PkA==</UUID>
        <Times><CreationTime>2026-08-01T09:00:00Z</CreationTime><LastModificationTime>2026-08-01T10:00:00Z</LastModificationTime><LastAccessTime>2026-08-01T10:00:00Z</LastAccessTime><LocationChanged>2026-08-01T09:00:00Z</LocationChanged></Times>
        <String><Key>Title</Key><Value>Placeholder only</Value></String>
        <String><Key>URL</Key><Value>{REF:A@I:1234ABCD}</Value></String>
      </Entry>
      <Group>
        <UUID>ABEiM0RVZneImaq7zN3u/w==</UUID>
        <Name>Web</Name>
        <EnableSearching>null</EnableSearching>
        <Entry>
          <UUID>UVJTVFVWV1hZWltcXV5fYA==</UUID>
          <Times><CreationTime>2026-08-01T09:00:00Z</CreationTime><LastModificationTime>2026-08-01T10:00:00Z</LastModificationTime><LastAccessTime>2026-08-01T10:00:00Z</LastAccessTime><LocationChanged>2026-08-01T09:00:00Z</LocationChanged></Times>
          <String><Key>Title</Key><Value>Bank</Value></String>
          <String><Key>URL</Key><Value>bank.example.com</Value></String>
          <String><Key>KP2A_URL_1</Key><Value>https://m.bank.example.com/login</Value></String>
          <String><Key>KP2A_URL_2</Key><Value Protected="True">https://secret.bank.example.com/</Value></String>
          <String><Key>Notes</Key><Value>kontonummer 1234</Value></String>
        </Entry>
        <Group>
          <UUID>AQIDBAUGBwgJCgsMDQ4PEA==</UUID>
          <Name>Hidden</Name>
          <EnableSearching>False</EnableSearching>
          <Entry>
            <UUID>YWJjZGVmZ2hpamtsbW5vcA==</UUID>
            <Times><CreationTime>2026-08-01T09:00:00Z</CreationTime><LastModificationTime>2026-08-01T10:00:00Z</LastModificationTime><LastAccessTime>2026-08-01T10:00:00Z</LastAccessTime><LocationChanged>2026-08-01T09:00:00Z</LocationChanged></Times>
            <String><Key>Title</Key><Value>Hidden entry</Value></String>
            <String><Key>URL</Key><Value>https://hidden.example/</Value></String>
          </Entry>
        </Group>
      </Group>
      <Group>
        <UUID>ERITFBUWFxgZGhscHR4fIA==</UUID>
        <Name>Junk</Name>
        <EnableSearching>False</EnableSearching>
        <Group>
          <UUID>ISIjJCUmJygpKissLS4vMA==</UUID>
          <Name>JunkChild</Name>
          <EnableSearching>null</EnableSearching>
          <Entry>
            <UUID>cXJzdHV2d3h5ent8fX5/gA==</UUID>
            <Times><CreationTime>2026-08-01T09:00:00Z</CreationTime><LastModificationTime>2026-08-01T10:00:00Z</LastModificationTime><LastAccessTime>2026-08-01T10:00:00Z</LastAccessTime><LocationChanged>2026-08-01T09:00:00Z</LocationChanged></Times>
            <String><Key>Title</Key><Value>Inherited hidden</Value></String>
            <String><Key>URL</Key><Value>https://junk.example/</Value></String>
          </Entry>
        </Group>
      </Group>
      <Group>
        <UUID>MTIzNDU2Nzg5Ojs8PT4/QA==</UUID>
        <Name>Papirkurv</Name>
        <Entry>
          <UUID>kZKTlJWWl5iZmpucnZ6foA==</UUID>
          <Times><CreationTime>2026-08-01T09:00:00Z</CreationTime><LastModificationTime>2026-08-01T10:00:00Z</LastModificationTime><LastAccessTime>2026-08-01T10:00:00Z</LastAccessTime><LocationChanged>2026-08-01T09:00:00Z</LocationChanged></Times>
          <String><Key>Title</Key><Value>Trashed</Value></String>
          <String><Key>URL</Key><Value>https://trashed.example/</Value></String>
        </Entry>
      </Group>
    </Group>
    <DeletedObjects/>
  </Root>
</KeePassFile>`

func indexByTitle(t *testing.T, idx []indexEntry, title string) indexEntry {
	t.Helper()
	for _, e := range idx {
		if e.Title == title {
			return e
		}
	}
	t.Fatalf("no entry titled %q in index (%d entries)", title, len(idx))
	return indexEntry{}
}

func TestBuildIndex_HidesTrashAndSearchDisabledGroups(t *testing.T) {
	idx, err := buildIndex("privat", []byte(browserFixtureXML))
	if err != nil {
		t.Fatalf("buildIndex: %v", err)
	}

	got := make(map[string]bool, len(idx))
	for _, e := range idx {
		got[e.Title] = true
	}

	for _, want := range []string{"Root entry", "Bank", "Placeholder only"} {
		if !got[want] {
			t.Errorf("entry %q missing from index", want)
		}
	}
	// Papirkurven filtreres af ParseExport; de to andre af groupSearchable —
	// inklusive JunkChild, som arver sin forælders EnableSearching=False.
	for _, unwanted := range []string{"Trashed", "Hidden entry", "Inherited hidden"} {
		if got[unwanted] {
			t.Errorf("entry %q must not be searchable from the browser", unwanted)
		}
	}
}

func TestBuildIndex_URLsAndGroupPath(t *testing.T) {
	idx, err := buildIndex("privat", []byte(browserFixtureXML))
	if err != nil {
		t.Fatalf("buildIndex: %v", err)
	}

	bank := indexByTitle(t, idx, "Bank")
	if bank.Group != "Web" {
		t.Errorf("group path = %q, want %q", bank.Group, "Web")
	}
	if bank.DB != "privat" {
		t.Errorf("db = %q, want %q", bank.DB, "privat")
	}
	// Standardfeltet skal komme først, så udvidelsen kan bruge urls[0]
	// som den primære uden at sortere.
	if len(bank.URLs) != 2 {
		t.Fatalf("urls = %v, want the primary plus one additional", bank.URLs)
	}
	if bank.URLs[0] != "https://bank.example.com" {
		t.Errorf("primary url = %q, want the schemeless value upgraded to https", bank.URLs[0])
	}
	if bank.URLs[1] != "https://m.bank.example.com/login" {
		t.Errorf("additional url = %q", bank.URLs[1])
	}
	for _, u := range bank.URLs {
		if strings.Contains(u, "secret") {
			t.Errorf("protected custom field leaked into the index: %q", u)
		}
	}

	root := indexByTitle(t, idx, "Root entry")
	if root.Group != "" {
		t.Errorf("entry directly in Root should have an empty group path, got %q", root.Group)
	}

	// En entry hvis eneste URL er en placeholder er stadig værd at finde —
	// den kan bare ikke navigeres til.
	ph := indexByTitle(t, idx, "Placeholder only")
	if len(ph.URLs) != 0 {
		t.Errorf("placeholder URL must not be navigable, got %v", ph.URLs)
	}
}

// TestBuildIndex_NoSecretsSerialized er sikkerhedsgrænsen fra
// docs/browser-extension.md udtrykt som en test: uanset hvad databasen
// indeholder, må intet felt ud over titel/URL/placering nå indekset.
func TestBuildIndex_NoSecretsSerialized(t *testing.T) {
	idx, err := buildIndex("privat", []byte(browserFixtureXML))
	if err != nil {
		t.Fatalf("buildIndex: %v", err)
	}
	page, _, err := paginate(idx, 0, pageBytes)
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	blob := ""
	for _, e := range page {
		blob += e.UUID + e.Title + e.Group + e.DB + strings.Join(e.URLs, " ")
	}
	for _, secret := range []string{"hunter2", "hans", "kontonummer"} {
		if strings.Contains(blob, secret) {
			t.Errorf("index contains %q — only title, urls and group may be exposed", secret)
		}
	}
}

// TestBuildIndex_URLsSerialiseAsArray er den test der manglede da fejlen slap
// igennem: `len(urls) == 0` er sandt for både nil og en tom slice, men kun den
// tomme slice bliver til [] i JSON. En nil-slice bliver til null, og udvidelsen
// itererer over feltet.
func TestBuildIndex_URLsSerialiseAsArray(t *testing.T) {
	idx, err := buildIndex("privat", []byte(browserFixtureXML))
	if err != nil {
		t.Fatalf("buildIndex: %v", err)
	}

	ph := indexByTitle(t, idx, "Placeholder only")
	if ph.URLs == nil {
		t.Error("URLs is nil — it will serialise as JSON null and break the extension")
	}

	blob, err := json.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if bytes.Contains(blob, []byte(`"urls":null`)) {
		t.Error(`serialised index contains "urls":null; every entry must carry an array`)
	}
	if !bytes.Contains(blob, []byte(`"urls":[]`)) {
		t.Error(`expected at least one entry with an empty "urls":[] array in the fixture`)
	}
}

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"https://example.com/x", "https://example.com/x", true},
		{"http://example.com", "http://example.com", true},
		{"  https://example.com  ", "https://example.com", true},
		{"example.com/login", "https://example.com/login", true},
		{"", "", false},
		{"{REF:A@I:1234}", "", false},
		{"https://example.com/{S:Token}", "", false},
		{"cmd://open-something", "", false},
		{"javascript:alert(1)", "", false},
		{"ftp://files.example.com", "", false},
		{"//example.com", "", false},
		{"not a url", "", false},
		{"localhost", "", false},
	}
	for _, c := range cases {
		got, ok := normalizeURL(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("normalizeURL(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestIsAdditionalURLField(t *testing.T) {
	yes := []string{"KP2A_URL_1", "kp2a_url", "Additional URL", "additional url 3"}
	no := []string{"URL", "Password", "TOTP Seed", "urls"}
	for _, n := range yes {
		if !isAdditionalURLField(n) {
			t.Errorf("%q should be recognised as an additional URL field", n)
		}
	}
	for _, n := range no {
		if isAdditionalURLField(n) {
			t.Errorf("%q should not be treated as an additional URL field", n)
		}
	}
}

func TestExtractURLs_DeduplicatesAndKeepsPrimaryFirst(t *testing.T) {
	fields := map[string]entryField{
		"URL":        {Value: "example.com"},
		"KP2A_URL_1": {Value: "https://example.com"},
		"KP2A_URL_2": {Value: "https://other.example"},
	}
	got := extractURLs(fields)
	want := []string{"https://example.com", "https://other.example"}
	if len(got) != len(want) {
		t.Fatalf("extractURLs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("extractURLs = %v, want %v", got, want)
		}
	}
}

// TestPaginate_StaysUnderLimit dækker grunden til at indekset overhovedet
// sideinddeles: Firefox afviser beskeder over 1 MB fra applikationen.
func TestPaginate_StaysUnderLimit(t *testing.T) {
	var idx []indexEntry
	for i := 0; i < 500; i++ {
		idx = append(idx, indexEntry{
			UUID:  "41424344-4546-4748-494a-4b4c4d4e4f50",
			Title: strings.Repeat("t", 40),
			URLs:  []string{"https://example.com/" + strings.Repeat("p", 60)},
			Group: "Web/Bank",
			DB:    "privat",
		})
	}

	const limit = 8 * 1024
	seen, offset, pages := 0, 0, 0
	for offset < len(idx) {
		page, next, err := paginate(idx, offset, limit)
		if err != nil {
			t.Fatalf("paginate(%d): %v", offset, err)
		}
		if len(page) == 0 {
			t.Fatalf("paginate(%d) returned an empty page — would loop forever", offset)
		}
		size := 0
		for _, e := range page {
			size += approxJSONSize(e)
		}
		if pages > 0 && size > limit {
			t.Fatalf("page of %d bytes exceeds the %d byte limit", size, limit)
		}
		seen += len(page)
		offset = next
		pages++
	}
	if seen != len(idx) {
		t.Fatalf("paginated %d of %d entries", seen, len(idx))
	}
	if pages < 2 {
		t.Fatalf("expected the fixture to span several pages, got %d", pages)
	}
}

func TestPaginate_RejectsBadOffset(t *testing.T) {
	if _, _, err := paginate(nil, 1, pageBytes); err == nil {
		t.Fatal("expected an error for an out-of-range offset")
	}
}
