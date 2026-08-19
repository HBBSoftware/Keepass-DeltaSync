// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"

	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx"
)

// indexEntry er ét søgbart element, som det ser ud på wiren mod udvidelsen.
//
// Feltsættet HER er sikkerhedsgrænsen mod browseren. Se
// docs/browser-extension.md: hosten må aldrig serialisere andet end titel,
// URL'er og placering — aldrig UserName, Password, Notes, attachments eller
// custom-felter. Grænsen er en allow-list i denne struct frem for en
// "husk at nulstille password"-konvention, så en fremtidig bug i udvidelsens
// JS ikke kan lække et felt hosten aldrig sendte.
type indexEntry struct {
	UUID  string   `json:"uuid"`
	Title string   `json:"title"`
	URLs  []string `json:"urls"`
	Group string   `json:"group"`
	DB    string   `json:"db"`
}

// maxGroupDepth beskytter gruppe-traversering mod en korrupt eksport hvor
// parent-kæden peger i ring. KDBX-træer er i praksis få niveauer dybe.
const maxGroupDepth = 64

// buildIndex oversætter en keepassxc-cli XML-eksport til søgeindekset.
//
// Papirkurven behøver vi ikke filtrere selv — kdbx.ParseExport udelader
// allerede recycle-bin-undertræet. Søge-deaktiverede grupper filtrerer vi
// derimod her, fordi ParseExport bevidst kun eksponerer flaget og lader
// forbrugeren om politikken (sync synkroniserer også skjulte grupper).
func buildIndex(dbName string, xmlBytes []byte) ([]indexEntry, error) {
	entries, groups, _, err := kdbx.ParseExport(xmlBytes)
	if err != nil {
		return nil, err
	}

	tbl := make(map[string]kdbx.Group, len(groups))
	for _, g := range groups {
		tbl[g.UUID] = g
	}

	index := make([]indexEntry, 0, len(entries))
	for _, e := range entries {
		if !groupSearchable(tbl, e.ParentGroupUUID) {
			continue
		}
		fields, err := parseEntryFields(e.Fragment)
		if err != nil {
			// En enkelt uparsbar entry må ikke vælte hele indekset —
			// resten af databasen er stadig brugbar at søge i.
			continue
		}
		title := strings.TrimSpace(fields["Title"].Value)
		urls := extractURLs(fields)
		if title == "" && len(urls) == 0 {
			continue
		}
		index = append(index, indexEntry{
			UUID:  e.UUID,
			Title: title,
			URLs:  urls,
			Group: groupPath(tbl, e.ParentGroupUUID),
			DB:    dbName,
		})
	}
	return index, nil
}

// groupSearchable afgør om en entry i gruppen skal med i indekset.
// KDBX' EnableSearching arves nedad: nil på en gruppe betyder "spørg
// forælderen", og roden defaulter til true. Et eksplicit false hvor som helst
// i kæden skjuler hele undertræet.
func groupSearchable(tbl map[string]kdbx.Group, groupUUID string) bool {
	uuid := groupUUID
	for depth := 0; depth < maxGroupDepth; depth++ {
		if uuid == "" {
			return true // Root-sentinel — ParseExport emitterer ikke Root selv.
		}
		g, ok := tbl[uuid]
		if !ok {
			return true // Ukendt forælder: hellere synlig end tabt.
		}
		if g.EnableSearching != nil {
			return *g.EnableSearching
		}
		uuid = g.ParentUUID
	}
	return true
}

// groupPath bygger den menneskelæsbare sti ("Web/Bank") til visning i
// søgeresultatet. Entries direkte i Root får tom sti.
func groupPath(tbl map[string]kdbx.Group, groupUUID string) string {
	var parts []string
	uuid := groupUUID
	for depth := 0; depth < maxGroupDepth && uuid != ""; depth++ {
		g, ok := tbl[uuid]
		if !ok {
			break
		}
		parts = append(parts, g.Name)
		uuid = g.ParentUUID
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, "/")
}

// extractURLs samler alle navigérbare URL'er fra en entry.
//
// Standardfeltet URL er ikke nok: KeePassXC gemmer "Additional URLs" som
// custom-felter (KP2A_URL_1, KP2A_URL_2, ... — navngivningen er arvet fra
// KeePass2Android), og de er lige så navigérbare som den primære. Den
// primære kommer altid først, så udvidelsen kan bruge urls[0] uden at sortere.
//
// Protected custom-felter springes over: brugeren har markeret dem som
// hemmelige, og et hemmeligt felt hører ikke hjemme i browseren, uanset om
// det tilfældigvis ligner en URL. Standard-URL-feltet tages med selv hvis
// det er protected — det ER entry'ens adresse.
func extractURLs(fields map[string]entryField) []string {
	// IKKE `var urls []string`: en nil-slice serialiseres til JSON-null, og
	// udvidelsen itererer over feltet. En entry uden navigerbar URL er helt
	// normal (placeholders, cmd://, ingen URL overhovedet), så det ville
	// vælte søgningen på den første af dem. Wire-kontrakten er et array.
	urls := []string{}
	seen := make(map[string]bool)

	add := func(raw string) {
		u, ok := normalizeURL(raw)
		if !ok || seen[u] {
			return
		}
		seen[u] = true
		urls = append(urls, u)
	}

	add(fields["URL"].Value)
	for name, f := range fields {
		if name == "URL" || f.Protected {
			continue
		}
		if isAdditionalURLField(name) {
			add(f.Value)
		}
	}
	return urls
}

