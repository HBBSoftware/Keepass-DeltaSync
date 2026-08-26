// SPDX-License-Identifier: GPL-3.0-or-later

package kdbx

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// StagingEntry er én entry der skal med i staging-databasen.
// Fragment er det rå inner-XML (uden den ydre <Entry>...</Entry>-wrapper) —
// præcis det vi gemmer i krypteret form på serveren.
type StagingEntry struct {
	UUID       string // standard UUID-format med bindestreger
	Fragment   []byte // raw inner XML
	ModifiedAt time.Time

	// ParentGroup er standard-UUID på gruppen entry'en skal placeres i, "" =
	// Root (sentinel). Bruges kun af BuildStagingXMLWithGroups (v4 group-sync).
	ParentGroup string
}

// StagingGroup er én gruppe der skal genopbygges i staging-databasens
// gruppetræ (v4 group-sync). UUID/ParentUUID er standard hex-format;
// ParentUUID == "" betyder Root.
type StagingGroup struct {
	UUID            string
	ParentUUID      string
	Name            string
	Notes           string
	IconID          int
	CreatedAt       time.Time
	ModifiedAt      time.Time
	LocationChanged time.Time

	// EnableSearching er KDBX' per-gruppe søgeflag; nil = arv fra forælder.
	// Bæres med så et pull ikke nulstiller en gruppe brugeren har taget ud
	// af søgeresultater. Wire-formatet (canonical.Group) kender ikke feltet,
	// så det udfyldes kun fra det lokale gruppetræ.
	EnableSearching *bool
}

// StagingDeletion er én tombstone der skal med i staging-databasens
// <DeletedObjects>-sektion. KeePassXC's merge bruger denne til at slette
// matchede entries i target-databasen.
type StagingDeletion struct {
	UUID      string
	DeletedAt time.Time
}

