// SPDX-License-Identifier: GPL-3.0-or-later

package canonical

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"
)

func sampleGroup() *Group {
	return &Group{
		V:           GroupSchemaVersion,
		UUID:        "11112222-3333-4444-5555-666677778888",
		Name:        "Work",
		Notes:       "Arbejdsrelaterede logins",
		ParentGroup: "00010203-0405-0607-0809-0a0b0c0d0e0f",
		IconID:      48,
		Times: Times{
			Created:         time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			Modified:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			Accessed:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			LocationChanged: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
		},
	}
}

func TestGroupEnvelopeRoundTrip(t *testing.T) {
	original := sampleGroup()
	encoded, err := EncodeGroup(original)
	if err != nil {
		t.Fatalf("EncodeGroup: %v", err)
	}
	if encoded[0] != formatByteGroup {
		t.Errorf("encoded[0] = 0x%02x, want 0x%02x", encoded[0], formatByteGroup)
	}
	if got := DetectFormat(encoded); got != FormatGroup {
		t.Errorf("DetectFormat(encoded) = %v, want FormatGroup", got)
	}

	decoded, err := DecodeGroup(encoded)
	if err != nil {
		t.Fatalf("DecodeGroup: %v", err)
	}
	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("round-trip mismatch:\nwant: %+v\ngot:  %+v", original, decoded)
	}
}

func TestDetectFormat_Group(t *testing.T) {
	if got := DetectFormat([]byte{formatByteGroup, '{', '}'}); got != FormatGroup {
		t.Errorf("DetectFormat(group byte) = %v, want FormatGroup", got)
	}
}

func TestEncodeGroup_ForcesSchemaVersion(t *testing.T) {
	g := sampleGroup()
	g.V = 999
	if _, err := EncodeGroup(g); err != nil {
		t.Fatalf("EncodeGroup: %v", err)
	}
	if g.V != GroupSchemaVersion {
		t.Errorf("Group.V = %d, want %d (Encode should normalize)", g.V, GroupSchemaVersion)
	}
}

func TestEncodeGroup_NilGroup(t *testing.T) {
	if _, err := EncodeGroup(nil); err == nil {
		t.Error("expected error for nil group")
	}
}

func TestDecodeGroup_RejectsEntryByte(t *testing.T) {
	// En entry-blob (0x01) må ikke dekodes som gruppe — caller har fejldispatched.
	entryBlob, err := EncodeCanonical(sampleEntry())
	if err != nil {
		t.Fatalf("EncodeCanonical: %v", err)
	}
	if _, err := DecodeGroup(entryBlob); err == nil {
		t.Error("expected error decoding entry blob as group")
	}
}

func TestDecodeGroup_RejectsFutureSchema(t *testing.T) {
	future := append([]byte{formatByteGroup}, []byte(`{"v":999,"uuid":"11112222-3333-4444-5555-666677778888","name":"X","icon_id":0,"times":{"created":"2026-05-01T10:00:00Z","modified":"2026-05-01T10:00:00Z","accessed":"2026-05-01T10:00:00Z","expires":false,"usage_count":0,"location_changed":"2026-05-01T10:00:00Z"}}`)...)
	_, err := DecodeGroup(future)
	if err == nil {
		t.Fatal("expected error decoding future group schema")
	}
	if !strings.Contains(err.Error(), "newer than supported") {
		t.Errorf("error = %q, want version-mismatch message", err)
	}
}

// TestEntryParentGroup_RoundTrips sikrer at det nye parent_group-felt overlever
// canonical encode/decode.
func TestEntryParentGroup_RoundTrips(t *testing.T) {
	e := sampleEntry()
	e.ParentGroup = "11112222-3333-4444-5555-666677778888"
	encoded, err := EncodeCanonical(e)
	if err != nil {
		t.Fatalf("EncodeCanonical: %v", err)
	}
	decoded, err := DecodeCanonical(encoded)
	if err != nil {
		t.Fatalf("DecodeCanonical: %v", err)
	}
	if decoded.ParentGroup != e.ParentGroup {
		t.Errorf("ParentGroup = %q, want %q", decoded.ParentGroup, e.ParentGroup)
	}
}

// TestEntryParentGroup_OmittedWhenEmpty er backward-compat-garantien: en entry
// uden gruppe-placering (sentinel = tom string) serialiserer IDENTISK med før
// v4, så v1-klienter og golden fixtures er upåvirkede.
func TestEntryParentGroup_OmittedWhenEmpty(t *testing.T) {
	encoded, err := EncodeCanonical(sampleEntry()) // ingen ParentGroup sat
	if err != nil {
		t.Fatalf("EncodeCanonical: %v", err)
	}
	if bytes.Contains(encoded, []byte("parent_group")) {
		t.Error("empty ParentGroup should be omitted from JSON (omitempty), but 'parent_group' is present")
	}
}
