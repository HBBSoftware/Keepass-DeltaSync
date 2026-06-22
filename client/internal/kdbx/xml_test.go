// SPDX-License-Identifier: GPL-3.0-or-later

package kdbx

import (
	"testing"
	"time"
)

func TestParseKdbxTime_ISO(t *testing.T) {
	got, err := parseKdbxTime("2026-05-27T10:00:00Z")
	if err != nil {
		t.Fatalf("ISO: %v", err)
	}
	want := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestParseKdbxTime_KDBX4Binary(t *testing.T) {
	// "HtWo4Q4AAAA=" is a real timestamp from a KDBX4 export — verifies the
	// little-endian int64 = seconds since 0001-01-01 path.
	got, err := parseKdbxTime("HtWo4Q4AAAA=")
	if err != nil {
		t.Fatalf("binary: %v", err)
	}
	// Sanity: should be in 2026 (KeePassXC's epoch + ~2026 years of seconds).
	if got.Year() != 2026 {
		t.Fatalf("expected year 2026, got %d (full time: %v)", got.Year(), got)
	}
}

func TestParseKdbxTime_RoundTripBinary(t *testing.T) {
	// Encoding a known time as KDBX4 binary, parsing it back, should match.
	want := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	secs := uint64(want.Unix() - kdbx4EpochToUnixOffset)
	buf := make([]byte, 8)
	for i := 0; i < 8; i++ {
		buf[i] = byte(secs >> (8 * i))
	}
	// base64-encode to mimic what KeePassXC's XML produces
	encoded := stdBase64Encode(buf)
	got, err := parseKdbxTime(encoded)
	if err != nil {
		t.Fatalf("round-trip: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("round-trip mismatch: got %v want %v (encoded %q)", got, want, encoded)
	}
}

func TestParseKdbxTime_BadInput(t *testing.T) {
	for _, bad := range []string{"", "not-a-time", "definitely_not_base64_or_iso"} {
		if _, err := parseKdbxTime(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

// recycleBinXML builder en minimal KeePassFile-XML med konfigurerbar Meta og
// to entries — én i recycle bin, én i Root. Bruges af recycle-bin-testene.
func recycleBinXML(recycleEnabled, recycleUUID string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<KeePassFile>
  <Meta>
    <RecycleBinEnabled>` + recycleEnabled + `</RecycleBinEnabled>
    <RecycleBinUUID>` + recycleUUID + `</RecycleBinUUID>
  </Meta>
  <Root>
    <Group>
      <UUID>2w8iR46R3u2YWzT0zjtpA==</UUID>
      <Name>Root</Name>
      <Entry>
        <UUID>AAECAwQFBgcICQoLDA0ODw==</UUID>
        <Times>
          <LastModificationTime>2026-05-28T10:00:00Z</LastModificationTime>
          <LocationChanged>2026-05-28T10:00:00Z</LocationChanged>
        </Times>
        <String>
          <Key>Title</Key>
          <Value>alive</Value>
        </String>
      </Entry>
      <Group>
        <UUID>p06hnoEmTfeRpUWM1a14cw==</UUID>
        <Name>Papirkurv</Name>
        <Entry>
          <UUID>EBESExQVFhcYGRobHB0eHw==</UUID>
          <Times>
            <LastModificationTime>2026-05-28T11:00:00Z</LastModificationTime>
            <LocationChanged>2026-05-28T11:30:00Z</LocationChanged>
          </Times>
          <String>
            <Key>Title</Key>
            <Value>trashed</Value>
          </String>
        </Entry>
      </Group>
    </Group>
    <DeletedObjects/>
  </Root>
</KeePassFile>`
}

func TestParseExport_RecycleBinSynthesizesDeletion(t *testing.T) {
	xml := recycleBinXML("True", "p06hnoEmTfeRpUWM1a14cw==")
	entries, _, deletions, err := ParseExport([]byte(xml))
	if err != nil {
		t.Fatalf("ParseExport: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 live entry, got %d", len(entries))
	}
	if len(deletions) != 1 {
		t.Fatalf("expected 1 synthetic deletion, got %d", len(deletions))
	}
	// Den trashede entry skal være den synthetic deletion.
	wantTrashedUUID := "10111213-1415-1617-1819-1a1b1c1d1e1f"
	if deletions[0].UUID != wantTrashedUUID {
		t.Fatalf("deletion uuid = %q, want %q", deletions[0].UUID, wantTrashedUUID)
	}
	// DeletedAt skal være LocationChanged (11:30), ikke LastModificationTime (11:00).
	wantDeletedAt := time.Date(2026, 5, 28, 11, 30, 0, 0, time.UTC)
	if !deletions[0].DeletedAt.Equal(wantDeletedAt) {
		t.Fatalf("DeletedAt = %v, want %v (should be LocationChanged, not LastModificationTime)", deletions[0].DeletedAt, wantDeletedAt)
	}
}

func TestParseExport_RecycleBinDisabledNoSynthesis(t *testing.T) {
	xml := recycleBinXML("False", "p06hnoEmTfeRpUWM1a14cw==")
	entries, _, deletions, err := ParseExport([]byte(xml))
	if err != nil {
		t.Fatalf("ParseExport: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries when recycle bin disabled (no synthesis), got %d", len(entries))
	}
	if len(deletions) != 0 {
		t.Fatalf("expected 0 deletions, got %d", len(deletions))
	}
}

func TestParseExport_NullRecycleBinUUIDNoSynthesis(t *testing.T) {
	// Recycle bin enabled, men UUID er null-sentinel — gruppen er aldrig
	// blevet materialiseret, så ingen entries er i den. Vi må ikke
	// uagtsomt klassificere noget som slettet.
	xml := recycleBinXML("True", nullKdbxUUID)
	entries, _, deletions, err := ParseExport([]byte(xml))
	if err != nil {
		t.Fatalf("ParseExport: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries when RecycleBinUUID is null, got %d", len(entries))
	}
	if len(deletions) != 0 {
		t.Fatalf("expected 0 deletions, got %d", len(deletions))
	}
}

func TestParseExport_RecycleBinUUIDMustMatchNotJustName(t *testing.T) {
	// Gruppe hedder "Papirkurv" men har et ANDET UUID end RecycleBinUUID.
	// Vi må kun synthesize på UUID-match — ikke navne-match.
	xml := recycleBinXML("True", "deadbeefdeadbeefdeadbeefdead==")
	entries, _, deletions, err := ParseExport([]byte(xml))
	if err != nil {
		t.Fatalf("ParseExport: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries when RecycleBinUUID doesn't match group, got %d", len(entries))
	}
	if len(deletions) != 0 {
		t.Fatalf("expected 0 deletions, got %d", len(deletions))
	}
}

// groupTreeXML: Root med en entry, en undergruppe "Work" (med en entry og en
// under-undergruppe "Work/Sub" der også har en entry). Ingen recycle bin.
// UUID'er: Root=AAAA..., Work=EBES...(10111213-...), Sub=ICEi...(20212223-...).
func groupTreeXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<KeePassFile>
  <Meta><RecycleBinEnabled>False</RecycleBinEnabled><RecycleBinUUID>` + nullKdbxUUID + `</RecycleBinUUID></Meta>
  <Root>
    <Group>
      <UUID>AAAAAAAAAAAAAAAAAAAAAA==</UUID>
      <Name>Root</Name>
      <Entry>
        <UUID>AAECAwQFBgcICQoLDA0ODw==</UUID>
        <Times><LastModificationTime>2026-05-28T10:00:00Z</LastModificationTime></Times>
      </Entry>
      <Group>
        <UUID>EBESExQVFhcYGRobHB0eHw==</UUID>
        <Name>Work</Name>
        <IconID>48</IconID>
        <Times>
          <LastModificationTime>2026-05-28T12:00:00Z</LastModificationTime>
          <LocationChanged>2026-05-28T12:30:00Z</LocationChanged>
        </Times>
        <Entry>
          <UUID>ICEiIyQlJicoKSorLC0uLw==</UUID>
          <Times><LastModificationTime>2026-05-28T13:00:00Z</LastModificationTime></Times>
        </Entry>
        <Group>
          <UUID>MDEyMzQ1Njc4OTo7PD0+Pw==</UUID>
          <Name>Sub</Name>
          <Times><LastModificationTime>2026-05-28T14:00:00Z</LastModificationTime></Times>
          <Entry>
            <UUID>QEFCQ0RFRkdISUpLTE1OTw==</UUID>
            <Times><LastModificationTime>2026-05-28T15:00:00Z</LastModificationTime></Times>
          </Entry>
        </Group>
      </Group>
    </Group>
    <DeletedObjects/>
  </Root>
</KeePassFile>`
}

func TestParseExport_CollectsGroupsAndParents(t *testing.T) {
	entries, groups, _, err := ParseExport([]byte(groupTreeXML()))
	if err != nil {
		t.Fatalf("ParseExport: %v", err)
	}

	// 3 entries: én i Root, én i Work, én i Work/Sub.
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// 2 grupper: Work og Sub (Root emittes ikke).
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	rootEntry := "00010203-0405-0607-0809-0a0b0c0d0e0f"
	workUUID := "10111213-1415-1617-1819-1a1b1c1d1e1f"
	workEntry := "20212223-2425-2627-2829-2a2b2c2d2e2f"
	subUUID := "30313233-3435-3637-3839-3a3b3c3d3e3f"
	subEntry := "40414243-4445-4647-4849-4a4b4c4d4e4f"

	parentOf := map[string]string{}
	for _, e := range entries {
		parentOf[e.UUID] = e.ParentGroupUUID
	}
	// Root-entry: sentinel "".
	if parentOf[rootEntry] != "" {
		t.Errorf("root entry parent = %q, want \"\" (sentinel)", parentOf[rootEntry])
	}
	if parentOf[workEntry] != workUUID {
		t.Errorf("work entry parent = %q, want %q", parentOf[workEntry], workUUID)
	}
	if parentOf[subEntry] != subUUID {
		t.Errorf("sub entry parent = %q, want %q", parentOf[subEntry], subUUID)
	}

	byUUID := map[string]Group{}
	for _, g := range groups {
		byUUID[g.UUID] = g
	}
	// Work: parent = Root-sentinel "", navn + ikon + tider parset.
	work, ok := byUUID[workUUID]
	if !ok {
		t.Fatalf("Work group not collected")
	}
	if work.ParentUUID != "" {
		t.Errorf("Work parent = %q, want \"\" (Root sentinel)", work.ParentUUID)
	}
	if work.Name != "Work" {
		t.Errorf("Work name = %q", work.Name)
	}
	if work.IconID != 48 {
		t.Errorf("Work icon = %d, want 48", work.IconID)
	}
	if want := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC); !work.ModifiedAt.Equal(want) {
		t.Errorf("Work modified = %v, want %v", work.ModifiedAt, want)
	}
	// Sub: parent = Work.
	sub, ok := byUUID[subUUID]
	if !ok {
		t.Fatalf("Sub group not collected")
	}
	if sub.ParentUUID != workUUID {
		t.Errorf("Sub parent = %q, want %q", sub.ParentUUID, workUUID)
	}
}

// stdBase64Encode is a tiny inline helper to avoid pulling in encoding/base64
// at the top of the test file when this is the only use.
func stdBase64Encode(b []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var out []byte
	for i := 0; i < len(b); i += 3 {
		end := i + 3
		if end > len(b) {
			end = len(b)
		}
		chunk := b[i:end]
		var n uint32
		for j := 0; j < 3; j++ {
			n <<= 8
			if j < len(chunk) {
				n |= uint32(chunk[j])
			}
		}
		out = append(out, alphabet[(n>>18)&0x3F], alphabet[(n>>12)&0x3F])
		if len(chunk) > 1 {
			out = append(out, alphabet[(n>>6)&0x3F])
		} else {
			out = append(out, '=')
		}
		if len(chunk) > 2 {
			out = append(out, alphabet[n&0x3F])
		} else {
			out = append(out, '=')
		}
	}
	return string(out)
}
