// SPDX-License-Identifier: GPL-3.0-or-later

package kdbx

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Hele vejen gennem den rigtige keepassxc-cli: XML ind via ImportXML, poster
// ud igen med export.
func TestImportXML_RoundTrip(t *testing.T) {
	bin, err := exec.LookPath("keepassxc-cli")
	if err != nil {
		t.Skipf("keepassxc-cli not available: %v", err)
	}
	c := &CLI{binary: bin}
	dir := t.TempDir()
	pw := []byte("test-password")
	src := filepath.Join(dir, "src.kdbx")

	create := exec.Command(bin, "db-create", "-p", src)
	create.Stdin = strings.NewReader("test-password\ntest-password\n")
	if out, err := create.CombinedOutput(); err != nil {
		t.Fatalf("db-create: %v\n%s", err, out)
	}
	add := exec.Command(bin, "add", "-u", "alice", src, "Staged entry")
	add.Stdin = strings.NewReader("test-password\n")
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	ctx := context.Background()
	xml, err := c.Export(ctx, src, pw)
	if err != nil {
		t.Fatal(err)
	}

	stage := filepath.Join(dir, "stage")
	if err := os.Mkdir(stage, 0o700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(stage, "out.kdbx")
	if err := c.ImportXML(ctx, xml, stage, out, pw); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(stage, "staging.xml")); !os.IsNotExist(err) {
		t.Fatalf("staging.xml left behind: %v", err)
	}
	back, err := c.Export(ctx, out, pw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(back, []byte("Staged entry")) {
		t.Fatal("imported database lacks the entry")
	}
}

// Et program der aldrig åbner pipen må ikke efterlade skrivetråden hængende —
// det ville låse sync-dæmonen ved første forkerte kodeord.
func TestImportXML_ReaderNeverOpens(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no fifo on windows")
	}
	assertFailsQuickly(t, "#!/bin/sh\nexit 1\n", []byte("<KeePassFile/>"))
}

// Et program der læser lidt og dør, mens skriveren er midt i en XML større
// end pipe-bufferen (64 KiB på Linux).
func TestImportXML_ReaderDiesMidway(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no fifo on windows")
	}
	big := bytes.Repeat([]byte("<Entry/>"), 100_000) // ~800 KB
	assertFailsQuickly(t, "#!/bin/sh\nhead -c 100 \"$4\" >/dev/null\nexit 1\n", big)
}

func assertFailsQuickly(t *testing.T, script string, xml []byte) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-cli")
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	c := &CLI{binary: bin}
	done := make(chan error, 1)
	go func() {
		done <- c.ImportXML(context.Background(), xml, dir, filepath.Join(dir, "out.kdbx"), []byte("pw"))
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error from a failing keepassxc-cli")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("ImportXML hung after keepassxc-cli failed")
	}
}

// fakeCLI skriver et script der logger om det fik en pipe eller en fil, og
// kun lykkes med en almindelig fil — som en keepassxc-cli der ikke kan læse
// fra en pipe.
func fakeCLI(t *testing.T, dir string) (*CLI, string) {
	t.Helper()
	log := filepath.Join(dir, "calls.log")
	bin := filepath.Join(dir, "fake-cli")
	script := "#!/bin/sh\nif [ -p \"$4\" ]; then echo pipe >> " + log + "; exit 1; fi\n" +
		"echo file >> " + log + "\ncat \"$4\" >/dev/null\ntouch \"$5\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return &CLI{binary: bin}, log
}

// Med reserveløsningen slået til (som på macOS): pipen fejler, filen lykkes,
// og næste import går direkte til filen.
func TestImportXML_FallbackToFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no fifo on windows")
	}
	pipeBroken.Store(false)
	t.Cleanup(func() { pipeFallback = runtime.GOOS == "darwin"; pipeBroken.Store(false) })
	pipeFallback = true

	dir := t.TempDir()
	c, log := fakeCLI(t, dir)
	for i := 0; i < 2; i++ {
		if err := c.ImportXML(context.Background(), []byte("<KeePassFile/>"), dir, filepath.Join(dir, "out.kdbx"), []byte("pw")); err != nil {
			t.Fatalf("import %d: %v", i, err)
		}
	}
	calls, _ := os.ReadFile(log)
	if got := strings.Fields(string(calls)); strings.Join(got, ",") != "pipe,file,file" {
		t.Fatalf("calls = %v, want pipe,file,file", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "staging.xml")); !os.IsNotExist(err) {
		t.Fatal("staging.xml left behind")
	}
}

// Uden reserveløsningen (Linux): en fejl gennem pipen må ikke føre til at
// klarteksten skrives til en fil.
func TestImportXML_NoFallbackOnLinux(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no fifo on windows")
	}
	pipeBroken.Store(false)
	t.Cleanup(func() { pipeFallback = runtime.GOOS == "darwin" })
	pipeFallback = false

	dir := t.TempDir()
	c, log := fakeCLI(t, dir)
	if err := c.ImportXML(context.Background(), []byte("<KeePassFile/>"), dir, filepath.Join(dir, "out.kdbx"), []byte("pw")); err == nil {
		t.Fatal("expected the pipe failure to be returned")
	}
	calls, _ := os.ReadFile(log)
	if strings.TrimSpace(string(calls)) != "pipe" {
		t.Fatalf("calls = %q, want only the pipe attempt", calls)
	}
}
