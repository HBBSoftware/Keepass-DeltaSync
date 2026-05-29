// SPDX-License-Identifier: GPL-3.0-or-later

package canonical

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

// kdbx4EpochToUnixOffset er sekunder mellem 0001-01-01 UTC og Unix-epoken
// (1970-01-01). Bruges til at konvertere KDBX4's binary-timestamp-format
// (int64 sekunder siden 0001) til Unix-tid uden tid.Duration-overflow.
var kdbx4EpochToUnixOffset = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC).Unix()

// nullKdbxUUID er KeePassXC's repræsentation af "ingen UUID" — 16 null-bytes
// base64-encoded. Vi behandler den som "felt ikke sat" når den optræder.
const nullKdbxUUID = "AAAAAAAAAAAAAAAAAAAAAA=="

// FromInnerXML parser et keepassxc-cli InnerXML-fragment (indholdet af et
// <Entry>...</Entry>-element, uden ydre tags) til en canonical Entry.
// Fragmentet er præcis det kdbx.ParseExport returnerer som Entry.Fragment.
//
// Ukendte child-elementer ignoreres stille — schema-versionering håndteres
// via Entry.V på wire-niveau, ikke via XML-fejlhåndtering. Hvis vi vil fange
// ukendte felter senere, kræver det en eksplicit allowlist-pass.
func FromInnerXML(fragment []byte) (*Entry, error) {
	if len(fragment) == 0 {
		return nil, errors.New("empty fragment")
	}

	// encoding/xml kræver et root-element. Wrap fragmentet i en syntetisk
	// <Entry>-root — keepassxc-cli's eget output har samme tags, så dette
	// matcher schema'en eksakt.
	wrapped := make([]byte, 0, len(fragment)+15)
	wrapped = append(wrapped, []byte("<Entry>")...)
	wrapped = append(wrapped, fragment...)
	wrapped = append(wrapped, []byte("</Entry>")...)

	var raw xmlEntry
	if err := xml.Unmarshal(wrapped, &raw); err != nil {
		return nil, fmt.Errorf("parse entry xml: %w", err)
	}

	return raw.toCanonical()
}

// xmlEntry mirrors KeePassXC's <Entry>-XML-schema. Ukendte child-elementer
// ignoreres af encoding/xml. Tom string-værdier bevares så ToInnerXML kan
// emitterer dem tilbage uden at tabe Layout — KeePassXC's import er
// tolerant overfor manglende tomme felter, men round-trip-fidelity er
// nemmere at debugge når vi bevarer dem.
type xmlEntry struct {
	UUID                string         `xml:"UUID"`
	IconID              int            `xml:"IconID"`
	CustomIconUUID      string         `xml:"CustomIconUUID"`
	ForegroundColor     string         `xml:"ForegroundColor"`
	BackgroundColor     string         `xml:"BackgroundColor"`
	OverrideURL         string         `xml:"OverrideURL"`
	Tags                string         `xml:"Tags"`
	PreviousParentGroup string         `xml:"PreviousParentGroup"`
	QualityCheck        *xmlBool       `xml:"QualityCheck"`
	Times               xmlTimes       `xml:"Times"`
	Strings             []xmlString    `xml:"String"`
	Binaries            []xmlBinary    `xml:"Binary"`
	AutoType            *xmlAutoType   `xml:"AutoType"`
	CustomData          *xmlCustomData `xml:"CustomData"`
	History             *xmlHistory    `xml:"History"`
}

type xmlTimes struct {
	CreationTime         string `xml:"CreationTime"`
	LastModificationTime string `xml:"LastModificationTime"`
	LastAccessTime       string `xml:"LastAccessTime"`
	ExpiryTime           string `xml:"ExpiryTime"`
	Expires              string `xml:"Expires"`
	UsageCount           int    `xml:"UsageCount"`
	LocationChanged      string `xml:"LocationChanged"`
}

type xmlString struct {
	Key   string   `xml:"Key"`
	Value xmlValue `xml:"Value"`
}

type xmlValue struct {
	Content string `xml:",chardata"`
	// ProtectInMemory er det attribut-navn keepassxc-cli's eget export
	// emitterer for memory-protected felter (Password, custom protected
	// fields). Protected er KDBX-spec'ens in-database form og indikerer
	// ciphertext — vi accepterer den for robusthed mod ikke-KeePassXC
	// eksporter, men foretrækker ProtectInMemory ved tvivl.
	ProtectInMemory string `xml:"ProtectInMemory,attr"`
	Protected       string `xml:"Protected,attr"`
}

