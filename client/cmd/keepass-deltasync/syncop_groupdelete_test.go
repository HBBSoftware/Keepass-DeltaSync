// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx"
)

func set(uuids ...string) map[string]bool {
	m := make(map[string]bool, len(uuids))
	for _, u := range uuids {
		m[u] = true
	}
	return m
}

// En gruppe der er væk fra træet skal tombstones — men kun de sporløse tæller
// med i spærrens mistanke-regnskab.
func TestDoomedGroups(t *testing.T) {
	cases := []struct {
		name            string
		known           []string
		current         map[string]bool
		trashed         []string
		wantDoomed      int
		wantUnexplained int
	}{
		{
			name:    "intet slettet",
			known:   []string{"a", "b"},
			current: set("a", "b"),
		},
		{
			name:            "mappe smidt i papirkurven",
			known:           []string{"a", "b"},
			current:         set("a"),
			trashed:         []string{"b"},
			wantDoomed:      1,
			wantUnexplained: 0,
		},
		{
			name:            "gruppe væk uden spor",
			known:           []string{"a", "b"},
			current:         set("a"),
			wantDoomed:      1,
			wantUnexplained: 1,
		},
		{
			name:            "stor mappe med undermapper — alt i papirkurven",
			known:           []string{"a", "b", "c", "d", "e", "f", "g"},
			current:         set("a"),
			trashed:         []string{"b", "c", "d", "e", "f", "g"},
			wantDoomed:      6,
			wantUnexplained: 0,
		},
		{
			name:            "blandet: én i papirkurven, resten sporløse",
			known:           []string{"a", "b", "c", "d"},
			current:         set("a"),
			trashed:         []string{"b"},
			wantDoomed:      3,
			wantUnexplained: 2,
		},
		{
			name:    "trashed gruppe der stadig er i træet tæller ikke som slettet",
			known:   []string{"a"},
			current: set("a"),
			trashed: []string{"a"},
		},
	}
	for _, c := range cases {
		doomed, unexplained := doomedGroups(c.known, c.current, c.trashed)
		if len(doomed) != c.wantDoomed {
			t.Errorf("%s: %d doomed, ville have %d (%v)", c.name, len(doomed), c.wantDoomed, doomed)
		}
		if unexplained != c.wantUnexplained {
			t.Errorf("%s: %d uforklarede, ville have %d", c.name, unexplained, c.wantUnexplained)
		}
	}
}

// Sammensat: den store mappe-sletning må IKKE ramme spærren, mens den samme
// mængde sporløst forsvundne grupper skal.
func TestBigFolderDeleteIsNotRefused(t *testing.T) {
	known := make([]string, 0, 40)
	for _, u := range []string{"keep"} {
		known = append(known, u)
	}
	trashedList := make([]string, 0, 39)
	for i := 0; i < 39; i++ {
		u := string(rune('A'+i%26)) + string(rune('a'+i/26))
		known = append(known, u)
		trashedList = append(trashedList, u)
	}

	doomed, unexplained := doomedGroups(known, set("keep"), trashedList)
	if len(doomed) != 39 {
		t.Fatalf("doomed = %d, ville have 39", len(doomed))
	}
	if refuseGroupDeletion(unexplained, len(known)) {
		t.Errorf("en sletning af 39 mapper der alle ligger i papirkurven blev afvist")
	}
	// Samme sletning uden papirkurvs-bevis skal stadig afvises.
	if _, unexplained := doomedGroups(known, set("keep"), nil); !refuseGroupDeletion(unexplained, len(known)) {
		t.Errorf("39 sporløst forsvundne grupper blev ikke afvist")
	}
}

// En gruppe hvis tombstone vi lige har pullet skal ud af KnownGroups. Ellers
// ser næste push den som "forsvundet uden spor": den genudsender sletningen og
// tæller med i sikkerhedsspærrens mistanke — permanent, fordi spærren bevarer
// KnownGroups netop når den nægter.
func TestPruneKnownGroups(t *testing.T) {
	known := []string{"grp-a", "grp-b", "grp-c"}
	deletions := []kdbx.StagingDeletion{
		{UUID: "grp-b"},
		{UUID: "en-entry-der-aldrig-var-en-gruppe"},
	}

	got := pruneKnownGroups(known, deletions)
	if len(got) != 2 || got[0] != "grp-a" || got[1] != "grp-c" {
		t.Fatalf("pruneKnownGroups = %v, ville have [grp-a grp-c]", got)
	}

	// Og så er der ingen falsk mistanke tilbage til spærren.
	if _, unexplained := doomedGroups(got, set("grp-a", "grp-c"), nil); unexplained != 0 {
		t.Errorf("%d uforklarede grupper efter prune, ville have 0", unexplained)
	}
	// Uden prune ville den samme sync se grp-b som sporløst forsvundet.
	if _, unexplained := doomedGroups(known, set("grp-a", "grp-c"), nil); unexplained != 1 {
		t.Errorf("kontrolprøve: %d uforklarede uden prune, ville have 1", unexplained)
	}
}

func TestPruneKnownGroupsEmptyInputs(t *testing.T) {
	if got := pruneKnownGroups(nil, []kdbx.StagingDeletion{{UUID: "x"}}); len(got) != 0 {
		t.Errorf("tomt kendt sæt gav %v", got)
	}
	known := []string{"a"}
	if got := pruneKnownGroups(known, nil); len(got) != 1 {
		t.Errorf("ingen deletions skal lade sættet stå: %v", got)
	}
}
