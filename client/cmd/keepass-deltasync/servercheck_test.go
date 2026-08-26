// SPDX-License-Identifier: GPL-3.0-or-later

package main

// Læse-kun diagnose: hvad mener SERVEREN om hver entrys gruppeplacering?
//
// Ligger som test, så den kan genbruge klientens egne dekrypterings-helpers
// uden at komme med i den udgivne binary. Springes over medmindre
// DELTASYNC_CHECK er sat, så den aldrig kører i CI.
//
//	DELTASYNC_CHECK=1 go test ./cmd/keepass-deltasync/ -run TestServerPlacementReport -v
//
// Masterpassword læses fra stdin. Der skrives INTET til serveren.

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"sort"
	"testing"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx"
	"gitlab.com/Star95/keepass-deltasync/client/internal/passwd"
)

var titleRe = regexp.MustCompile(`(?s)<Key>Title</Key>\s*<Value[^>]*>(.*?)</Value>`)

func TestServerPlacementReport(t *testing.T) {
	if os.Getenv("DELTASYNC_CHECK") == "" {
		t.Skip("sæt DELTASYNC_CHECK=1 for at køre denne diagnose")
	}
	dbName := os.Getenv("DELTASYNC_DB")
	if dbName == "" {
		dbName = "mypasswords"
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	db := cfg.FindDatabase(dbName)
	if db == nil {
		t.Fatalf("ingen lokal database ved navn %q", dbName)
	}

	// passwd.Read slukker ekkoet, så adgangskoden ikke havner i terminalens
	// scrollback. Falder tilbage til stdin hvis der ikke er en tty.
	password, perr := passwd.Read("Masterpassword: ", false)
	if perr != nil {
		t.Fatalf("læs password: %v", perr)
	}
	defer passwd.Zero(password)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	client := api.New(cfg.Server.URL)

	serverDBs, err := client.ListDatabases(ctx, cfg.Server.DeviceToken)
	if err != nil {
		t.Fatalf("list databases: %v", err)
	}
	var role string
	var wrapped *string
	for i := range serverDBs {
		if serverDBs[i].ID == db.RemoteID {
			role, wrapped = serverDBs[i].Role, serverDBs[i].WrappedMasterKey
		}
	}
	_, entryKey, err := resolveMasterEntryKeys(password, role, db.RemoteID, wrapped, cfg.Server.DevicePrivateKey)
	if err != nil {
		t.Fatalf("nøgler: %v", err)
	}

	changes, err := client.GetChanges(ctx, cfg.Server.DeviceToken, db.RemoteID, 0, true)
	if err != nil {
		t.Fatalf("GET /changes: %v", err)
	}

	groupNames := map[string]string{}
	groupParent := map[string]string{}
	type ent struct{ title, parent string }
	entries := map[string]ent{}
	tombstones := 0

	for _, c := range changes.Entries {
		blob, derr := base64.StdEncoding.DecodeString(c.Blob)
		if derr != nil {
			continue
		}
		modAt, _ := time.Parse(time.RFC3339, c.ModifiedAt)
		if c.Deleted {
			tombstones++
			delete(entries, c.UUID)
			continue
		}
		if c.Kind == 2 {
			g, gerr := decryptToGroup(entryKey, blob, c.UUID, modAt)
			if gerr != nil {
				t.Fatalf("gruppe %s: %v", c.UUID, gerr)
			}
			groupNames[c.UUID] = g.Name
			groupParent[c.UUID] = g.ParentUUID
			continue
		}
		frag, parent, ferr := decryptToFragmentWithParent(entryKey, blob, c.UUID, modAt)
		if ferr != nil {
			t.Fatalf("entry %s: %v", c.UUID, ferr)
		}
		title := ""
		if m := titleRe.FindSubmatch(frag); m != nil {
			title = string(m[1])
		}
		entries[c.UUID] = ent{title: title, parent: parent}
	}

	var path func(uuid string, depth int) string
	path = func(uuid string, depth int) string {
		if uuid == "" || depth > 20 {
			return ""
		}
		name, ok := groupNames[uuid]
		if !ok {
			return "<UKENDT-GRUPPE " + uuid + ">"
		}
		if p := path(groupParent[uuid], depth+1); p != "" {
			return p + "/" + name
		}
		return name
	}

	var inRoot []string
	for _, e := range entries {
		if e.parent == "" {
			inRoot = append(inRoot, e.title)
		}
	}
	sort.Strings(inRoot)

	fmt.Printf("\n=== SERVERENS BILLEDE (current_seq=%d) ===\n", changes.CurrentSeq)
	fmt.Printf("entries: %d    grupper: %d    tombstones: %d\n\n", len(entries), len(groupNames), tombstones)

	fmt.Printf("--- entries serveren mener ligger i RODEN (%d) ---\n", len(inRoot))
	for _, ti := range inRoot {
		fmt.Printf("  %s\n", ti)
	}

	fmt.Printf("\n--- entries med forældregruppe serveren IKKE kender ---\n")
	unknown := 0
	for _, e := range entries {
		if e.parent != "" {
			if _, ok := groupNames[e.parent]; !ok {
				fmt.Printf("  %-42s -> %s\n", e.title, e.parent)
				unknown++
			}
		}
	}
	if unknown == 0 {
		fmt.Println("  (ingen)")
	}

	fmt.Printf("\n--- de 19 poster fra hændelsen ---\n")
	watch := []string{
		"192.168.1.166", "192.168.1.230 (Yacht)", "192.168.2.10 zigbee web",
		"Linuxin", "Min måler", "PayPal", "Piko-solar-portal",
		"Router USG 110 Windows lan. (192.168.0.1)", "Ryanair", "Suntrol portal",
		"Tastselv skat.", "Watts", "app.thestorygraph.com", "openweathermap.org",
		"seas-nve.dk", "vault.bitwarden.com", "www.linux-fan-shop.de",
	}
	for _, w := range watch {
		found := false
		for _, e := range entries {
			if e.title == w {
				found = true
				loc := path(e.parent, 0)
				if loc == "" {
					loc = "ROOT"
				}
				fmt.Printf("  %-42s -> %s\n", w, loc)
			}
		}
		if !found {
			fmt.Printf("  %-42s -> (ikke på serveren)\n", w)
		}
	}
}

// TestPendingGroupDeletions viser hvilke grupper den FØRSTE sync ville
// tombstone på serveren. Push-siden sletter enhver gruppe i config's
// known_groups der ikke længere findes i den lokale .kdbx (syncop.go), og
// tombstones propagerer videre til alle andre enheder. Denne rapport gør det
// synligt før det sker. Læse-kun; der skrives intet.
func TestPendingGroupDeletions(t *testing.T) {
	if os.Getenv("DELTASYNC_CHECK") == "" {
		t.Skip("sæt DELTASYNC_CHECK=1 for at køre denne diagnose")
	}
	dbName := os.Getenv("DELTASYNC_DB")
	if dbName == "" {
		dbName = "mypasswords"
	}

	cfg, db, cli, err := loadDBAndCLI(dbName, "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	password, perr := passwd.Read("Masterpassword: ", false)
	if perr != nil {
		t.Fatalf("læs password: %v", perr)
	}
	defer passwd.Zero(password)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	xmlBytes, err := cli.Export(ctx, db.LocalPath, password)
	if err != nil {
		t.Fatalf("export lokal kdbx: %v", err)
	}
	_, localGroups, _, err := kdbx.ParseExport(xmlBytes)
	if err != nil {
		t.Fatalf("parse export: %v", err)
	}
	current := map[string]bool{}
	for _, g := range localGroups {
		current[g.UUID] = true
	}

	var doomed []string
	for _, known := range db.KnownGroups {
		if !current[known] {
			doomed = append(doomed, known)
		}
	}

	fmt.Printf("\n=== FØRSTE SYNC VILLE TOMBSTONE %d GRUPPER ===\n", len(doomed))
	fmt.Printf("known_groups i config: %d    grupper i .kdbx nu: %d\n\n", len(db.KnownGroups), len(current))
	if len(doomed) == 0 {
		fmt.Println("  (ingen — intet slettes)")
		return
	}

	// Slå navnene op på serveren, så listen kan læses af et menneske.
	client := api.New(cfg.Server.URL)
	serverDBs, err := client.ListDatabases(ctx, cfg.Server.DeviceToken)
	if err != nil {
		t.Fatalf("list databases: %v", err)
	}
	var role string
	var wrapped *string
	for i := range serverDBs {
		if serverDBs[i].ID == db.RemoteID {
			role, wrapped = serverDBs[i].Role, serverDBs[i].WrappedMasterKey
		}
	}
	_, entryKey, err := resolveMasterEntryKeys(password, role, db.RemoteID, wrapped, cfg.Server.DevicePrivateKey)
	if err != nil {
		t.Fatalf("nøgler: %v", err)
	}
	changes, err := client.GetChanges(ctx, cfg.Server.DeviceToken, db.RemoteID, 0, true)
	if err != nil {
		t.Fatalf("GET /changes: %v", err)
	}
	names := map[string]string{}
	entryCount := map[string]int{}
	for _, c := range changes.Entries {
		blob, derr := base64.StdEncoding.DecodeString(c.Blob)
		if derr != nil || c.Deleted {
			continue
		}
		modAt, _ := time.Parse(time.RFC3339, c.ModifiedAt)
		if c.Kind == 2 {
			if g, gerr := decryptToGroup(entryKey, blob, c.UUID, modAt); gerr == nil {
				names[c.UUID] = g.Name
			}
			continue
		}
		if _, parent, ferr := decryptToFragmentWithParent(entryKey, blob, c.UUID, modAt); ferr == nil && parent != "" {
			entryCount[parent]++
		}
	}

	sort.Slice(doomed, func(i, j int) bool { return names[doomed[i]] < names[doomed[j]] })
	for _, u := range doomed {
		n := names[u]
		if n == "" {
			n = "(kender serveren ikke)"
		}
		warn := ""
		if entryCount[u] > 0 {
			warn = fmt.Sprintf("   <-- %d entries peger stadig på den!", entryCount[u])
		}
		fmt.Printf("  %-30s %s%s\n", n, u, warn)
	}
}

// TestMissingOnServer finder objekter der findes LOKALT men ikke på serveren,
// selvom config's entry_states påstår de er synkroniseret. Sådan en post
// pushes aldrig af en normal sync — filteret tror den er oppe.
//
// Skriver UUID-listen til missing-uuids.txt, så staten kan beskæres kirurgisk
// i stedet for at tvinge hele databasen op med `push --force` (som ville
// brænde versionshistorikken for alle 591 poster). Læse-kun mod serveren.
func TestMissingOnServer(t *testing.T) {
	if os.Getenv("DELTASYNC_CHECK") == "" {
		t.Skip("sæt DELTASYNC_CHECK=1 for at køre denne diagnose")
	}
	dbName := os.Getenv("DELTASYNC_DB")
	if dbName == "" {
		dbName = "mypasswords"
	}

	cfg, db, cli, err := loadDBAndCLI(dbName, "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	password, perr := passwd.Read("Masterpassword: ", false)
	if perr != nil {
		t.Fatalf("læs password: %v", perr)
	}
	defer passwd.Zero(password)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	xmlBytes, err := cli.Export(ctx, db.LocalPath, password)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	localEntries, localGroups, _, err := kdbx.ParseExport(xmlBytes)
	if err != nil {
		t.Fatalf("parse export: %v", err)
	}

	client := api.New(cfg.Server.URL)
	changes, err := client.GetChanges(ctx, cfg.Server.DeviceToken, db.RemoteID, 0, true)
	if err != nil {
		t.Fatalf("GET /changes: %v", err)
	}
	// Feed'et er seq-ordnet: en senere tombstone ophæver et tidligere objekt.
	onServer := map[string]bool{}
	everSeen := map[string]bool{}
	tombstonedAt := map[string]string{}
	for _, c := range changes.Entries {
		everSeen[c.UUID] = true
		if c.Deleted {
			delete(onServer, c.UUID)
			tombstonedAt[c.UUID] = c.ModifiedAt
			continue
		}
		delete(tombstonedAt, c.UUID)
		onServer[c.UUID] = true
	}

	states := db.EntryStates
	var missGroups, missEntries []string
	for _, g := range localGroups {
		if !onServer[g.UUID] {
			missGroups = append(missGroups, g.UUID)
		}
	}
	for _, e := range localEntries {
		if !onServer[e.UUID] {
			missEntries = append(missEntries, e.UUID)
		}
	}

	claimed := 0
	for _, u := range append(append([]string{}, missGroups...), missEntries...) {
		if _, ok := states[u]; ok {
			claimed++
		}
	}

	fmt.Printf("\n=== FINDES LOKALT, MEN IKKE PÅ SERVEREN ===\n")
	fmt.Printf("lokalt : %d grupper, %d entries\n", len(localGroups), len(localEntries))
	fmt.Printf("mangler: %d grupper, %d entries\n", len(missGroups), len(missEntries))
	fmt.Printf("heraf %d som entry_states fejlagtigt kalder synkroniseret\n\n", claimed)

	names := map[string]string{}
	for _, g := range localGroups {
		names[g.UUID] = g.Name
	}
	classify := func(u string) string {
		if ts, ok := tombstonedAt[u]; ok {
			return "SLETTET paa serveren " + ts
		}
		if everSeen[u] {
			return "set, men ikke aktuel"
		}
		return "aldrig naaet op"
	}
	for _, u := range missGroups {
		mark := " "
		if _, ok := states[u]; ok {
			mark = "!"
		}
		fmt.Printf("  %s gruppe  %-28s %s\n", mark, names[u], classify(u))
	}
	nTomb, nNever := 0, 0
	for _, u := range append(append([]string{}, missGroups...), missEntries...) {
		if _, ok := tombstonedAt[u]; ok {
			nTomb++
		} else if !everSeen[u] {
			nNever++
		}
	}
	fmt.Printf("\nopsummering: %d tombstonet paa serveren, %d aldrig naaet op\n", nTomb, nNever)

	out := "C:/Users/Hans/DeltaSync-Rescue-20260826/missing-uuids.txt"
	f, ferr := os.Create(out)
	if ferr != nil {
		t.Fatalf("skriv liste: %v", ferr)
	}
	defer f.Close()
	for _, u := range append(append([]string{}, missGroups...), missEntries...) {
		fmt.Fprintln(f, u)
	}
	fmt.Printf("\n%d UUID'er skrevet til %s\n", len(missGroups)+len(missEntries), out)
}
