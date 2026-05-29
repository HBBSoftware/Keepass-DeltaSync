// SPDX-License-Identifier: GPL-3.0-or-later

package canonical

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func sampleEntry() *Entry {
	return &Entry{
		V:    SchemaVersion,
		UUID: "00010203-0405-0607-0809-0a0b0c0d0e0f",
		Times: Times{
			Created:         time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			Modified:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			Accessed:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			LocationChanged: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
		},
		Strings: map[string]String{
			"Title":    {V: "Sample"},
			"Password": {V: "hunter2", Protected: true},
		},
	}
}

func TestDetectFormat(t *testing.T) {
	cases := []struct {
		name  string
		input []byte
		want  Format
	}{
		{"canonical byte", []byte{0x01, 0xff, 0xff}, FormatCanonical},
		{"legacy XML", []byte("<UUID>foo</UUID>"), FormatLegacyXML},
		{"empty", []byte{}, FormatUnknown},
		{"nil", nil, FormatUnknown},
		{"unknown first byte", []byte{0x42, 0xff}, FormatUnknown},
		{"zero byte", []byte{0x00, 0xff}, FormatUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectFormat(tc.input); got != tc.want {
				t.Errorf("DetectFormat(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestEncodeCanonical_PrefixByte(t *testing.T) {
	encoded, err := EncodeCanonical(sampleEntry())
	if err != nil {
		t.Fatalf("EncodeCanonical: %v", err)
	}
	if len(encoded) < 2 {
		t.Fatalf("encoded too short: %d bytes", len(encoded))
	}
	if encoded[0] != formatByteCanonical {
		t.Errorf("encoded[0] = 0x%02x, want 0x%02x", encoded[0], formatByteCanonical)
	}
	if encoded[1] != '{' {
		t.Errorf("encoded[1] = 0x%02x (%q), want '{' (JSON object opening)", encoded[1], encoded[1])
	}
}

func TestEncodeCanonical_ForcesSchemaVersion(t *testing.T) {
	e := sampleEntry()
	e.V = 999 // caller-supplied bogus version
	encoded, err := EncodeCanonical(e)
	if err != nil {
		t.Fatalf("EncodeCanonical: %v", err)
	}
	if e.V != SchemaVersion {
		t.Errorf("Entry.V mutated to %d, want %d (Encode should normalize)", e.V, SchemaVersion)
	}
	decoded, err := DecodeCanonical(encoded)
	if err != nil {
		t.Fatalf("DecodeCanonical: %v", err)
	}
	if decoded.V != SchemaVersion {
		t.Errorf("decoded.V = %d, want %d", decoded.V, SchemaVersion)
	}
}

func TestEnvelopeRoundTrip(t *testing.T) {
	original := sampleEntry()
	encoded, err := EncodeCanonical(original)
	if err != nil {
		t.Fatalf("EncodeCanonical: %v", err)
	}

	if got := DetectFormat(encoded); got != FormatCanonical {
		t.Errorf("DetectFormat(encoded) = %v, want FormatCanonical", got)
	}

	decoded, err := DecodeCanonical(encoded)
	if err != nil {
		t.Fatalf("DecodeCanonical: %v", err)
	}
	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("round-trip mismatch:\nwant: %+v\ngot:  %+v", original, decoded)
	}
}

func TestEncodeCanonical_NilEntry(t *testing.T) {
	if _, err := EncodeCanonical(nil); err == nil {
		t.Error("expected error for nil entry")
	}
}

func TestDecodeCanonical_RejectsLegacyXML(t *testing.T) {
	// Legacy XML-fragment skal afvises af DecodeCanonical — caller har
	// fejldispatched (skulle have brugt FromInnerXML).
	xmlBytes := []byte("<UUID>AAECAwQFBgcICQoLDA0ODw==</UUID>")
	_, err := DecodeCanonical(xmlBytes)
	if err == nil {
		t.Error("expected error decoding legacy XML as canonical")
	}
	if !strings.Contains(err.Error(), "format byte") {
		t.Errorf("error = %q, want it to mention format byte", err)
	}
}

func TestDecodeCanonical_RejectsTooShort(t *testing.T) {
	for _, bad := range [][]byte{
		nil,
		{},
		{0x01}, // just the format byte, no payload
	} {
		if _, err := DecodeCanonical(bad); err == nil {
			t.Errorf("expected error for input %v", bad)
		}
	}
}

func TestDecodeCanonical_RejectsFutureSchema(t *testing.T) {
	// Hånd-konstrueret blob med v=999 — fremtidig skema, vi kan ikke
	// håndtere den. Caller bør se en klar besked.
	future := append([]byte{formatByteCanonical}, []byte(`{"v":999,"uuid":"00010203-0405-0607-0809-0a0b0c0d0e0f","times":{"created":"2026-05-01T10:00:00Z","modified":"2026-05-29T10:00:00Z","accessed":"2026-05-29T10:00:00Z","expires":false,"usage_count":0,"location_changed":"2026-05-01T10:00:00Z"},"icon_id":0}`)...)
	_, err := DecodeCanonical(future)
	if err == nil {
		t.Fatal("expected error decoding future schema version")
	}
	if !strings.Contains(err.Error(), "newer than supported") {
		t.Errorf("error = %q, want it to mention version mismatch", err)
	}
}

func TestDecodeCanonical_RejectsMissingVersion(t *testing.T) {
	// JSON uden v-felt — schema-version skal være sat.
	bad := append([]byte{formatByteCanonical}, []byte(`{"uuid":"00010203-0405-0607-0809-0a0b0c0d0e0f","times":{"created":"2026-05-01T10:00:00Z","modified":"2026-05-29T10:00:00Z","accessed":"2026-05-29T10:00:00Z","expires":false,"usage_count":0,"location_changed":"2026-05-01T10:00:00Z"},"icon_id":0}`)...)
	if _, err := DecodeCanonical(bad); err == nil {
		t.Error("expected error for missing schema version")
	}
}

// TestEndToEnd_FullChain demonstrerer hele wire-path'en:
// fragment (legacy InnerXML) → canonical → JSON+envelope → detection →
// decode → ToInnerXML → re-parse. Det er den path både push og pull
// kommer til at gå igennem efter B2/B3.
func TestEndToEnd_FullChain(t *testing.T) {
	// 1. Start med InnerXML-fragment (det push-pipeline'en får fra ParseExport).
	first, err := FromInnerXML([]byte(realisticFragment))
	if err != nil {
		t.Fatalf("FromInnerXML: %v", err)
	}

	// 2. Push side: encode til wire-format.
	plaintext, err := EncodeCanonical(first)
	if err != nil {
		t.Fatalf("EncodeCanonical: %v", err)
	}

	// 3. (Krypto skipped — det er allerede testet i internal/crypto.)
	// Pretend dette er hvad vi har efter DecryptBlob på pull-siden.

	// 4. Pull side: detect format, dispatch.
	switch DetectFormat(plaintext) {
	case FormatCanonical:
		// expected path
	default:
		t.Fatalf("DetectFormat = %v, want FormatCanonical", DetectFormat(plaintext))
	}

	decoded, err := DecodeCanonical(plaintext)
	if err != nil {
		t.Fatalf("DecodeCanonical: %v", err)
	}

	// 5. Render tilbage til InnerXML for staging-pipeline.
	fragment, err := ToInnerXML(decoded)
	if err != nil {
		t.Fatalf("ToInnerXML: %v", err)
	}

	// 6. Re-parse skal give samme canonical struct.
	reparsed, err := FromInnerXML(fragment)
	if err != nil {
		t.Fatalf("FromInnerXML re-parse: %v", err)
	}
	if !reflect.DeepEqual(first, reparsed) {
		t.Errorf("end-to-end mismatch:\nfirst:    %+v\nreparsed: %+v", first, reparsed)
	}
}