// entryField er ét <String>-felt fra en entry. Vi materialiserer bevidst kun
// navn, værdi og protected-flaget.
type entryField struct {
	Value     string
	Protected bool
}

// parseEntryFields læser entry-fragmentets <String>-felter og intet andet.
//
// Vi kunne have brugt canonical.FromInnerXML, men den er bygget til sync og
// kræver derfor et komplet, gyldigt entry — bl.a. alle fire tidsstempler. En
// entry med et manglende CreationTime ville dermed forsvinde fra søgningen,
// selvom dens titel og URL er helt i orden. Søgning skal være lempelig hvor
// sync skal være streng, så indekseringen får sin egen minimale parser.
func parseEntryFields(fragment []byte) (map[string]entryField, error) {
	if len(fragment) == 0 {
		return nil, fmt.Errorf("empty entry fragment")
	}

	// encoding/xml kræver et root-element; fragmentet er entry'ens INDHOLD.
	wrapped := make([]byte, 0, len(fragment)+15)
	wrapped = append(wrapped, "<Entry>"...)
	wrapped = append(wrapped, fragment...)
	wrapped = append(wrapped, "</Entry>"...)

	var raw struct {
		Strings []struct {
			Key   string `xml:"Key"`
			Value struct {
				Protected string `xml:"Protected,attr"`
				Text      string `xml:",chardata"`
			} `xml:"Value"`
		} `xml:"String"`
	}
	if err := xml.Unmarshal(wrapped, &raw); err != nil {
		return nil, err
	}

	fields := make(map[string]entryField, len(raw.Strings))
	for _, s := range raw.Strings {
		fields[s.Key] = entryField{
			Value:     s.Value.Text,
			Protected: strings.EqualFold(strings.TrimSpace(s.Value.Protected), "true"),
		}
	}
	return fields, nil
}

// isAdditionalURLField genkender de custom-feltnavne KeePassXC og
// KeePass2Android bruger til ekstra URL'er.
func isAdditionalURLField(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(lower, "kp2a_url") || strings.HasPrefix(lower, "additional url")
}

// normalizeURL afviser alt der ikke kan sendes direkte til tabs.update, og
// normaliserer resten.
//
// De tre farlige kategorier er: KDBX-placeholders ({REF:...}, {S:felt},
// {TITLE}), som først giver mening efter KeePass' egen substitution;
// ikke-web-schemes som cmd:// og javascript:, hvor navigation enten er
// meningsløs eller direkte farlig; og tomme værdier. Skema-løse værdier
// ("example.com") er derimod almindelige i rigtige databaser og opgraderes
// til https.
func normalizeURL(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" || strings.ContainsAny(s, "{}") || strings.ContainsAny(s, " \t\r\n") {
		return "", false
	}

	if !strings.Contains(s, "://") {
		if strings.HasPrefix(s, "//") || !strings.Contains(s, ".") {
			return "", false
		}
		s = "https://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return "", false
	}
	if u.Host == "" {
		return "", false
	}
	return u.String(), true
}

// paginate deler indekset i bidder der hver især holder sig under limit
// bytes serialiseret. Firefox' native messaging tillader højst 1 MB pr.
// besked FRA applikationen, så et stort indeks skal hentes i sider — se
// docs/browser-extension.md.
func paginate(index []indexEntry, offset, limitBytes int) (page []indexEntry, next int, err error) {
	if offset < 0 || offset > len(index) {
		return nil, 0, fmt.Errorf("offset %d out of range (index has %d entries)", offset, len(index))
	}
	size := 0
	i := offset
	for ; i < len(index); i++ {
		n := approxJSONSize(index[i])
		if i > offset && size+n > limitBytes {
			break
		}
		size += n
	}
	return index[offset:i], i, nil
}

// approxJSONSize estimerer en entrys serialiserede størrelse. Et estimat er
// nok: sider skal bare holde sig komfortabelt under wire-loftet, ikke ramme
// det præcist.
func approxJSONSize(e indexEntry) int {
	n := 96 + len(e.UUID) + len(e.Title) + len(e.Group) + len(e.DB)
	for _, u := range e.URLs {
		n += len(u) + 4
	}
	return n
}
