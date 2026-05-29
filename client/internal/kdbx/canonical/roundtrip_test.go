// SPDX-License-Identifier: GPL-3.0-or-later

package canonical

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

// realisticFragment er et håndsamlet InnerXML-fragment, der ligner det
// keepassxc-cli's export producerer for en typisk entry. Bruges som
// canonical fixture i flere tests.
const realisticFragment = `<UUID>AAECAwQFBgcICQoLDA0ODw==</UUID>` +
	`<IconID>0</IconID>` +
	`<ForegroundColor></ForegroundColor>` +
	`<BackgroundColor></BackgroundColor>` +
	`<OverrideURL></OverrideURL>` +
	`<Tags>work;important</Tags>` +
	`<Times>` +
	`<CreationTime>2026-05-01T10:00:00Z</CreationTime>` +
	`<LastModificationTime>2026-05-29T10:00:00Z</LastModificationTime>` +
	`<LastAccessTime>2026-05-29T10:00:00Z</LastAccessTime>` +
	`<ExpiryTime>2026-05-29T10:00:00Z</ExpiryTime>` +
	`<Expires>False</Expires>` +
	`<UsageCount>5</UsageCount>` +
	`<LocationChanged>2026-05-01T10:00:00Z</LocationChanged>` +
	`</Times>` +
	`<String><Key>Title</Key><Value>GitLab</Value></String>` +
	`<String><Key>UserName</Key><Value>hans</Value></String>` +
	`<String><Key>Password</Key><Value Protected="True">s3cr3t&amp;p4ss</Value></String>` +
	`<String><Key>URL</Key><Value>https://gitlab.com</Value></String>` +
	`<String><Key>Notes</Key><Value></Value></String>` +
	`<String><Key>API-Token</Key><Value Protected="True">abc123</Value></String>` +
	`<AutoType>` +
	`<Enabled>True</Enabled>` +
	`<DataTransferObfuscation>0</DataTransferObfuscation>` +
	`<DefaultSequence>{USERNAME}{TAB}{PASSWORD}{ENTER}</DefaultSequence>` +
	`</AutoType>`

func TestFromInnerXML_Basic(t *testing.T) {
	e, err := FromInnerXML([]byte(realisticFragment))
	if err != nil {
		t.Fatalf("FromInnerXML: %v", err)
	}

	if e.UUID != "00010203-0405-0607-0809-0a0b0c0d0e0f" {
		t.Errorf("UUID = %q, want 00010203-0405-0607-0809-0a0b0c0d0e0f", e.UUID)
	}
	if e.IconID != 0 {
		t.Errorf("IconID = %d, want 0", e.IconID)
	}
	if !reflect.DeepEqual(e.Tags, []string{"work", "important"}) {
		t.Errorf("Tags = %v, want [work important]", e.Tags)
	}

	if got := e.Strings["Title"]; got.V != "GitLab" || got.Protected {
		t.Errorf("Title = %+v, want {GitLab false}", got)
	}
	if got := e.Strings["Password"]; got.V != "s3cr3t&p4ss" || !got.Protected {
		t.Errorf("Password = %+v, want {s3cr3t&p4ss true}", got)
	}
	if got := e.Strings["API-Token"]; got.V != "abc123" || !got.Protected {
		t.Errorf("API-Token = %+v, want {abc123 true}", got)
	}
	if got := e.Strings["Notes"]; got.V != "" || got.Protected {
		t.Errorf("Notes = %+v, want {\"\" false}", got)
	}

	if e.AutoType == nil {
		t.Fatal("AutoType nil, want non-nil")
	}
	if !e.AutoType.Enabled {
		t.Error("AutoType.Enabled = false, want true")
	}
	if e.AutoType.DefaultSequence != "{USERNAME}{TAB}{PASSWORD}{ENTER}" {
		t.Errorf("AutoType.DefaultSequence = %q", e.AutoType.DefaultSequence)
	}

	if e.Times.Expires {
		t.Error("Expires = true, want false")
	}
	if e.Times.ExpiresAt != nil {
		t.Errorf("ExpiresAt = %v, want nil when Expires=false", e.Times.ExpiresAt)
	}
	if e.Times.UsageCount != 5 {
		t.Errorf("UsageCount = %d, want 5", e.Times.UsageCount)
	}
}

func TestRoundTrip_XML(t *testing.T) {
	// Parse → emit → parse-igen. Resultatet skal være struct-equal med
	// første parse.
	first, err := FromInnerXML([]byte(realisticFragment))
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}

	emitted, err := ToInnerXML(first)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}

	second, err := FromInnerXML(emitted)
	if err != nil {
		t.Fatalf("re-parse failed (emitted output is not valid InnerXML): %v\n%s", err, emitted)
	}

	if !reflect.DeepEqual(first, second) {
		t.Errorf("round-trip mismatch:\nfirst:  %+v\nsecond: %+v", first, second)
	}
}

