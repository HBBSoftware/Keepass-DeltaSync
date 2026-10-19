// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/base64"
	"strings"
	"testing"

	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/crypto"
)

const testDBID = "0b6f6c1e-5b0a-4c55-9d7a-2f1a3c4d5e6f"

// wrappedFor laver en wrapped_master_key til enheden — som serveren ville
// kunne, for den kender enhedens public-key.
func wrappedFor(t *testing.T, devicePub, masterKey []byte) *string {
	t.Helper()
	w, err := crypto.WrapKey(masterKey, devicePub)
	if err != nil {
		t.Fatal(err)
	}
	s := base64.StdEncoding.EncodeToString(w)
	return &s
}

func testDevice(t *testing.T) (pub, priv []byte) {
	t.Helper()
	pub, priv, err := crypto.GenerateBoxKeypair()
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

// Angrebet: serveren kalder ejeren for medlem og giver sin egen nøgle med.
// Uden kontrollen ville klienten bruge den til alt den pusher.
func TestResolveKeys_OwnerCalledMemberIsRefused(t *testing.T) {
	pub, priv := testDevice(t)
	db := &config.Database{Name: "familie", RemoteID: testDBID, Role: "owner"}
	serverKey := make([]byte, 32) // nøgle serveren kender
	_, _, err := resolveMasterEntryKeys([]byte("pw"), "member", db, wrappedFor(t, pub, serverKey), priv)
	if err == nil || !strings.Contains(err.Error(), "set it up as owner") {
		t.Fatalf("expected refusal, got %v", err)
	}
}

// Et rigtigt medlem, hvor serveren bytter wrapped_master_key ud.
func TestResolveKeys_MemberKeySwapIsRefused(t *testing.T) {
	pub, priv := testDevice(t)
	realKey := []byte(strings.Repeat("r", 32))
	_, realEntry, err := keysForRole(nil, "member", testDBID, wrappedFor(t, pub, realKey), priv)
	if err != nil {
		t.Fatal(err)
	}
	db := &config.Database{Name: "familie", RemoteID: testDBID, Role: "member", KeyCheck: crypto.KeyCheck(realEntry)}

	if _, _, err := resolveMasterEntryKeys(nil, "member", db, wrappedFor(t, pub, realKey), priv); err != nil {
		t.Fatalf("the real key must still work: %v", err)
	}
	swapped := []byte(strings.Repeat("s", 32))
	_, _, err = resolveMasterEntryKeys(nil, "member", db, wrappedFor(t, pub, swapped), priv)
	if err == nil || !strings.Contains(err.Error(), "not the one this device has been using") {
		t.Fatalf("expected refusal of swapped key, got %v", err)
	}
}

// Ejer med kendt nøgle og forkert password: en klar besked, ikke MAC-fejl senere.
func TestResolveKeys_OwnerWrongPassword(t *testing.T) {
	_, entry, err := keysForRole([]byte("rigtigt"), "owner", testDBID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	db := &config.Database{Name: "familie", RemoteID: testDBID, Role: "owner", KeyCheck: crypto.KeyCheck(entry)}
	if _, _, err := resolveMasterEntryKeys([]byte("rigtigt"), "owner", db, nil, nil); err != nil {
		t.Fatalf("right password refused: %v", err)
	}
	_, _, err = resolveMasterEntryKeys([]byte("forkert"), "owner", db, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "right password") {
		t.Fatalf("expected wrong-password error, got %v", err)
	}
}

// En binding fra før kontrollen: rollen huskes ved første kørsel, og en
// pre-v2-server uden rolle betyder ejer.
func TestResolveKeys_LegacyBindingAdoptsRole(t *testing.T) {
	db := &config.Database{Name: "familie", RemoteID: testDBID}
	if _, _, err := resolveMasterEntryKeys([]byte("pw"), "", db, nil, nil); err != nil {
		t.Fatal(err)
	}
	if db.Role != "owner" {
		t.Fatalf("role = %q, want owner", db.Role)
	}
	if db.KeyCheck != "" {
		t.Fatal("key check must not be recorded from an unverified password")
	}
	pub, priv := testDevice(t)
	if _, _, err := resolveMasterEntryKeys(nil, "member", db, wrappedFor(t, pub, make([]byte, 32)), priv); err == nil {
		t.Fatal("role adopted as owner must not flip to member afterwards")
	}
}

func TestRecordKeyCheckOnlyOnce(t *testing.T) {
	db := &config.Database{}
	recordKeyCheck(db, []byte(strings.Repeat("a", 32)))
	first := db.KeyCheck
	recordKeyCheck(db, []byte(strings.Repeat("b", 32)))
	if db.KeyCheck != first || first == "" {
		t.Fatal("an existing key check must not be overwritten")
	}
}
