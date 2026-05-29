// SPDX-License-Identifier: GPL-3.0-or-later

package canonical

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ToInnerXML emitterer en canonical Entry som et keepassxc-cli-kompatibelt
// InnerXML-fragment — *uden* den ydre <Entry>...</Entry>-wrapper, så det
// matcher det format kdbx.ParseExport returnerer som Entry.Fragment.
//
// Output'et kan fodres direkte til kdbx.BuildStagingXML i pull-pipeline'en,
// og dets struktur er identisk med hvad keepassxc-cli's egen export ville
// have produceret for en tilsvarende entry — undtagen formatering
// (whitespace), som keepassxc-cli ignorerer ved import.
func ToInnerXML(e *Entry) ([]byte, error) {
	if e == nil {
		return nil, fmt.Errorf("nil entry")
	}
	var buf bytes.Buffer
	if err := writeEntryInner(&buf, e); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeEntryInner skriver entry'ens børn (ingen <Entry>-wrap). Bruges både
// af ToInnerXML (top-level) og af History-emission, der wrapper hver
// historisk version i sin egen <Entry>...</Entry>.
func writeEntryInner(buf *bytes.Buffer, e *Entry) error {
	uuidB64, err := encodeUUID(e.UUID)
	if err != nil {
		return fmt.Errorf("uuid: %w", err)
	}
	fmt.Fprintf(buf, "<UUID>%s</UUID>", uuidB64)

	fmt.Fprintf(buf, "<IconID>%d</IconID>", e.IconID)

	if e.CustomIconUUID != "" {
		cuuid, err := encodeUUID(e.CustomIconUUID)
		if err != nil {
			return fmt.Errorf("custom icon uuid: %w", err)
		}
		fmt.Fprintf(buf, "<CustomIconUUID>%s</CustomIconUUID>", cuuid)
	}

	fmt.Fprintf(buf, "<ForegroundColor>%s</ForegroundColor>", escapeText(e.ForegroundColor))
	fmt.Fprintf(buf, "<BackgroundColor>%s</BackgroundColor>", escapeText(e.BackgroundColor))
	fmt.Fprintf(buf, "<OverrideURL>%s</OverrideURL>", escapeText(e.OverrideURL))
	fmt.Fprintf(buf, "<Tags>%s</Tags>", escapeText(strings.Join(e.Tags, ";")))

	writeTimes(buf, e.Times)

	// Strings emitteres i sorteret nøgle-orden for determinisme — gør diffs
	// og round-trip-tests reproducerbare uden at miste informationen
	// (KDBX behandler entry-strings som et sæt, ikke en sekvens).
	keys := make([]string, 0, len(e.Strings))
	for k := range e.Strings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		writeString(buf, k, e.Strings[k])
	}

	for _, b := range e.Binaries {
		writeBinary(buf, b)
	}

	if e.AutoType != nil {
		writeAutoType(buf, e.AutoType)
	}

	if len(e.CustomData) > 0 {
		writeCustomData(buf, e.CustomData)
	}

	if e.QualityCheck != nil {
		if *e.QualityCheck {
			buf.WriteString("<QualityCheck>True</QualityCheck>")
		} else {
			buf.WriteString("<QualityCheck>False</QualityCheck>")
		}
	}

	if e.PreviousParentGroup != "" {
		ppg, err := encodeUUID(e.PreviousParentGroup)
		if err != nil {
			return fmt.Errorf("previous parent group uuid: %w", err)
		}
		fmt.Fprintf(buf, "<PreviousParentGroup>%s</PreviousParentGroup>", ppg)
	}

	if len(e.History) > 0 {
		buf.WriteString("<History>")
		for i := range e.History {
			buf.WriteString("<Entry>")
			// History-elementer må ikke selv have History — vi stripper
			// defensivt (og round-trippet validerer at vores parser også
			// gør det).
			h := e.History[i]
			h.History = nil
			if err := writeEntryInner(buf, &h); err != nil {
				return fmt.Errorf("history entry %d: %w", i, err)
			}
			buf.WriteString("</Entry>")
		}
		buf.WriteString("</History>")
	}

	return nil
}

