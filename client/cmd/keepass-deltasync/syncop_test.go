// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"testing"
	"time"
)

func TestRewriteLastModificationTime_Basic(t *testing.T) {
	fragment := []byte(`
		<UUID>foo==</UUID>
		<Times>
			<LastModificationTime>2026-05-27T09:35:00Z</LastModificationTime>
			<CreationTime>2026-05-01T00:00:00Z</CreationTime>
		</Times>
		<String><Key>Title</Key><Value>github</Value></String>
	`)
	target, _ := time.Parse(time.RFC3339, "2026-05-27T10:55:18Z")
	out := rewriteLastModificationTime(fragment, target)

	if !bytes.Contains(out, []byte("<LastModificationTime>2026-05-27T10:55:18Z</LastModificationTime>")) {
		t.Fatalf("expected rewritten timestamp not found in:\n%s", out)
	}
	if bytes.Contains(out, []byte("2026-05-27T09:35:00Z")) {
		t.Fatalf("original timestamp should be gone, still in:\n%s", out)
	}
}

func TestRewriteLastModificationTime_OnlyFirst(t *testing.T) {
	// Verify the FIRST occurrence is rewritten (entry's own Times) but the
	// History block's LastModificationTime is left alone.
	fragment := []byte(`<Times><LastModificationTime>2026-05-27T09:35:00Z</LastModificationTime></Times><History><Entry><Times><LastModificationTime>2026-05-20T08:00:00Z</LastModificationTime></Times></Entry></History>`)
	target, _ := time.Parse(time.RFC3339, "2026-05-27T10:55:18Z")
	out := rewriteLastModificationTime(fragment, target)

	if !bytes.Contains(out, []byte("<LastModificationTime>2026-05-27T10:55:18Z</LastModificationTime>")) {
		t.Fatalf("expected new timestamp:\n%s", out)
	}
	if !bytes.Contains(out, []byte("<LastModificationTime>2026-05-20T08:00:00Z</LastModificationTime>")) {
		t.Fatalf("historical timestamp should be preserved:\n%s", out)
	}
}

func TestRewriteLastModificationTime_NoMatch(t *testing.T) {
	fragment := []byte(`<UUID>foo==</UUID><String><Key>Title</Key><Value>github</Value></String>`)
	target, _ := time.Parse(time.RFC3339, "2026-05-27T10:55:18Z")
	out := rewriteLastModificationTime(fragment, target)

	if !bytes.Equal(out, fragment) {
		t.Fatal("no LastModificationTime present should return fragment unchanged")
	}
}
