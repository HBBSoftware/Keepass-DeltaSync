// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !unix

package kdbx

// stageXML skriver XML'en til en almindelig fil, fordi Windows ikke har
// navngivne pipes i filsystemet, og keepassxc-cli kun læser fra en sti.
// Klarteksten er dermed på disken i det korte øjeblik importen varer — se
// stageXMLFile og docs/threat-model.md.
func stageXML(xmlPath string, xml []byte, run func() error) error {
	return stageXMLFile(xmlPath, xml, run)
}
