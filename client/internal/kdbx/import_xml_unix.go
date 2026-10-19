// SPDX-License-Identifier: GPL-3.0-or-later

//go:build unix

package kdbx

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// stageXML lægger XML'en i en navngiven pipe (FIFO) på xmlPath og kører run,
// der læser fra den. En pipe har intet indhold på disken; dataene går direkte
// fra denne proces til keepassxc-cli gennem kernen.
//
// Faldgruben: at åbne en FIFO til skrivning blokerer, indtil nogen åbner den
// til læsning. Fejler keepassxc-cli før den når så langt (forkert kodeord,
// manglende binær, fuld disk), ville skrivetråden hænge for evigt. Derfor
// åbner vi selv læsesiden, når run er færdig og skriveren ikke er: så kommer
// den løs, får EPIPE ved første skrivning og slutter.
func stageXML(xmlPath string, xml []byte, run func() error) error {
	if err := syscall.Mkfifo(xmlPath, 0o600); err != nil {
		return fmt.Errorf("mkfifo: %w", err)
	}
	defer os.Remove(xmlPath)

	written := make(chan error, 1)
	go func() {
		f, err := os.OpenFile(xmlPath, os.O_WRONLY, 0)
		if err != nil {
			written <- err
			return
		}
		_, err = f.Write(xml)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		written <- err
	}()

	runErr := run()

	var writeErr error
	select {
	case writeErr = <-written:
	default:
		writeErr = releaseWriter(xmlPath, written)
	}

	if runErr != nil {
		return runErr
	}
	if writeErr != nil {
		return fmt.Errorf("write staging xml to pipe: %w", writeErr)
	}
	return nil
}

// releaseWriter får en skrivetråd løs, som venter på en læser der aldrig kom.
// Den åbner læsesiden ikke-blokerende og holder den åben, til skriveren har
// meldt tilbage — lukkes den for tidligt, kan skriverens open() på nogle
// systemer gå glip af at der var en læser og blive ved at vente. Skriveren får
// så EPIPE (eller skriver ud i en pipe ingen læser færdig) og slutter.
func releaseWriter(xmlPath string, written <-chan error) error {
	deadline := time.After(10 * time.Second)
	for {
		r, err := os.OpenFile(xmlPath, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			select {
			case werr := <-written:
				return werr
			case <-deadline:
				return errors.New("staging pipe writer did not finish")
			case <-time.After(50 * time.Millisecond):
				continue
			}
		}
		select {
		case werr := <-written:
			_ = r.Close()
			return werr
		case <-time.After(100 * time.Millisecond):
			// Skriveren kan være kommet igennem open() og nu blokere på en
			// fuld pipe; læs den tom, så den kan skrive færdig.
			_, _ = r.Read(make([]byte, 64*1024))
			_ = r.Close()
		case <-deadline:
			_ = r.Close()
			return errors.New("staging pipe writer did not finish")
		}
	}
}