// BuildStagingXML konstruerer en valid KDBX-import-XML der wrapper alle
// dekrypterede entries i én staging-gruppe + en <DeletedObjects>-liste.
// Resultatet kan fodres til `keepassxc-cli import` for at få en kdbx-fil,
// som derefter merges ind i den lokale database.
//
// groupUUID styrer hvor merge placerer NYE entries (entries hvis UUID ikke
// allerede findes i target). Hvis groupUUID matcher target's Root-gruppe-
// UUID, lander de i Root. Hvis det er en ny UUID, oprettes en "deltasync"-
// undergruppe. Tom string → der genereres en frisk random UUID.
//
// Allerede-eksisterende entries (matchet på UUID) bliver opdateret in-place
// i deres oprindelige grupper uanset.
func BuildStagingXML(entries []StagingEntry, deletions []StagingDeletion, groupUUID string) ([]byte, error) {
	if groupUUID == "" {
		var err error
		groupUUID, err = randomUUIDBase64()
		if err != nil {
			return nil, fmt.Errorf("staging group uuid: %w", err)
		}
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	buf.WriteString("<KeePassFile>\n")
	buf.WriteString("  <Meta>\n")
	buf.WriteString("    <Generator>keepass-deltasync-client</Generator>\n")
	buf.WriteString("  </Meta>\n")
	buf.WriteString("  <Root>\n")
	buf.WriteString("    <Group>\n")
	buf.WriteString("      <UUID>" + groupUUID + "</UUID>\n")
	buf.WriteString("      <Name>deltasync</Name>\n")
	buf.WriteString("      <Notes>Synced entries from keepass-deltasync. Move to your preferred group; merge will keep them in place by UUID.</Notes>\n")
	buf.WriteString("      <IconID>48</IconID>\n")
	buf.WriteString("      <Times>\n")
	buf.WriteString("        <CreationTime>" + now + "</CreationTime>\n")
	buf.WriteString("        <LastModificationTime>" + now + "</LastModificationTime>\n")
	buf.WriteString("        <LastAccessTime>" + now + "</LastAccessTime>\n")
	buf.WriteString("        <ExpiryTime>" + now + "</ExpiryTime>\n")
	buf.WriteString("        <Expires>False</Expires>\n")
	buf.WriteString("        <UsageCount>0</UsageCount>\n")
	buf.WriteString("        <LocationChanged>" + now + "</LocationChanged>\n")
	buf.WriteString("      </Times>\n")
	buf.WriteString("      <IsExpanded>True</IsExpanded>\n")
	buf.WriteString("      <DefaultAutoTypeSequence/>\n")
	buf.WriteString("      <EnableAutoType>null</EnableAutoType>\n")
	buf.WriteString("      <EnableSearching>null</EnableSearching>\n")
	buf.WriteString("      <LastTopVisibleEntry>AAAAAAAAAAAAAAAAAAAAAA==</LastTopVisibleEntry>\n")

	for _, e := range entries {
		buf.WriteString("      <Entry>")
		buf.Write(e.Fragment)
		buf.WriteString("</Entry>\n")
	}

	buf.WriteString("    </Group>\n")

	if len(deletions) > 0 {
		buf.WriteString("    <DeletedObjects>\n")
		for _, d := range deletions {
			b64, err := uuidHexToBase64(d.UUID)
			if err != nil {
				return nil, fmt.Errorf("encode deletion uuid %s: %w", d.UUID, err)
			}
			buf.WriteString("      <DeletedObject>\n")
			buf.WriteString("        <UUID>" + b64 + "</UUID>\n")
			buf.WriteString("        <DeletionTime>" + d.DeletedAt.UTC().Format("2006-01-02T15:04:05Z") + "</DeletionTime>\n")
			buf.WriteString("      </DeletedObject>\n")
		}
		buf.WriteString("    </DeletedObjects>\n")
	}

	buf.WriteString("  </Root>\n")
	buf.WriteString("</KeePassFile>\n")
	return buf.Bytes(), nil
}

// BuildStagingXMLWithGroups bygger en staging-import-XML hvor entries placeres
// i deres rigtige gruppe (StagingEntry.ParentGroup) og hele gruppetræet
// genopbygges fra groups. rootUUID skal være target-databasens Root-UUID
// (base64) så keepassxc-cli's merge folder træet ind i den eksisterende Root;
// tom → en frisk random UUID (børnene merges stadig ind i target Root).
//
// Parent-referencer der peger på en ukendt gruppe behandles som Root. En
// visited-vagt bryder evt. cykler (hver gruppe emittes højst én gang); grupper
// der derved bliver uopnåelige fra Root udelades (pathologisk, skærpes i fase 6).
func BuildStagingXMLWithGroups(entries []StagingEntry, groups []StagingGroup, deletions []StagingDeletion, rootUUID string) ([]byte, error) {
	if rootUUID == "" {
		var err error
		rootUUID, err = randomUUIDBase64()
		if err != nil {
			return nil, fmt.Errorf("staging root uuid: %w", err)
		}
	}

	known := make(map[string]bool, len(groups))
	for _, g := range groups {
		known[g.UUID] = true
	}
	resolveParent := func(p string) string {
		if p != "" && !known[p] {
			return "" // orphan/ukendt parent → Root
		}
		return p
	}
	childGroups := make(map[string][]StagingGroup)
	for _, g := range groups {
		p := resolveParent(g.ParentUUID)
		childGroups[p] = append(childGroups[p], g)
	}
	entriesByParent := make(map[string][]StagingEntry)
	for _, e := range entries {
		p := resolveParent(e.ParentGroup)
		entriesByParent[p] = append(entriesByParent[p], e)
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	buf.WriteString("<KeePassFile>\n")
	buf.WriteString("  <Meta>\n    <Generator>keepass-deltasync-client</Generator>\n  </Meta>\n")
	buf.WriteString("  <Root>\n")
	// Root-gruppen matcher target's Root-UUID, så børn merges ind i Root.
	buf.WriteString("    <Group>\n")
	buf.WriteString("      <UUID>" + rootUUID + "</UUID>\n")
	writeGroupHeader(&buf, "      ", "Root", "", 49, time.Time{}, time.Time{}, time.Time{}, now, nil)
	visited := make(map[string]bool)
	if err := writeGroupChildren(&buf, "", "      ", childGroups, entriesByParent, visited, now); err != nil {
		return nil, err
	}
	buf.WriteString("    </Group>\n")

	if len(deletions) > 0 {
		buf.WriteString("    <DeletedObjects>\n")
		for _, d := range deletions {
			b64, err := uuidHexToBase64(d.UUID)
			if err != nil {
				return nil, fmt.Errorf("encode deletion uuid %s: %w", d.UUID, err)
			}
			buf.WriteString("      <DeletedObject>\n")
			buf.WriteString("        <UUID>" + b64 + "</UUID>\n")
			buf.WriteString("        <DeletionTime>" + d.DeletedAt.UTC().Format("2006-01-02T15:04:05Z") + "</DeletionTime>\n")
			buf.WriteString("      </DeletedObject>\n")
		}
		buf.WriteString("    </DeletedObjects>\n")
	}

	buf.WriteString("  </Root>\n</KeePassFile>\n")
	return buf.Bytes(), nil
}

// writeGroupChildren emitterer entries og undergrupper for gruppen med
// reference parentRef ("" = Root), rekursivt.
func writeGroupChildren(buf *bytes.Buffer, parentRef, indent string, childGroups map[string][]StagingGroup, entriesByParent map[string][]StagingEntry, visited map[string]bool, now string) error {
	for _, e := range entriesByParent[parentRef] {
		buf.WriteString(indent + "<Entry>")
		buf.Write(e.Fragment)
		buf.WriteString("</Entry>\n")
	}
	for _, g := range childGroups[parentRef] {
		if visited[g.UUID] {
			continue // cykel-vagt
		}
		visited[g.UUID] = true
		b64, err := uuidHexToBase64(g.UUID)
		if err != nil {
			return fmt.Errorf("encode group uuid %s: %w", g.UUID, err)
		}
		buf.WriteString(indent + "<Group>\n")
		buf.WriteString(indent + "  <UUID>" + b64 + "</UUID>\n")
		writeGroupHeader(buf, indent+"  ", g.Name, g.Notes, g.IconID, g.CreatedAt, g.ModifiedAt, g.LocationChanged, now, g.EnableSearching)
		if err := writeGroupChildren(buf, g.UUID, indent+"  ", childGroups, entriesByParent, visited, now); err != nil {
			return err
		}
		buf.WriteString(indent + "</Group>\n")
	}
	return nil
}

// writeGroupHeader skriver en gruppes felter fra <Name> til <LastTopVisibleEntry>
// (UUID skrives af caller). Tidsstempler defaulter til now ved zero-time.
func writeGroupHeader(buf *bytes.Buffer, indent, name, notes string, iconID int, created, modified, locationChanged time.Time, now string, enableSearching *bool) {
	buf.WriteString(indent + "<Name>" + xmlEscape(name) + "</Name>\n")
	if notes != "" {
		buf.WriteString(indent + "<Notes>" + xmlEscape(notes) + "</Notes>\n")
	}
	buf.WriteString(indent + "<IconID>" + strconv.Itoa(iconID) + "</IconID>\n")
	buf.WriteString(indent + "<Times>\n")
	buf.WriteString(indent + "  <CreationTime>" + isoOrNow(created, now) + "</CreationTime>\n")
	buf.WriteString(indent + "  <LastModificationTime>" + isoOrNow(modified, now) + "</LastModificationTime>\n")
	buf.WriteString(indent + "  <LastAccessTime>" + now + "</LastAccessTime>\n")
	buf.WriteString(indent + "  <ExpiryTime>" + now + "</ExpiryTime>\n")
	buf.WriteString(indent + "  <Expires>False</Expires>\n")
	buf.WriteString(indent + "  <UsageCount>0</UsageCount>\n")
	buf.WriteString(indent + "  <LocationChanged>" + isoOrNow(locationChanged, now) + "</LocationChanged>\n")
	buf.WriteString(indent + "</Times>\n")
	buf.WriteString(indent + "<IsExpanded>True</IsExpanded>\n")
	buf.WriteString(indent + "<DefaultAutoTypeSequence/>\n")
	buf.WriteString(indent + "<EnableAutoType>null</EnableAutoType>\n")
	buf.WriteString(indent + "<EnableSearching>" + tristate(enableSearching) + "</EnableSearching>\n")
	buf.WriteString(indent + "<LastTopVisibleEntry>AAAAAAAAAAAAAAAAAAAAAA==</LastTopVisibleEntry>\n")
}

// tristate serialiserer KDBX' per-gruppe tri-state-boolean: nil betyder
// "arv fra forælder" og skrives som null, jf. parseTristateBool i xml.go.
func tristate(b *bool) string {
	if b == nil {
		return "null"
	}
	if *b {
		return "True"
	}
	return "False"
}

func isoOrNow(t time.Time, now string) string {
	if t.IsZero() {
		return now
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// randomUUIDBase64 returnerer 16 random bytes som base64 — KeePassXC's
// XML-UUID-form.
func randomUUIDBase64() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b[:]), nil
}

// uuidHexToBase64 konverterer "8-4-4-4-12" hex-UUID til base64-encoded
// 16-byte form (KeePassXC's XML-format).
func uuidHexToBase64(uuid string) (string, error) {
	stripped := strings.ReplaceAll(uuid, "-", "")
	if len(stripped) != 32 {
		return "", fmt.Errorf("uuid must be 32 hex chars, got %d", len(stripped))
	}
	raw, err := hex.DecodeString(stripped)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}