type xmlBinary struct {
	Key   string         `xml:"Key"`
	Value xmlBinaryValue `xml:"Value"`
}

type xmlBinaryValue struct {
	Content string `xml:",chardata"`
	Ref     string `xml:"Ref,attr"`
}

type xmlAutoType struct {
	Enabled                 string                   `xml:"Enabled"`
	DataTransferObfuscation int                      `xml:"DataTransferObfuscation"`
	DefaultSequence         string                   `xml:"DefaultSequence"`
	Associations            []xmlAutoTypeAssociation `xml:"Association"`
}

type xmlAutoTypeAssociation struct {
	Window            string `xml:"Window"`
	KeystrokeSequence string `xml:"KeystrokeSequence"`
}

type xmlCustomData struct {
	Items []xmlCustomDataItem `xml:"Item"`
}

type xmlCustomDataItem struct {
	Key                  string `xml:"Key"`
	Value                string `xml:"Value"`
	LastModificationTime string `xml:"LastModificationTime"`
}

type xmlHistory struct {
	Entries []xmlEntry `xml:"Entry"`
}

// xmlBool håndterer KDBX' "True"/"False"-string-encoding for XML-booleans.
// encoding/xml's std bool-parser accepterer "true"/"false" + "1"/"0" men IKKE
// PascalCase "True"/"False" som er hvad KeePassXC emitterer.
type xmlBool bool

func (b *xmlBool) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var s string
	if err := d.DecodeElement(&s, &start); err != nil {
		return err
	}
	*b = xmlBool(strings.EqualFold(strings.TrimSpace(s), "True"))
	return nil
}

func (x *xmlEntry) toCanonical() (*Entry, error) {
	uuid, err := decodeUUID(x.UUID)
	if err != nil {
		return nil, fmt.Errorf("uuid: %w", err)
	}

	times, err := x.Times.toCanonical()
	if err != nil {
		return nil, fmt.Errorf("times: %w", err)
	}

	e := &Entry{
		V:               SchemaVersion,
		UUID:            uuid,
		Times:           times,
		IconID:          x.IconID,
		ForegroundColor: x.ForegroundColor,
		BackgroundColor: x.BackgroundColor,
		OverrideURL:     x.OverrideURL,
	}

	if x.QualityCheck != nil {
		qc := bool(*x.QualityCheck)
		e.QualityCheck = &qc
	}

	if x.CustomIconUUID != "" && x.CustomIconUUID != nullKdbxUUID {
		cuuid, err := decodeUUID(x.CustomIconUUID)
		if err != nil {
			return nil, fmt.Errorf("custom icon uuid: %w", err)
		}
		e.CustomIconUUID = cuuid
	}

	if x.PreviousParentGroup != "" && x.PreviousParentGroup != nullKdbxUUID {
		ppg, err := decodeUUID(x.PreviousParentGroup)
		if err != nil {
			return nil, fmt.Errorf("previous parent group uuid: %w", err)
		}
		e.PreviousParentGroup = ppg
	}

	if tags := strings.TrimSpace(x.Tags); tags != "" {
		// KDBX accepterer både komma og semikolon som tag-separator.
		// KeePassXC emitterer med komma; nogle andre tools (gamle KeePass-
		// versioner, manuelle eksporter) bruger semikolon. Vi splitter på
		// begge for robusthed. Tomme segmenter dropper vi.
		for _, t := range strings.FieldsFunc(tags, func(r rune) bool { return r == ',' || r == ';' }) {
			if trimmed := strings.TrimSpace(t); trimmed != "" {
				e.Tags = append(e.Tags, trimmed)
			}
		}
	}

	if len(x.Strings) > 0 {
		e.Strings = make(map[string]String, len(x.Strings))
		for _, s := range x.Strings {
			protected := strings.EqualFold(strings.TrimSpace(s.Value.ProtectInMemory), "True") ||
				strings.EqualFold(strings.TrimSpace(s.Value.Protected), "True")
			e.Strings[s.Key] = String{
				V:         s.Value.Content,
				Protected: protected,
			}
		}
	}

	for _, b := range x.Binaries {
		if b.Value.Ref != "" {
			// Pool-reference uden adgang til pool'en. Vi kan ikke materialisere
			// indholdet her — caller skal kalde en pool-aware variant hvis
			// attachment-bevarelse er nødvendig. Skip for v1.
			continue
		}
		raw := strings.TrimSpace(b.Value.Content)
		if raw == "" {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("binary %q: decode base64: %w", b.Key, err)
		}
		e.Binaries = append(e.Binaries, Binary{Name: b.Key, Data: data})
	}

	if x.AutoType != nil {
		e.AutoType = &AutoType{
			Enabled:         strings.EqualFold(strings.TrimSpace(x.AutoType.Enabled), "True"),
			Obfuscation:     x.AutoType.DataTransferObfuscation,
			DefaultSequence: x.AutoType.DefaultSequence,
		}
		for _, a := range x.AutoType.Associations {
			e.AutoType.Associations = append(e.AutoType.Associations, AutoTypeAssociation{
				Window:   a.Window,
				Sequence: a.KeystrokeSequence,
			})
		}
	}

	if x.CustomData != nil && len(x.CustomData.Items) > 0 {
		e.CustomData = make(map[string]CustomDataItem, len(x.CustomData.Items))
		for _, it := range x.CustomData.Items {
			item := CustomDataItem{V: it.Value}
			if it.LastModificationTime != "" {
				t, err := parseKdbxTime(it.LastModificationTime)
				if err != nil {
					return nil, fmt.Errorf("custom data item %q: %w", it.Key, err)
				}
				item.Modified = &t
			}
			e.CustomData[it.Key] = item
		}
	}

	if x.History != nil && len(x.History.Entries) > 0 {
		for i := range x.History.Entries {
			hist, err := x.History.Entries[i].toCanonical()
			if err != nil {
				return nil, fmt.Errorf("history entry %d: %w", i, err)
			}
			// KDBX tillader ikke indlejret history (history-elementer indeholder
			// ikke en <History>-undertræ for sig selv). Defensivt stripper vi
			// alt der måtte være sneget sig ind.
			hist.History = nil
			e.History = append(e.History, *hist)
		}
	}

	return e, nil
}

