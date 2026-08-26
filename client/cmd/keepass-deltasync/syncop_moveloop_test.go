package main

import (
	"strings"
	"testing"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx"
)

const isoLayout = "2006-01-02T15:04:05Z"

// En flytning mellem grupper bumper LocationChanged, ikke ModifiedAt. Push
// registrerer derfor laterTime(ModifiedAt, LocationChanged), mens serveren kun
// kender ModifiedAt. Pull må ikke rulle staten tilbage til serverens værdi —
// ellers siger push-tjekket ja igen ved næste tick, i det uendelige.
func TestMovedEntryDoesNotRePushForever(t *testing.T) {
	const uuid = "0088f6f3-f9b3-43a6-b7ae-d0ee15d35f55"
	mod := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC) // sidst redigeret
	loc := time.Date(2026, 8, 26, 7, 30, 0, 0, time.UTC) // flyttet i dag

	db := &config.Database{}

	trigger := laterTime(mod, loc)
	if !shouldPush(db.EntryStates, uuid, trigger) {
		t.Fatal("tick 1: den flyttede entry burde pushes")
	}
	db.RecordEntryState(uuid, trigger.UTC().Format(isoLayout))

	// Serveren fik en.ModifiedAt og ekkoer entry'en tilbage i næste pull.
	db.RecordEntryStateIfNewer(uuid, mod.UTC().Format(isoLayout))

	if shouldPush(db.EntryStates, uuid, trigger) {
		t.Fatalf("løkke: entry re-pushes i det uendelige (state=%q)", db.EntryStates[uuid])
	}
}

// Samme regression for grupper: gruppe-loopet bruger nu også laterTime, så en
// gruppe-flytning opdages — og må heller ikke ende i en løkke.
func TestMovedGroupIsPushedOnceAndNotAgain(t *testing.T) {
	const uuid = "00daaba0-56c6-4c7f-a999-bc831b412c75"
	mod := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	loc := time.Date(2026, 8, 26, 7, 30, 0, 0, time.UTC)

	db := &config.Database{}
	db.RecordEntryState(uuid, mod.UTC().Format(isoLayout)) // synket før flytningen

	trigger := laterTime(mod, loc)
	if !shouldPush(db.EntryStates, uuid, trigger) {
		t.Fatal("gruppe-flytning blev ikke opdaget")
	}
	db.RecordEntryState(uuid, trigger.UTC().Format(isoLayout))

	db.RecordEntryStateIfNewer(uuid, mod.UTC().Format(isoLayout)) // pull-ekko
	if shouldPush(db.EntryStates, uuid, trigger) {
		t.Fatal("løkke: gruppe re-pushes i det uendelige")
	}
}

// Et delta-pull bærer kun de grupper der er ændret siden last_seq. Uden det
// fulde lokale træ betragter staging-builderen en uændret forældregruppe som
// ukendt og placerer entry'en i Root — præcis den fejl der flyttede 19 poster
// op i roden. mergeGroupTree skal levere stien.
func TestPulledEntryKeepsGroupWhenParentNotInDelta(t *testing.T) {
	const parentUUID = "1a4d6d09-1eb7-4a5f-9e2b-0e9a2a9f1c33"

	entries := []kdbx.StagingEntry{{
		UUID:        "0088f6f3-f9b3-43a6-b7ae-d0ee15d35f55",
		Fragment:    []byte("<String><Key>Title</Key><Value>seas-nve.dk</Value></String>"),
		ModifiedAt:  time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC),
		ParentGroup: parentUUID,
	}}
	local := []kdbx.Group{{
		UUID:       parentUUID,
		ParentUUID: "",
		Name:       "Hjem",
		IconID:     48,
		ModifiedAt: time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC),
	}}

	groups := mergeGroupTree(local, nil, nil) // delta uden gruppen
	xmlBytes, err := kdbx.BuildStagingXMLWithGroups(entries, groups, nil, "")
	if err != nil {
		t.Fatalf("build staging: %v", err)
	}
	out := string(xmlBytes)
	if n := strings.Count(out, "<Group>"); n < 2 {
		t.Fatalf("entry placeret i Root i stedet for sin gruppe: kun %d <Group> i %s", n, out)
	}
	if !strings.Contains(out, "<Name>Hjem</Name>") {
		t.Fatalf("forældregruppen blev ikke genskabt i staging-træet: %s", out)
	}
}

// En gruppe brugeren har taget ud af søgeresultater må ikke få flaget nulstillet
// af et pull. Wire-formatet kender ikke flaget, så det skal bæres videre fra
// det lokale træ.
func TestPullPreservesEnableSearching(t *testing.T) {
	const uuid = "1a4d6d09-1eb7-4a5f-9e2b-0e9a2a9f1c33"
	off := false
	local := []kdbx.Group{{UUID: uuid, Name: "Skjult", IconID: 48, EnableSearching: &off}}

	// Gruppen er også ændret på serveren, så den er med i delta — uden flag.
	delta := []kdbx.StagingGroup{{UUID: uuid, Name: "Skjult", IconID: 48}}

	got := mergeGroupTree(local, delta, nil)
	if len(got) != 1 {
		t.Fatalf("forventede 1 gruppe, fik %d", len(got))
	}
	if got[0].EnableSearching == nil || *got[0].EnableSearching {
		t.Fatalf("søgeflaget gik tabt: %v", got[0].EnableSearching)
	}

	xmlBytes, err := kdbx.BuildStagingXMLWithGroups(nil, got, nil, "")
	if err != nil {
		t.Fatalf("build staging: %v", err)
	}
	if !strings.Contains(string(xmlBytes), "<EnableSearching>False</EnableSearching>") {
		t.Fatalf("staging-XML nulstiller søgeflaget: %s", xmlBytes)
	}
}

// Spærren mod massesletning af grupper. En eksport der mangler størstedelen af
// de kendte grupper er et symptom (mislykket merge, restore fra gammel backup),
// ikke en brugerhandling — og tombstones propagerer til alle enheder.
func TestRefuseGroupDeletion(t *testing.T) {
	cases := []struct {
		name          string
		doomed, known int
		want          bool
	}{
		{"ingen sletninger", 0, 107, false},
		{"én gruppe ryddet op", 1, 107, false},
		{"en håndfuld, under gulvet", 5, 107, false},
		{"seks af 107 — normalt arbejde", 6, 107, false},
		{"27 af 107 — hændelsen 2026-08-26", 27, 107, true},
		{"alle kendte forsvundet", 107, 107, true},
		{"lille database, halvdelen væk", 6, 10, true},
	}
	for _, c := range cases {
		if got := refuseGroupDeletion(c.doomed, c.known); got != c.want {
			t.Errorf("%s: refuseGroupDeletion(%d, %d) = %v, ville have %v",
				c.name, c.doomed, c.known, got, c.want)
		}
	}
}
