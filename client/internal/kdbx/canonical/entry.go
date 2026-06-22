// SPDX-License-Identifier: GPL-3.0-or-later

// Package canonical definerer det platform-uafhængige entry-format der
// krypteres og sendes til serveren fra v3 og frem. Se
// docs/v3-canonical-entry-format.md for designgrundlaget.
//
// Strukturerne her modellerer KDBX4's entry-skema 1:1, så desktop-klienten
// (via keepassxc-cli's InnerXML-fragment) og Android-klienten (via kotpass'
// typed Entry) begge kan emittere og parse det samme on-wire format uden
// at den ene side er afhængig af den andens implementation.
package canonical

import "time"

// SchemaVersion er den canonical-skema-version dette package emitterer.
// Når formatet ændres breaking, bumpes denne. Læsere skal acceptere alle
// kendte versioner — vi har dual-read for legacy XML-fragmenter (byte 0
// == '<') såvel som canonical v1 (byte 0 == 0x01).
const SchemaVersion = 1

// Entry er den platform-uafhængige entry-repræsentation. Den round-tripper
// gennem KDBX4 via keepassxc-cli's export/import, og kan også produceres/
// forbruges direkte af kotpass på Android.
//
// Felt-navngivning følger KDBX4's egne termer hvor det giver mening
// (`UUID`, `Times`, `Strings`) men normaliserer til JSON-snake_case for
// bedre cross-platform læsbarhed.
type Entry struct {
	// V er skema-versionen. Skal være SchemaVersion ved emission.
	V int `json:"v"`

	// UUID i standard 8-4-4-4-12 hex-format (samme som server's UUID-felt
	// i entry_versions). Konverteres til KDBX4's base64-16-byte form ved
	// ToInnerXML.
	UUID string `json:"uuid"`

	Times Times `json:"times"`

	// Strings rummer alle entry-felter — både de "kanoniske" (Title,
	// UserName, Password, URL, Notes) og custom-felter brugeren har
	// tilføjet. KDBX' datamodel skelner ikke mellem dem, og det gør vi
	// heller ikke. Iteration-orden er ikke garanteret meningsfuld for
	// sync-korrekthed.
	Strings map[string]String `json:"strings,omitempty"`

	// Binaries er inline attachments. KDBX deler attachments via en pool
	// pr. database, men på wiren er hver entry self-contained.
	Binaries []Binary `json:"binaries,omitempty"`

	Tags []string `json:"tags,omitempty"`

	// IconID er KDBX' built-in ikon (0-68). 0 (= Key-ikonet) er standard.
	IconID int `json:"icon_id"`

	// CustomIconUUID peger på en custom-ikon-ressource i kdbx' Meta.
	// Tomt hvis ikke sat. Standard 8-4-4-4-12 hex-format.
	CustomIconUUID string `json:"custom_icon_uuid,omitempty"`

	// Farver er hex-strings ("#RRGGBB") eller tomme.
	ForegroundColor string `json:"foreground_color,omitempty"`
	BackgroundColor string `json:"background_color,omitempty"`

	OverrideURL string `json:"override_url,omitempty"`

	// AutoType er nil hvis entry'en ikke har auto-type-konfiguration
	// (≠ "auto-type disabled" — det er AutoType.Enabled=false).
	AutoType *AutoType `json:"autotype,omitempty"`

	// QualityCheck er KeePassXC-specifikt — om password-quality-checks
	// skal køres på dette entry. Pointer for at skelne "ikke sat"
	// (KeePassXC default) fra eksplicit false.
	QualityCheck *bool `json:"quality_check,omitempty"`

	// CustomData er en plugin-data-store på entry-niveau. KDBX4 tracker
	// LastModificationTime pr. item; den bevares for round-trip.
	CustomData map[string]CustomDataItem `json:"custom_data,omitempty"`

	// PreviousParentGroup er KeePassXC-specifikt: UUID på den gruppe en
	// entry blev flyttet fra, så undo virker på tværs af sync. Tom hvis
	// entry'en ikke er blevet flyttet.
	PreviousParentGroup string `json:"previous_parent_group,omitempty"`

	// ParentGroup er UUID på den gruppe entry'en aktuelt ligger i (v4
	// gruppe-sync). Tom string = roden (sentinel) — hver klient mapper den
	// til sin egen lokale Root-UUID, da Root-UUID er forskellig pr. enhed.
	// Populeres fra entry'ens position i gruppetræet ved indsamling og
	// konsumeres til at placere entry'en ved merge. Lever i JSON-blob'en, men
	// IKKE i KDBX' <Entry>-fragment (dér er parent = træ-position). Additivt:
	// en v1-klient ignorerer feltet og placerer entry'en i staging/Root.
	ParentGroup string `json:"parent_group,omitempty"`

	// History er entry'ens historiske versioner — ældste først. Indlejret
	// History er forbudt (KDBX' eget format gør det også) så vi behøver
	// ikke begrænse rekursionsdybden, men en valideret entry har altid
	// nil History på sine History-elementer.
	History []Entry `json:"history,omitempty"`
}