func TestRoundTrip_JSON(t *testing.T) {
	// Parse → JSON → struct-decode → reflect.DeepEqual.
	first, err := FromInnerXML([]byte(realisticFragment))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	raw, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var second Entry
	if err := json.Unmarshal(raw, &second); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(first, &second) {
		t.Errorf("json round-trip mismatch:\nfirst:  %+v\nsecond: %+v", first, &second)
	}
}

func TestRoundTrip_WithHistory(t *testing.T) {
	// Entry med history-undertræ — verificer at history-entries
	// round-tripper og at nested history bliver strippet defensivt.
	fragment := `<UUID>AAECAwQFBgcICQoLDA0ODw==</UUID>` +
		`<IconID>0</IconID>` +
		`<ForegroundColor></ForegroundColor>` +
		`<BackgroundColor></BackgroundColor>` +
		`<OverrideURL></OverrideURL>` +
		`<Tags></Tags>` +
		`<Times>` +
		`<CreationTime>2026-05-01T10:00:00Z</CreationTime>` +
		`<LastModificationTime>2026-05-29T10:00:00Z</LastModificationTime>` +
		`<LastAccessTime>2026-05-29T10:00:00Z</LastAccessTime>` +
		`<ExpiryTime>2026-05-29T10:00:00Z</ExpiryTime>` +
		`<Expires>False</Expires>` +
		`<UsageCount>0</UsageCount>` +
		`<LocationChanged>2026-05-01T10:00:00Z</LocationChanged>` +
		`</Times>` +
		`<String><Key>Title</Key><Value>v2</Value></String>` +
		`<History>` +
		`<Entry>` +
		`<UUID>AAECAwQFBgcICQoLDA0ODw==</UUID>` +
		`<IconID>0</IconID>` +
		`<ForegroundColor></ForegroundColor>` +
		`<BackgroundColor></BackgroundColor>` +
		`<OverrideURL></OverrideURL>` +
		`<Tags></Tags>` +
		`<Times>` +
		`<CreationTime>2026-05-01T10:00:00Z</CreationTime>` +
		`<LastModificationTime>2026-05-15T10:00:00Z</LastModificationTime>` +
		`<LastAccessTime>2026-05-15T10:00:00Z</LastAccessTime>` +
		`<ExpiryTime>2026-05-15T10:00:00Z</ExpiryTime>` +
		`<Expires>False</Expires>` +
		`<UsageCount>0</UsageCount>` +
		`<LocationChanged>2026-05-01T10:00:00Z</LocationChanged>` +
		`</Times>` +
		`<String><Key>Title</Key><Value>v1</Value></String>` +
		`</Entry>` +
		`</History>`

	first, err := FromInnerXML([]byte(fragment))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(first.History) != 1 {
		t.Fatalf("History len = %d, want 1", len(first.History))
	}
	if first.History[0].Strings["Title"].V != "v1" {
		t.Errorf("History[0].Title = %q, want v1", first.History[0].Strings["Title"].V)
	}
	if first.History[0].History != nil {
		t.Error("History[0].History should be nil (no nested history allowed)")
	}

	emitted, err := ToInnerXML(first)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	second, err := FromInnerXML(emitted)
	if err != nil {
		t.Fatalf("re-parse: %v\n%s", err, emitted)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("round-trip mismatch:\nfirst:  %+v\nsecond: %+v", first, second)
	}
}

func TestRoundTrip_Expires(t *testing.T) {
	expiryTime := time.Date(2027, 1, 1, 12, 0, 0, 0, time.UTC)
	e := &Entry{
		V:    SchemaVersion,
		UUID: "00010203-0405-0607-0809-0a0b0c0d0e0f",
		Times: Times{
			Created:         time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			Modified:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			Accessed:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			LocationChanged: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			Expires:         true,
			ExpiresAt:       &expiryTime,
			UsageCount:      0,
		},
		Strings: map[string]String{
			"Title": {V: "expiring"},
		},
	}

	emitted, err := ToInnerXML(e)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}

	parsed, err := FromInnerXML(emitted)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !parsed.Times.Expires {
		t.Fatal("Expires lost during round-trip")
	}
	if parsed.Times.ExpiresAt == nil {
		t.Fatal("ExpiresAt nil after round-trip")
	}
	if !parsed.Times.ExpiresAt.Equal(expiryTime) {
		t.Errorf("ExpiresAt = %v, want %v", *parsed.Times.ExpiresAt, expiryTime)
	}
}