func writeTimes(buf *bytes.Buffer, t Times) {
	buf.WriteString("<Times>")
	fmt.Fprintf(buf, "<LastModificationTime>%s</LastModificationTime>", formatKdbxTime(t.Modified))
	fmt.Fprintf(buf, "<CreationTime>%s</CreationTime>", formatKdbxTime(t.Created))
	fmt.Fprintf(buf, "<LastAccessTime>%s</LastAccessTime>", formatKdbxTime(t.Accessed))
	// KDBX kræver ExpiryTime-feltet selv når Expires=false. KeePassXC
	// emitterer typisk en nu-tid eller en sentinel; vi bruger ExpiresAt
	// hvis sat, ellers Modified som best-effort sentinel.
	expiryTime := t.Modified
	if t.ExpiresAt != nil {
		expiryTime = *t.ExpiresAt
	}
	fmt.Fprintf(buf, "<ExpiryTime>%s</ExpiryTime>", formatKdbxTime(expiryTime))
	if t.Expires {
		buf.WriteString("<Expires>True</Expires>")
	} else {
		buf.WriteString("<Expires>False</Expires>")
	}
	fmt.Fprintf(buf, "<UsageCount>%d</UsageCount>", t.UsageCount)
	fmt.Fprintf(buf, "<LocationChanged>%s</LocationChanged>", formatKdbxTime(t.LocationChanged))
	buf.WriteString("</Times>")
}

func writeString(buf *bytes.Buffer, key string, s String) {
	buf.WriteString("<String>")
	fmt.Fprintf(buf, "<Key>%s</Key>", escapeText(key))
	if s.Protected {
		fmt.Fprintf(buf, "<Value Protected=\"True\">%s</Value>", escapeText(s.V))
	} else {
		fmt.Fprintf(buf, "<Value>%s</Value>", escapeText(s.V))
	}
	buf.WriteString("</String>")
}

func writeBinary(buf *bytes.Buffer, b Binary) {
	buf.WriteString("<Binary>")
	fmt.Fprintf(buf, "<Key>%s</Key>", escapeText(b.Name))
	// Inline base64 — pool-references håndteres ikke i v1. Hvis keepassxc-cli
	// senere refuserer inline (tvivlsomt — import skal kunne genopbygge
	// pool'en), fall-back: post-import-trin der flytter til pool.
	fmt.Fprintf(buf, "<Value>%s</Value>", base64.StdEncoding.EncodeToString(b.Data))
	buf.WriteString("</Binary>")
}

func writeAutoType(buf *bytes.Buffer, a *AutoType) {
	buf.WriteString("<AutoType>")
	if a.Enabled {
		buf.WriteString("<Enabled>True</Enabled>")
	} else {
		buf.WriteString("<Enabled>False</Enabled>")
	}
	fmt.Fprintf(buf, "<DataTransferObfuscation>%d</DataTransferObfuscation>", a.Obfuscation)
	fmt.Fprintf(buf, "<DefaultSequence>%s</DefaultSequence>", escapeText(a.DefaultSequence))
	for _, as := range a.Associations {
		buf.WriteString("<Association>")
		fmt.Fprintf(buf, "<Window>%s</Window>", escapeText(as.Window))
		fmt.Fprintf(buf, "<KeystrokeSequence>%s</KeystrokeSequence>", escapeText(as.Sequence))
		buf.WriteString("</Association>")
	}
	buf.WriteString("</AutoType>")
}

func writeCustomData(buf *bytes.Buffer, cd map[string]CustomDataItem) {
	keys := make([]string, 0, len(cd))
	for k := range cd {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	buf.WriteString("<CustomData>")
	for _, k := range keys {
		item := cd[k]
		buf.WriteString("<Item>")
		fmt.Fprintf(buf, "<Key>%s</Key>", escapeText(k))
		fmt.Fprintf(buf, "<Value>%s</Value>", escapeText(item.V))
		if item.Modified != nil {
			fmt.Fprintf(buf, "<LastModificationTime>%s</LastModificationTime>", formatKdbxTime(*item.Modified))
		}
		buf.WriteString("</Item>")
	}
	buf.WriteString("</CustomData>")
}

// encodeUUID konverterer standard hex-UUID ("01234567-89ab-...") til den
// base64-encoded 16-byte form keepassxc-cli forventer.
func encodeUUID(hexUUID string) (string, error) {
	stripped := strings.ReplaceAll(hexUUID, "-", "")
	if len(stripped) != 32 {
		return "", fmt.Errorf("uuid must be 32 hex chars, got %d", len(stripped))
	}
	raw, err := hex.DecodeString(stripped)
	if err != nil {
		return "", fmt.Errorf("hex decode: %w", err)
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// formatKdbxTime emitterer ISO 8601 til sekund-præcision. KeePassXC accepterer
// både ISO og KDBX4-binary ved import; ISO er nemmere at debugge og kompakt
// nok at størrelsen ikke er en bekymring.
func formatKdbxTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

// escapeText kører encoding/xml's text-escaper på en string og returnerer
// resultatet. Håndterer &, <, >, ' og " — alt hvad XML's text-content kræver.
func escapeText(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}
