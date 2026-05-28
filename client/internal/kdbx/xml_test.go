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
	entries, deletions, err := ParseExport([]byte(xml))
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
	entries, deletions, err := ParseExport([]byte(xml))
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
	entries, deletions, err := ParseExport([]byte(xml))
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
	entries, deletions, err := ParseExport([]byte(xml))
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
