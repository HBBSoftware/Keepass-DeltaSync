// SPDX-License-Identifier: GPL-3.0-or-later

package kdbx

import (
	"testing"
	"time"
)

// deletedGroupXML: Root med én levende entry, og en papirkurv der indeholder en
// slettet gruppe "Slettet" — med en entry OG en undergruppe "Slettet/Under"
// der selv har en entry. Det er formen KeePass efterlader når man sletter en
// ikke-tom mappe: hele undertræet flyttes ned i papirkurven.
//
// UUID'er: Root=AAAA…, Papirkurv=p06h…, Slettet=AQID…(01020304-…),
// Under=ICEi…(20212223-…), entries=EBES…(10111213-…) og QEFC…(40414243-…).
func deletedGroupXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<KeePassFile>
  <Meta>
    <RecycleBinEnabled>True</RecycleBinEnabled>
    <RecycleBinUUID>p06hnoEmTfeRpUWM1a14cw==</RecycleBinUUID>
  </Meta>
  <Root>
    <Group>
      <UUID>AAAAAAAAAAAAAAAAAAAAAA==</UUID>
      <Name>Root</Name>
      <Entry>
        <UUID>AAECAwQFBgcICQoLDA0ODw==</UUID>
        <Times><LastModificationTime>2026-05-28T10:00:00Z</LastModificationTime></Times>
      </Entry>
      <Group>
        <UUID>p06hnoEmTfeRpUWM1a14cw==</UUID>
        <Name>Papirkurv</Name>
        <Group>
          <UUID>AQIDBAUGBwgJCgsMDQ4PEA==</UUID>
          <Name>Slettet</Name>
          <Times>
            <LastModificationTime>2026-05-28T11:00:00Z</LastModificationTime>
            <LocationChanged>2026-05-28T11:30:00Z</LocationChanged>
          </Times>
          <Entry>
            <UUID>EBESExQVFhcYGRobHB0eHw==</UUID>
            <Times>
              <LastModificationTime>2026-05-28T11:00:00Z</LastModificationTime>
              <LocationChanged>2026-05-28T11:30:00Z</LocationChanged>
            </Times>
          </Entry>
          <Group>
            <UUID>ICEiIyQlJicoKSorLC0uLw==</UUID>
            <Name>Under</Name>
            <Times><LastModificationTime>2026-05-28T11:00:00Z</LastModificationTime></Times>
            <Entry>
              <UUID>QEFCQ0RFRkdISUpLTE1OTw==</UUID>
              <Times>
                <LastModificationTime>2026-05-28T11:00:00Z</LastModificationTime>
                <LocationChanged>2026-05-28T11:30:00Z</LocationChanged>
              </Times>
            </Entry>
          </Group>
        </Group>
      </Group>
    </Group>
    <DeletedObjects/>
  </Root>
</KeePassFile>`
}

// Kernen i rettelsen: entries i en slettet MAPPE ligger i en undergruppe af
// papirkurven, ikke direkte i den. Før arvede "vi er i papirkurven" ikke nedad,
// så de blev opsamlet som levende entries med en forældergruppe der samtidig
// blev tombstonet — og dukkede op i roden på alle andre enheder.
func TestParseExport_DeletedGroupSubtreeIsTombstoned(t *testing.T) {
	exp, err := ParseExportFull([]byte(deletedGroupXML()))
	if err != nil {
		t.Fatalf("ParseExportFull: %v", err)
	}

	if len(exp.Entries) != 1 {
		t.Fatalf("expected 1 live entry (kun Root's), got %d: %+v", len(exp.Entries), exp.Entries)
	}
	if exp.Entries[0].UUID != "00010203-0405-0607-0809-0a0b0c0d0e0f" {
		t.Errorf("live entry = %s, want Root-entry'en", exp.Entries[0].UUID)
	}

	// Ingen af grupperne i papirkurven må emitteres som levende grupper —
	// gør de det, holder de sig selv i live på serveren for evigt.
	if len(exp.Groups) != 0 {
		t.Fatalf("expected 0 live groups, got %d: %+v", len(exp.Groups), exp.Groups)
	}

	// Begge entries i det slettede undertræ skal være tombstones, med
	// LocationChanged som slette-tidspunkt.
	wantDeleted := map[string]bool{
		"10111213-1415-1617-1819-1a1b1c1d1e1f": false,
		"40414243-4445-4647-4849-4a4b4c4d4e4f": false,
	}
	for _, d := range exp.Deletions {
		if _, ok := wantDeleted[d.UUID]; !ok {
			t.Errorf("uventet tombstone %s", d.UUID)
			continue
		}
		wantDeleted[d.UUID] = true
		want := time.Date(2026, 5, 28, 11, 30, 0, 0, time.UTC)
		if !d.DeletedAt.Equal(want) {
			t.Errorf("%s: DeletedAt = %v, want %v (LocationChanged)", d.UUID, d.DeletedAt, want)
		}
	}
	for uuid, seen := range wantDeleted {
		if !seen {
			t.Errorf("entry %s i slettet gruppe blev ikke tombstonet", uuid)
		}
	}
}

// Grupperne i papirkurven skal kunne genkendes af push-siden, så
// sikkerhedsspærren kan skelne "brugeren slettede mappen" fra "mappen
// forsvandt uden spor". Papirkurven SELV må aldrig med — den er lokal.
func TestParseExport_TrashedGroupsReported(t *testing.T) {
	exp, err := ParseExportFull([]byte(deletedGroupXML()))
	if err != nil {
		t.Fatalf("ParseExportFull: %v", err)
	}
	want := map[string]bool{
		"01020304-0506-0708-090a-0b0c0d0e0f10": false, // Slettet
		"20212223-2425-2627-2829-2a2b2c2d2e2f": false, // Slettet/Under
	}
	for _, uuid := range exp.TrashedGroups {
		if uuid == "a74ea19e-8126-4df7-91a5-458cd5ad7873" {
			t.Errorf("papirkurven selv rapporteret som trashed group")
		}
		if _, ok := want[uuid]; !ok {
			t.Errorf("uventet trashed group %s", uuid)
			continue
		}
		want[uuid] = true
	}
	for uuid, seen := range want {
		if !seen {
			t.Errorf("gruppe %s i papirkurven mangler i TrashedGroups", uuid)
		}
	}
}

// Modprøve: uden papirkurv er intet trashed, og hele træet er levende.
func TestParseExport_NoRecycleBinNothingTrashed(t *testing.T) {
	exp, err := ParseExportFull([]byte(groupTreeXML()))
	if err != nil {
		t.Fatalf("ParseExportFull: %v", err)
	}
	if len(exp.TrashedGroups) != 0 {
		t.Errorf("expected no trashed groups, got %v", exp.TrashedGroups)
	}
	if len(exp.Groups) != 2 || len(exp.Entries) != 3 {
		t.Errorf("træet skal være uændret: %d grupper, %d entries", len(exp.Groups), len(exp.Entries))
	}
}