func (t xmlTimes) toCanonical() (Times, error) {
	created, err := parseKdbxTime(t.CreationTime)
	if err != nil {
		return Times{}, fmt.Errorf("creation time: %w", err)
	}
	modified, err := parseKdbxTime(t.LastModificationTime)
	if err != nil {
		return Times{}, fmt.Errorf("last modification time: %w", err)
	}
	accessed, err := parseKdbxTime(t.LastAccessTime)
	if err != nil {
		return Times{}, fmt.Errorf("last access time: %w", err)
	}
	locChanged, err := parseKdbxTime(t.LocationChanged)
	if err != nil {
		return Times{}, fmt.Errorf("location changed: %w", err)
	}
	expires := strings.EqualFold(strings.TrimSpace(t.Expires), "True")

	out := Times{
		Created:         created,
		Modified:        modified,
		Accessed:        accessed,
		Expires:         expires,
		UsageCount:      t.UsageCount,
		LocationChanged: locChanged,
	}
	if expires && t.ExpiryTime != "" {
		exp, err := parseKdbxTime(t.ExpiryTime)
		if err != nil {
			return Times{}, fmt.Errorf("expiry time: %w", err)
		}
		out.ExpiresAt = &exp
	}
	return out, nil
}

// decodeUUID konverterer KeePassXC's base64-encoded 16-byte UUID til
// standard hex-format med bindestreger ("01234567-89ab-cdef-0123-456789abcdef").
func decodeUUID(b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	if len(raw) != 16 {
		return "", fmt.Errorf("uuid must be 16 bytes, got %d", len(raw))
	}
	h := hex.EncodeToString(raw)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32]), nil
}

// parseKdbxTime accepterer både KDBX3's ISO 8601 og KDBX4's base64-encoded
// little-endian int64 sekunder siden 0001-01-01 UTC. Hvilket format der
// bruges afhænger af kdbx-version + keepassxc-cli's output-konfiguration,
// så vi skal kunne læse begge.
func parseKdbxTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
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
	if raw, err := base64.StdEncoding.DecodeString(s); err == nil && len(raw) == 8 {
		secs := int64(binary.LittleEndian.Uint64(raw))
		unixSecs := secs + kdbx4EpochToUnixOffset
		return time.Unix(unixSecs, 0).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp format: %q", s)
}
