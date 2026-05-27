// SPDX-License-Identifier: GPL-3.0-or-later

package kdbx

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"time"
)

// Entry er én entry udlæst fra en .kdbx-eksport. Fragment er det rå XML-indhold
// (uden den ydre <Entry>...</Entry>-wrapper). Det er denne fragment der krypteres
// og uploades; ved pull skal vi kunne wrappe den tilbage som <Entry>fragment</Entry>
// og fodre keepassxc-cli's import/merge.
type Entry struct {
	UUID       string    // standard UUID-format, fx "01234567-89ab-cdef-0123-456789abcdef"
	ModifiedAt time.Time
	Fragment  []byte // raw inner XML
}

// Deletion er én tombstone fra <DeletedObjects>. Pushes som DELETE-call til serveren.
type Deletion struct {
	UUID       string
	DeletedAt time.Time
}

// ParseExport tager en keepassxc-cli export-XML og udtrækker alle entries og
// deletions. Entries i <History>-undertræer ignoreres — det er entry'ens egen
// version-historik som serveren håndterer separat.
func ParseExport(xmlBytes []byte) ([]Entry, []Deletion, error) {
	var doc kdbxFile
	if err := xml.Unmarshal(xmlBytes, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse kdbx xml: %w", err)
	}

	var entries []Entry
	if err := collectEntries(&doc.Root.Group, &entries); err != nil {
		return nil, nil, err
	}

	deletions := make([]Deletion, 0, len(doc.Root.DeletedObjects.Objects))
	for _, d := range doc.Root.DeletedObjects.Objects {
		uuid, err := decodeUUID(d.UUID)
		if err != nil {
			return nil, nil, fmt.Errorf("deleted-object uuid: %w", err)
		}
		t, err := parseKdbxTime(d.DeletionTime)
		if err != nil {
			return nil, nil, fmt.Errorf("deletion-time for %s: %w", uuid, err)
		}
		deletions = append(deletions, Deletion{UUID: uuid, DeletedAt: t})
	}

	return entries, deletions, nil
}

// collectEntries walks groups recursively, collecting entries at every level.
// Nested <Entry> elements inside <History> are NOT visited — they live inside
// each Entry's InnerXML and stay there.
func collectEntries(g *group, out *[]Entry) error {
	for _, e := range g.Entries {
		uuid, err := decodeUUID(e.UUID)
		if err != nil {
			return fmt.Errorf("entry uuid: %w", err)
		}
		t, err := parseKdbxTime(e.Times.LastModificationTime)
		if err != nil {
			return fmt.Errorf("modified-time for entry %s: %w", uuid, err)
		}
		*out = append(*out, Entry{
			UUID:       uuid,
			ModifiedAt: t,
			Fragment:   []byte(e.InnerXML),
		})
	}
	for i := range g.Groups {
		if err := collectEntries(&g.Groups[i], out); err != nil {
			return err
		}
	}
	return nil
}

// decodeUUID konverterer KeePassXC's base64-encoded 16-byte UUID til standard
// hex-format med bindestreger (sådan som serveren forventer).
func decodeUUID(b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	if len(raw) != 16 {
		return "", fmt.Errorf("uuid must be 16 bytes, got %d", len(raw))
	}
	h := hex.EncodeToString(raw)
	// formatér som 8-4-4-4-12
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32]), nil
}

// parseKdbxTime accepterer ISO 8601 / RFC 3339 med Z eller offset. KeePassXC
// 2.5+ skriver ISO 8601 i Z-form ("2026-05-27T08:00:00Z"). Ældre versioner
// brugte et numerisk format — vi forventer kun det nye her.
func parseKdbxTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("empty timestamp")
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp format: %q", s)
}

// --- XML schema-typer ---
//
// Kun de felter vi rør ved er modelleret. ",innerxml" på entry fanger
// alle børn (UUID, Times, String, History, ...) som raw bytes —
// præcis det vi vil round-trippe.

type kdbxFile struct {
	XMLName xml.Name `xml:"KeePassFile"`
	Root    root     `xml:"Root"`
}

type root struct {
	Group          group           `xml:"Group"`
	DeletedObjects deletedObjects  `xml:"DeletedObjects"`
}

type group struct {
	Entries []entry `xml:"Entry"`
	Groups  []group `xml:"Group"`
}

type entry struct {
	UUID     string     `xml:"UUID"`
	Times    entryTimes `xml:"Times"`
	InnerXML string     `xml:",innerxml"`
}

type entryTimes struct {
	LastModificationTime string `xml:"LastModificationTime"`
}

type deletedObjects struct {
	Objects []deletedObject `xml:"DeletedObject"`
}

type deletedObject struct {
	UUID         string `xml:"UUID"`
	DeletionTime string `xml:"DeletionTime"`
}
