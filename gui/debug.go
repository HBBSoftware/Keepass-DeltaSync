// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// debugf skriver en linje til gui.log ved siden af gui.json — men KUN hvis
// miljøvariablen KDS_GUI_DEBUG er sat. Det gør det muligt at fejlsøge en
// opstart der "hænger" uden at en almindelig kørsel skriver til disk.
//
// Kør f.eks. (PowerShell):
//
//	$env:KDS_GUI_DEBUG=1; .\keepass-deltasync-gui.exe
//
// og kig derefter i gui.log.
func debugf(format string, args ...any) {
	if os.Getenv("KDS_GUI_DEBUG") == "" {
		return
	}
	p, err := settingsPath()
	if err != nil {
		return
	}
	logPath := filepath.Join(filepath.Dir(p), "gui.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	stamp := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Fprintf(f, "%s  "+format+"\n", append([]any{stamp}, args...)...)
}
