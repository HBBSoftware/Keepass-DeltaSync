// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"regexp"
	"testing"
)

// TestNewLocalID sikrer at id'et er et brugbart UUIDv4 og at to kald ikke
// giver det samme. Kollisionen ville betyde to databaser i ét keyring-slot.
func TestNewLocalID(t *testing.T) {
	seen := make(map[string]bool, 64)
	for i := 0; i < 64; i++ {
		id, err := newLocalID()
		if err != nil {
			t.Fatalf("newLocalID: %v", err)
		}
		if !uuidRe.MatchString(id) {
			t.Fatalf("not a UUID: %q", id)
		}
		if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4`).MatchString(id) {
			t.Fatalf("not version 4: %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id after %d draws: %q", i, id)
		}
		seen[id] = true
	}
}

// TestErrLocalOnly: fejlen skal nævne databasen OG vejen videre. Uden det
// ligner en lokal-kun binding bare en database der ikke virker.
func TestErrLocalOnly(t *testing.T) {
	err := errLocalOnly("private")
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	for _, want := range []string{"private", "add-local", "forget", "init"} {
		if !regexp.MustCompile(regexp.QuoteMeta(want)).MatchString(msg) {
			t.Fatalf("error message does not mention %q: %s", want, msg)
		}
	}
}
