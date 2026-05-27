// SPDX-License-Identifier: GPL-3.0-or-later

package kdbx

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
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
