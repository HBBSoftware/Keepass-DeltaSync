// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !unix

package kdbx

import (
	"fmt"
	"os"
)

// stageXML skriver XML'en til en almindelig fil, fordi Windows ikke har
// navngivne pipes i filsystemet, og keepassxc-cli kun læser fra en sti.
// Filen har kun ejerens rettigheder (mappen fra os.MkdirTemp), og den
// overskrives med nuller, før den slettes. Klarteksten er dermed på disken i
// det korte øjeblik importen varer — se docs/threat-model.md.
func stageXML(xmlPath string, xml []byte, run func() error) error {
	if err := os.WriteFile(xmlPath, xml, 0o600); err != nil {
		return fmt.Errorf("write staging xml: %w", err)
	}
	defer scrubAndRemove(xmlPath)
	return run()
}
