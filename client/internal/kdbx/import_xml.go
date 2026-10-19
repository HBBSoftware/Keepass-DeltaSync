// SPDX-License-Identifier: GPL-3.0-or-later

package kdbx

import (
	"context"
	"os"
	"path/filepath"
)

// ImportXML bygger kdbxPath ud fra et KeePass-XML-dokument i hukommelsen —
// samme resultat som Import, men uden at kalderen skriver XML'en til disk.
//
// XML'en er dekrypterede poster i klartekst: alle kodeord der er ændret siden
// sidste pull, og ved init-shared hele databasen. keepassxc-cli import læser
// kun fra en sti (stdin går til kodeordet), så noget skal ligge under den sti.
// Hvor det kan lade sig gøre, er det en navngiven pipe; se stageXML for hvad
// hver platform gør.
//
// tmpDir skal være en mappe kun denne proces bruger; kalderen fjerner den.
func (c *CLI) ImportXML(ctx context.Context, xml []byte, tmpDir, kdbxPath string, password []byte) error {
	xmlPath := filepath.Join(tmpDir, "staging.xml")
	run := func() error { return c.Import(ctx, xmlPath, kdbxPath, password) }
	return stageXML(xmlPath, xml, run)
}

// scrubAndRemove overskriver en fil med nuller før den slettes. På SSD'er og
// copy-on-write-filsystemer kan de gamle blokke stadig overleve, så det er en
// formindskelse af risikoen, ikke en garanti — men det fjerner klarteksten
// fra det sted man først ville kigge.
func scrubAndRemove(path string) {
	if f, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
		if st, err := f.Stat(); err == nil && st.Size() > 0 {
			_, _ = f.Write(make([]byte, st.Size()))
			_ = f.Sync()
		}
		_ = f.Close()
	}
	_ = os.Remove(path)
}
