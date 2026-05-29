// SPDX-License-Identifier: GPL-3.0-or-later

package canonical

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Format er den ydre format-familie indlejret i en dekrypteret entry-blob.
// Detekteres ved at peek'e plaintext[0] efter DecryptBlob.
//
// Schemaversionen *inden i* canonical-payloaden ligger på `Entry.V` —
// envelope-byte'en skelner kun mellem familier (legacy-XML vs canonical-JSON),
// ikke mellem canonical-versioner.
type Format int

const (
	// FormatUnknown er sentinel for "kunne ikke genkende byte 0".
	// Caller bør behandle som hård fejl (bug eller korrupt blob).
	FormatUnknown Format = iota

	// FormatLegacyXML er v1-blob-formatet: rå keepassxc-cli InnerXML-fragment.
	// Detekteres ved at plaintext[0] == '<'.
	FormatLegacyXML

	// FormatCanonical er v3-blob-formatet: byte 0x01 efterfulgt af
	// JSON-marshalled canonical.Entry.
	FormatCanonical
)

// formatByteCanonical er magic-byten der prefixer canonical JSON-payloads.
// Valgt så den ikke kolliderer med nogen gyldig start-byte for legacy XML —
// XML-fragmenter starter altid med '<' (0x3C).
const formatByteCanonical byte = 0x01

// DetectFormat klassificerer en dekrypteret entry-blob baseret på dens
// første byte. Tom plaintext → Unknown. Caller'en dispatcher derefter til
// FromInnerXML (legacy) eller DecodeCanonical (canonical).
func DetectFormat(plaintext []byte) Format {
	if len(plaintext) == 0 {
		return FormatUnknown
	}
	switch plaintext[0] {
	case formatByteCanonical:
		return FormatCanonical
	case '<':
		return FormatLegacyXML
	default:
		return FormatUnknown
	}
}

// EncodeCanonical marshaller en Entry til JSON og prepender format-version-
// byten. Resultatet er det plaintext der skal fodres til crypto.EncryptBlob.
//
// Entry.V sættes til SchemaVersion uanset hvad caller har sat — vi tillader
// ikke at skrive blobs med en anden skemaversion end den vi selv emitterer.
func EncodeCanonical(e *Entry) ([]byte, error) {
	if e == nil {
		return nil, errors.New("nil entry")
	}
	e.V = SchemaVersion

	raw, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical: %w", err)
	}

	out := make([]byte, 0, len(raw)+1)
	out = append(out, formatByteCanonical)
	out = append(out, raw...)
	return out, nil
}

// DecodeCanonical inverterer EncodeCanonical: validerer at plaintext har
// canonical format-byten og unmarshaller resten som JSON. Returnerer fejl
// hvis byte 0 ikke matcher (i så fald har caller fejldispatched —
// DetectFormat skal kaldes først).
func DecodeCanonical(plaintext []byte) (*Entry, error) {
	if len(plaintext) < 2 {
		return nil, errors.New("plaintext too short for canonical envelope")
	}
	if plaintext[0] != formatByteCanonical {
		return nil, fmt.Errorf("expected canonical format byte 0x%02x, got 0x%02x", formatByteCanonical, plaintext[0])
	}

	var e Entry
	if err := json.Unmarshal(plaintext[1:], &e); err != nil {
		return nil, fmt.Errorf("unmarshal canonical: %w", err)
	}
	if e.V == 0 {
		return nil, errors.New("canonical entry missing schema version (v)")
	}
	if e.V > SchemaVersion {
		return nil, fmt.Errorf("canonical schema v%d newer than supported v%d — upgrade client", e.V, SchemaVersion)
	}
	return &e, nil
}
