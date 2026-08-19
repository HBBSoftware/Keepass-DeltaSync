// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// frame pakker en besked som Firefox gør: fire længdebytes i værtens egen
// byte-orden, derefter JSON.
func frame(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var hdr [4]byte
	binary.NativeEndian.PutUint32(hdr[:], uint32(len(body)))
	return append(hdr[:], body...)
}

// readFrames pakker en strøm af svar ud igen.
func readFrames(t *testing.T, raw []byte) []hostResponse {
	t.Helper()
	var out []hostResponse
	r := bytes.NewReader(raw)
	for r.Len() > 0 {
		msg, err := readNativeMessage(r)
		if err != nil {
			t.Fatalf("readNativeMessage: %v", err)
		}
		var resp hostResponse
		if err := json.Unmarshal(msg, &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		out = append(out, resp)
	}
	return out
}

func newTestHost() *browserHost {
	return &browserHost{
		cfg: &config.Config{
			Databases: []config.Database{{Name: "privat", RemoteID: "remote-1", LocalPath: "privat.kdbx"}},
		},
		idleLock:   time.Minute,
		index:      make(map[string][]indexEntry),
		watching:   make(map[string]bool),
		fallbackPW: make(map[string][]byte),
		pwTimers:   make(map[string]*time.Timer),
	}
}

func TestServe_FramingRoundTrip(t *testing.T) {
	in := bytes.NewBuffer(nil)
	in.Write(frame(t, hostRequest{ID: 1, Cmd: "status"}))
	in.Write(frame(t, hostRequest{ID: 2, Cmd: "wat"}))

	var out bytes.Buffer
	h := newTestHost()
	if err := h.serve(in, &out); err != nil {
		t.Fatalf("serve: %v", err)
	}

	resps := readFrames(t, out.Bytes())
	if len(resps) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(resps))
	}
	if resps[0].ID != 1 || !resps[0].OK {
		t.Errorf("status response = %+v", resps[0])
	}
	if len(resps[0].Databases) != 1 || resps[0].Databases[0].Name != "privat" {
		t.Errorf("status should list the registered database, got %+v", resps[0].Databases)
	}
	if resps[0].Databases[0].Unlocked {
		t.Error("database should not report as unlocked before unlock")
	}
	// Svaret skal bære requestens ID, ellers kan udvidelsen ikke parre
	// svar med kald når flere er i luften samtidig.
	if resps[1].ID != 2 || resps[1].OK || !strings.Contains(resps[1].Error, "wat") {
		t.Errorf("unknown-command response = %+v", resps[1])
	}
}

func TestServe_RejectsMalformedJSON(t *testing.T) {
	var in bytes.Buffer
	body := []byte("{not json")
	var hdr [4]byte
	binary.NativeEndian.PutUint32(hdr[:], uint32(len(body)))
	in.Write(hdr[:])
	in.Write(body)

	var out bytes.Buffer
	h := newTestHost()
	if err := h.serve(&in, &out); err != nil {
		t.Fatalf("serve should survive a malformed message: %v", err)
	}
	resps := readFrames(t, out.Bytes())
	if len(resps) != 1 || resps[0].OK {
		t.Fatalf("expected one failure response, got %+v", resps)
	}
}

func TestReadNativeMessage_RejectsOversizedFrame(t *testing.T) {
	var hdr [4]byte
	binary.NativeEndian.PutUint32(hdr[:], maxIncoming+1)
	_, err := readNativeMessage(bytes.NewReader(hdr[:]))
	if err == nil {
		t.Fatal("expected an oversized frame to be rejected before allocating")
	}
}

func TestIndexPage_RequiresUnlock(t *testing.T) {
	h := newTestHost()
	resp := h.handle(hostRequest{Cmd: "index", DB: "privat"})
	if resp.OK || !strings.Contains(resp.Error, "locked") {
		t.Fatalf("index on a locked database should fail clearly, got %+v", resp)
	}
}

func TestIndexPage_WalksPages(t *testing.T) {
	h := newTestHost()
	idx, err := buildIndex("privat", []byte(browserFixtureXML))
	if err != nil {
		t.Fatalf("buildIndex: %v", err)
	}
	h.index["privat"] = idx

	resp := h.handle(hostRequest{Cmd: "index", DB: "privat"})
	if !resp.OK {
		t.Fatalf("index: %+v", resp)
	}
	if resp.Count != len(idx) {
		t.Errorf("count = %d, want %d", resp.Count, len(idx))
	}
	if resp.Next != len(idx) {
		t.Errorf("next = %d, want %d — the whole fixture fits in one page", resp.Next, len(idx))
	}
	if len(resp.Entries) != len(idx) {
		t.Errorf("got %d entries, want %d", len(resp.Entries), len(idx))
	}
}

func TestFindDB_SingleDatabaseNeedsNoName(t *testing.T) {
	h := newTestHost()
	if db := h.findDB(""); db == nil || db.Name != "privat" {
		t.Fatalf("an empty name should resolve to the only database, got %+v", db)
	}

	h.cfg.Databases = append(h.cfg.Databases, config.Database{Name: "arbejde", RemoteID: "remote-2"})
	if db := h.findDB(""); db != nil {
		t.Fatal("an empty name must be ambiguous when several databases are registered")
	}
	if db := h.findDB("arbejde"); db == nil {
		t.Fatal("named lookup failed")
	}
}

// TestZeroAll_ClearsPasswordAndIndex dækker `lock`: både det cachede
// fallback-password og indekset skal være væk bagefter.
func TestZeroAll_ClearsPasswordAndIndex(t *testing.T) {
	h := newTestHost()
	pw := []byte("hunter2")
	h.rememberPassword("privat", pw)
	h.index["privat"] = []indexEntry{{Title: "x"}}

	h.zeroAll()

	if len(h.fallbackPW) != 0 || len(h.index) != 0 {
		t.Fatalf("lock left state behind: %d passwords, %d indexes", len(h.fallbackPW), len(h.index))
	}
	if !bytes.Equal(pw, make([]byte, len(pw))) {
		t.Fatalf("password buffer was not zeroed: %q", pw)
	}
}

// TestForgetPassword_KeepsIndex: idle-låsen tager nøglematerialet, men ikke
// søgeindekset — brugeren skal kunne blive ved med at søge.
func TestForgetPassword_KeepsIndex(t *testing.T) {
	h := newTestHost()
	h.rememberPassword("privat", []byte("hunter2"))
	h.index["privat"] = []indexEntry{{Title: "x"}}

	h.forgetPassword("privat")

	if len(h.fallbackPW) != 0 {
		t.Error("idle lock should have zeroed the cached password")
	}
	if len(h.index["privat"]) != 1 {
		t.Error("idle lock must not drop the search index")
	}
}
