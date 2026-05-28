// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

// TestCoalescer_SingleRequest verificerer at ét request kører ét stykke arbejde.
func TestCoalescer_SingleRequest(t *testing.T) {
	var c coalescer
	var runs int32
	done := make(chan struct{})

	c.request(func() {
		atomic.AddInt32(&runs, 1)
		close(done)
	})

	<-done
	c.wait()

	if got := atomic.LoadInt32(&runs); got != 1 {
		t.Fatalf("expected 1 run, got %d", got)
	}
}

// TestCoalescer_MultipleRequestsCoalesce verificerer at flere request-kald
// under et igangværende run resulterer i præcis én ekstra runde, ikke N.
//
// Vi tilbageholder første run via en gate-kanal, fyrer N=10 ekstra requests
// mens den første kører, frigiver gate'en, og forventer total runs = 2.
func TestCoalescer_MultipleRequestsCoalesce(t *testing.T) {
	var c coalescer
	var runs int32
	gate := make(chan struct{})
	firstStarted := make(chan struct{})

	work := func() {
		n := atomic.AddInt32(&runs, 1)
		if n == 1 {
			close(firstStarted)
			<-gate
		}
	}

	c.request(work)
	<-firstStarted

	for i := 0; i < 10; i++ {
		c.request(work)
	}

	close(gate)
	c.wait()

	if got := atomic.LoadInt32(&runs); got != 2 {
		t.Fatalf("expected 2 runs (1 initial + 1 coalesced), got %d", got)
	}
}

// TestCoalescer_RequestAfterCompletionStartsFresh verificerer at en request
// efter at runLoop er afsluttet starter en ny goroutine — coalescer'en må
// ikke "huske" sin running-state efter end.
func TestCoalescer_RequestAfterCompletionStartsFresh(t *testing.T) {
	var c coalescer
	var runs int32

	for i := 0; i < 3; i++ {
		done := make(chan struct{})
		c.request(func() {
			atomic.AddInt32(&runs, 1)
			close(done)
		})
		<-done
		c.wait()
	}

	if got := atomic.LoadInt32(&runs); got != 3 {
		t.Fatalf("expected 3 runs across 3 sequential requests, got %d", got)
	}
}

// TestEventConcernsFile filtrerer fsnotify-events korrekt: kun events der
// vedrører vores basename OG har en write-lignende op-flag bør trigge sync.
func TestEventConcernsFile(t *testing.T) {
	tests := []struct {
		name string
		ev   fsnotify.Event
		base string
		want bool
	}{
		{
			name: "Write på vores fil",
			ev:   fsnotify.Event{Name: filepath.Join("dir", "vault.kdbx"), Op: fsnotify.Write},
			base: "vault.kdbx",
			want: true,
		},
		{
			name: "Create på vores fil (keepassxc atomic save)",
			ev:   fsnotify.Event{Name: filepath.Join("dir", "vault.kdbx"), Op: fsnotify.Create},
			base: "vault.kdbx",
			want: true,
		},
		{
			name: "Rename på vores fil",
			ev:   fsnotify.Event{Name: filepath.Join("dir", "vault.kdbx"), Op: fsnotify.Rename},
			base: "vault.kdbx",
			want: true,
		},
		{
			name: "Anden fil i samme dir",
			ev:   fsnotify.Event{Name: filepath.Join("dir", "other.kdbx"), Op: fsnotify.Write},
			base: "vault.kdbx",
			want: false,
		},
		{
			name: "Tmp-fil under atomic save",
			ev:   fsnotify.Event{Name: filepath.Join("dir", "vault.kdbx.tmp"), Op: fsnotify.Write},
			base: "vault.kdbx",
			want: false,
		},
		{
			name: "Chmod på vores fil — irrelevant op",
			ev:   fsnotify.Event{Name: filepath.Join("dir", "vault.kdbx"), Op: fsnotify.Chmod},
			base: "vault.kdbx",
			want: false,
		},
		{
			name: "Remove på vores fil — irrelevant op (vi sletter ikke kdbx i sync)",
			ev:   fsnotify.Event{Name: filepath.Join("dir", "vault.kdbx"), Op: fsnotify.Remove},
			base: "vault.kdbx",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eventConcernsFile(tt.ev, tt.base)
			if got != tt.want {
				t.Fatalf("eventConcernsFile(%+v, %q) = %v, want %v", tt.ev, tt.base, got, tt.want)
			}
		})
	}
}

// TestCoalescer_WaitReturnsWhenIdle verificerer at wait() returnerer hurtigt
// hvis intet kører — det skal ikke poll'e forever.
func TestCoalescer_WaitReturnsWhenIdle(t *testing.T) {
	var c coalescer
	done := make(chan struct{})
	go func() {
		c.wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("wait() blokerede selvom intet kørte")
	}
}
