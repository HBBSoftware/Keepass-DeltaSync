// SPDX-License-Identifier: GPL-3.0-or-later

package kdbx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
)

// pipeFallback slår reserveløsningen til: fejler en import gennem pipen,
// prøves den igen med en almindelig fil. Kun på macOS, hvor pipen ikke er
// afprøvet mod keepassxc-cli — det er bedre at klarteksten ligger kortvarigt
// på disken, som før pipen, end at sync holder op. På Linux er pipen afprøvet,
// og der må en import der fejler af andre grunde (forkert password) ikke føre
// til at klarteksten skrives til disk. En var, så testene kan slå den til.
var pipeFallback = runtime.GOOS == "darwin"

// pipeBroken huskes resten af processens levetid, når reserveløsningen har
// virket: så går daemonen direkte til filen i stedet for at fejle hver gang.
var pipeBroken atomic.Bool

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
	if pipeBroken.Load() {
		return stageXMLFile(xmlPath, xml, run)
	}
	err := stageXML(xmlPath, xml, run)
	if err == nil || !pipeFallback {
		return err
	}
	if ferr := stageXMLFile(xmlPath, xml, run); ferr != nil {
		return err // fejlede også fra en fil: det var ikke pipen
	}
	pipeBroken.Store(true)
	fmt.Fprintln(os.Stderr, "note: keepassxc-cli could not read the staged entries from a pipe; using a temporary file instead (see docs/threat-model.md)")
	return nil
}

// stageXMLFile skriver XML'en til en almindelig fil og kører run. Filen har
// kun ejerens rettigheder (mappen fra os.MkdirTemp) og overskrives med nuller
// før den slettes. Bruges på platforme uden navngivne pipes, og som
// reserveløsning når en pipe ikke virker.
func stageXMLFile(xmlPath string, xml []byte, run func() error) error {
	if err := os.WriteFile(xmlPath, xml, 0o600); err != nil {
		return fmt.Errorf("write staging xml: %w", err)
	}
	defer scrubAndRemove(xmlPath)
	return run()
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