// GroupSchemaVersion er canonical-skema-versionen for Group-objekter. Bumpes
// uafhængigt af entry-skemaet (Entry.V) når gruppe-formatet ændres breaking.
const GroupSchemaVersion = 1

// Group er den platform-uafhængige gruppe-repræsentation til v4 gruppe-sync.
// Grupper synkroniseres som selvstændige UUID-nøglede objekter (object_kind=2
// på serveren, envelope-byte 0x02), parallelt med entries. Sletning håndteres
// via server's tombstone (deleted-flag), ikke et felt her. Se
// docs/v4-group-sync.md.
type Group struct {
	// V er skema-versionen. Skal være GroupSchemaVersion ved emission.
	V int `json:"v"`

	// UUID i standard 8-4-4-4-12 hex-format.
	UUID string `json:"uuid"`

	Name  string `json:"name"`
	Notes string `json:"notes,omitempty"`

	// ParentGroup er UUID på forældregruppen. Tom string = roden (sentinel),
	// samme konvention som Entry.ParentGroup. Root-gruppen selv synkroniseres
	// aldrig som et Group-objekt.
	ParentGroup string `json:"parent_group,omitempty"`

	IconID         int    `json:"icon_id"`
	CustomIconUUID string `json:"custom_icon_uuid,omitempty"`

	Times Times `json:"times"`
}

// Times holder KDBX' tidsstempler pr. entry. Alle tider er UTC.
type Times struct {
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
	Accessed time.Time `json:"accessed"`

	// ExpiresAt er nil når Expires=false. KDBX gemmer en sentinel-tid
	// her, men vi normaliserer til null på wiren for klarhed.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Expires   bool       `json:"expires"`

	UsageCount      int       `json:"usage_count"`
	LocationChanged time.Time `json:"location_changed"`
}

// String er et enkelt key-value-felt på en entry. Protected angiver om
// værdien skal memory-protectes (stream-cipher krypteres i kdbx-filen) ved
// re-import. Standard for Password er typisk true; andre felter false.
type String struct {
	V         string `json:"v"`
	Protected bool   `json:"protected,omitempty"`
}

// Binary er en inline attachment. Data er rå bytes; encoding/json sender
// den som base64 over wiren.
type Binary struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

// AutoType er auto-type-konfigurationen for en entry.
type AutoType struct {
	Enabled bool `json:"enabled"`

	// Obfuscation: 0 = none, 1 = use-clipboard. KDBX-spec.
	Obfuscation int `json:"obfuscation,omitempty"`

	// DefaultSequence er auto-type-template (fx "{USERNAME}{TAB}{PASSWORD}{ENTER}").
	// Tom betyder "brug database-default".
	DefaultSequence string `json:"default_sequence,omitempty"`

	Associations []AutoTypeAssociation `json:"associations,omitempty"`
}

// AutoTypeAssociation er en pr.-vindue auto-type-override.
type AutoTypeAssociation struct {
	Window   string `json:"window"`
	Sequence string `json:"sequence"`
}

// CustomDataItem er én plugin-værdi i Entry.CustomData. KDBX4 tracker
// LastModificationTime pr. item.
type CustomDataItem struct {
	V        string     `json:"v"`
	Modified *time.Time `json:"modified,omitempty"`
}
