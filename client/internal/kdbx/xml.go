// SPDX-License-Identifier: GPL-3.0-or-later

package kdbx

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"
)

// nullKdbxUUID er KeePassXC's repræsentation af "ingen UUID" (16 null-bytes
// base64-encoded). Bruges som sentinel for RecycleBinUUID når recycle bin
// endnu ikke er materialiseret — den bliver først rigtig UUID efter første
// gang en entry sendes til Papirkurv.
const nullKdbxUUID = "AAAAAAAAAAAAAAAAAAAAAA=="

// kdbx4EpochToUnixOffset er antal sekunder fra 0001-01-01 UTC til Unix-
// epoken (1970-01-01). KDBX4-timestamps er int64 sekunder fra 0001-01-01,
// så for at få en time.Time bruger vi time.Unix(secs+offset, 0). offset er
// negativ pga. 0001 < 1970. Vi kan IKKE bruge time.Duration aritmetik her —
// Go's Duration overflows for spans > ~292 år (max int64 nanosekunder).
var kdbx4EpochToUnixOffset = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC).Unix()

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

// RootGroupUUID parser XML-eksporten og returnerer Root-gruppens UUID i
// KDBX' native base64-form. Bruges af pull-flowet til at bygge en staging-
// kdbx hvis "deltasync"-gruppe har SAMME UUID som lokal Root, så
// keepassxc-cli's merge placerer nye entries direkte i Root i stedet for at
// oprette en separat undergruppe.
func RootGroupUUID(xmlBytes []byte) (string, error) {
	var doc kdbxFile
	if err := xml.Unmarshal(xmlBytes, &doc); err != nil {
		return "", fmt.Errorf("parse kdbx xml: %w", err)
	}
	if doc.Root.Group.UUID == "" {
		return "", errors.New("root group has no UUID")
	}
	return doc.Root.Group.UUID, nil
}

// ParseExport tager en keepassxc-cli export-XML og udtrækker alle entries og
// deletions. Entries i <History>-undertræer ignoreres — det er entry'ens egen
// version-historik som serveren håndterer separat.
//
// Entries der ligger i recycle-bin-gruppen (matches via <Meta><RecycleBinUUID>)
// behandles som synthetic deletions med DeletedAt = entry'ens
// <LocationChanged>. Det betyder at "delete i KeePassXC GUI" propagerer som
// faktisk sletning til andre enheder, selv om brugeren ikke har "Empty Recycle
// Bin" først. Trade-off: undelete (flyt entry tilbage ud af Papirkurv) virker
// ikke på tværs af enheder — entry'en er allerede slettet på server. Hvis
// recycle-bin er deaktiveret eller endnu ikke materialiseret
// (RecycleBinUUID = null), gør synthesis intet.
func ParseExport(xmlBytes []byte) ([]Entry, []Deletion, error) {
	var doc kdbxFile
	if err := xml.Unmarshal(xmlBytes, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse kdbx xml: %w", err)
	}

	recycleBinUUID := activeRecycleBinUUID(doc.Meta)

	var entries []Entry
	deletions := make([]Deletion, 0, len(doc.Root.DeletedObjects.Objects))
	if err := collectEntries(&doc.Root.Group, recycleBinUUID, &entries, &deletions); err != nil {
		return nil, nil, err
	}

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

// activeRecycleBinUUID returnerer recycle-bin-gruppens UUID i base64-form
// hvis recycle bin er enabled OG en gruppe faktisk er udpeget. Returnerer ""
// hvis ingen synthesis skal foretages.
func activeRecycleBinUUID(m meta) string {
	if !strings.EqualFold(strings.TrimSpace(m.RecycleBinEnabled), "True") {
		return ""
	}
	if m.RecycleBinUUID == "" || m.RecycleBinUUID == nullKdbxUUID {
		return ""
	}
	return m.RecycleBinUUID
}

// collectEntries walks groups recursively. Entries i gruppen med UUID
// recycleBinUUID synthesizes som Deletion i stedet for at lande i entries-
// listen. Entries inde i <History>-undertræer besøges ikke — de lever som
// raw InnerXML på hver parent-entry.
func collectEntries(g *group, recycleBinUUID string, entries *[]Entry, deletions *[]Deletion) error {
	inRecycleBin := recycleBinUUID != "" && g.UUID == recycleBinUUID
	for _, e := range g.Entries {
		uuid, err := decodeUUID(e.UUID)
		if err != nil {
			return fmt.Errorf("entry uuid: %w", err)
		}
		if inRecycleBin {
			// LocationChanged er tidspunktet hvor entry blev flyttet ind i
			// recycle bin — semantisk det rigtige "delete-tidspunkt". Fallback
			// til LastModificationTime hvis LocationChanged er tom (ældre
			// databaser eller corner cases).
			ts := e.Times.LocationChanged
			if ts == "" {
				ts = e.Times.LastModificationTime
			}
			t, err := parseKdbxTime(ts)
			if err != nil {
				return fmt.Errorf("recycle-bin deletion-time for entry %s: %w", uuid, err)
			}
			*deletions = append(*deletions, Deletion{UUID: uuid, DeletedAt: t})
			continue
		}
		t, err := parseKdbxTime(e.Times.LastModificationTime)
		if err != nil {
			return fmt.Errorf("modified-time for entry %s: %w", uuid, err)
		}
		*entries = append(*entries, Entry{
			UUID:       uuid,
			ModifiedAt: t,
			Fragment:   []byte(e.InnerXML),
		})
	}
	for i := range g.Groups {
		if err := collectEntries(&g.Groups[i], recycleBinUUID, entries, deletions); err != nil {
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

// parseKdbxTime accepterer både KDBX3's ISO 8601 ("2026-05-27T08:00:00Z") og
// KDBX4's base64-encoded little-endian int64 ("HtWo4Q4AAAA=") som er sekunder
// fra 0001-01-01 UTC. Hvilket format der bruges afhænger af kdbx-versionen +
// keepassxc-cli's output-konfiguration; vores parser skal kunne håndtere
// begge for at virke på tværs af databaser.
func parseKdbxTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("empty timestamp")
	}
	// ISO 8601 først (KDBX3 + nogle KDBX4-konfigurationer).
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	// Fallback: KDBX4 binary format. base64-decode → 8 bytes →
	// little-endian int64 sekunder fra 0001-01-01 UTC.
	if raw, err := base64.StdEncoding.DecodeString(s); err == nil && len(raw) == 8 {
		secs := int64(binary.LittleEndian.Uint64(raw))
		unixSecs := secs + kdbx4EpochToUnixOffset
		return time.Unix(unixSecs, 0).UTC(), nil
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
	Meta    meta     `xml:"Meta"`
	Root    root     `xml:"Root"`
}

type meta struct {
	RecycleBinEnabled string `xml:"RecycleBinEnabled"` // "True"/"False"
	RecycleBinUUID    string `xml:"RecycleBinUUID"`    // base64-encoded 16-byte UUID, nullKdbxUUID hvis ikke materialiseret
}

type root struct {
	Group          group          `xml:"Group"`
	DeletedObjects deletedObjects `xml:"DeletedObjects"`
}

type group struct {
	UUID    string  `xml:"UUID"`
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
	LocationChanged      string `xml:"LocationChanged"`
}

type deletedObjects struct {
	Objects []deletedObject `xml:"DeletedObject"`
}

type deletedObject struct {
	UUID         string `xml:"UUID"`
	DeletionTime string `xml:"DeletionTime"`
}
