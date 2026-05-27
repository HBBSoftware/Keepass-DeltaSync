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