func TestRoundTrip_KDBX4BinaryTimes(t *testing.T) {
	// Verificer at parser accepterer KDBX4's base64-int64-tidsstempler
	// (samme format som kdbx.parseKdbxTime håndterer). "HtWo4Q4AAAA="
	// er en ægte timestamp fra et KDBX4-export i 2026.
	fragment := `<UUID>AAECAwQFBgcICQoLDA0ODw==</UUID>` +
		`<IconID>0</IconID>` +
		`<ForegroundColor></ForegroundColor>` +
		`<BackgroundColor></BackgroundColor>` +
		`<OverrideURL></OverrideURL>` +
		`<Tags></Tags>` +
		`<Times>` +
		`<CreationTime>HtWo4Q4AAAA=</CreationTime>` +
		`<LastModificationTime>HtWo4Q4AAAA=</LastModificationTime>` +
		`<LastAccessTime>HtWo4Q4AAAA=</LastAccessTime>` +
		`<ExpiryTime>HtWo4Q4AAAA=</ExpiryTime>` +
		`<Expires>False</Expires>` +
		`<UsageCount>0</UsageCount>` +
		`<LocationChanged>HtWo4Q4AAAA=</LocationChanged>` +
		`</Times>` +
		`<String><Key>Title</Key><Value>binary-time</Value></String>`

	e, err := FromInnerXML([]byte(fragment))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if e.Times.Modified.Year() != 2026 {
		t.Errorf("Modified.Year = %d, want 2026 (full: %v)", e.Times.Modified.Year(), e.Times.Modified)
	}
}

func TestRoundTrip_XMLEscaping(t *testing.T) {
	// Værdier med XML-metakarakterer skal overleve emission + re-parse.
	e := &Entry{
		V:    SchemaVersion,
		UUID: "00010203-0405-0607-0809-0a0b0c0d0e0f",
		Times: Times{
			Created:         time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			Modified:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			Accessed:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			LocationChanged: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
		},
		Strings: map[string]String{
			"Title":    {V: `<weird> & "things"`},
			"Password": {V: `pa$$w0rd & ' < " >`, Protected: true},
		},
	}
	emitted, err := ToInnerXML(e)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if strings.Contains(string(emitted), `<weird>`) {
		t.Fatalf("emitted XML contains unescaped <weird>: %s", emitted)
	}
	parsed, err := FromInnerXML(emitted)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Strings["Title"].V != `<weird> & "things"` {
		t.Errorf("Title round-trip failed: %q", parsed.Strings["Title"].V)
	}
	if parsed.Strings["Password"].V != `pa$$w0rd & ' < " >` {
		t.Errorf("Password round-trip failed: %q", parsed.Strings["Password"].V)
	}
	if !parsed.Strings["Password"].Protected {
		t.Error("Password lost Protected flag")
	}
}

func TestRoundTrip_CustomData(t *testing.T) {
	mtime := time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC)
	e := &Entry{
		V:    SchemaVersion,
		UUID: "00010203-0405-0607-0809-0a0b0c0d0e0f",
		Times: Times{
			Created:         time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			Modified:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			Accessed:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			LocationChanged: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
		},
		CustomData: map[string]CustomDataItem{
			"KPXC_BROWSER_ext": {V: "browser-state", Modified: &mtime},
			"PluginXYZ_data":   {V: "plain"},
		},
	}
	emitted, err := ToInnerXML(e)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	parsed, err := FromInnerXML(emitted)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !reflect.DeepEqual(e.CustomData, parsed.CustomData) {
		t.Errorf("CustomData mismatch:\nwant: %+v\ngot:  %+v", e.CustomData, parsed.CustomData)
	}
}

func TestRoundTrip_Binaries(t *testing.T) {
	e := &Entry{
		V:    SchemaVersion,
		UUID: "00010203-0405-0607-0809-0a0b0c0d0e0f",
		Times: Times{
			Created:         time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			Modified:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			Accessed:        time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC),
			LocationChanged: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
		},
		Binaries: []Binary{
			{Name: "key.pem", Data: []byte("-----BEGIN KEY-----\nABCD\n-----END KEY-----\n")},
			{Name: "binary.dat", Data: []byte{0x00, 0xff, 0x42}},
		},
	}
	emitted, err := ToInnerXML(e)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	parsed, err := FromInnerXML(emitted)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !reflect.DeepEqual(e.Binaries, parsed.Binaries) {
		t.Errorf("Binaries mismatch:\nwant: %+v\ngot:  %+v", e.Binaries, parsed.Binaries)
	}
}

func TestFromInnerXML_EmptyFragment(t *testing.T) {
	if _, err := FromInnerXML(nil); err == nil {
		t.Error("expected error for nil fragment")
	}
	if _, err := FromInnerXML([]byte{}); err == nil {
		t.Error("expected error for empty fragment")
	}
}

func TestFromInnerXML_MalformedXML(t *testing.T) {
	for _, bad := range []string{
		`<UUID>not-base64</UUID>`,
		`<UUID></UUID><IconID>nan</IconID>`,
		`<broken`,
	} {
		if _, err := FromInnerXML([]byte(bad)); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}
