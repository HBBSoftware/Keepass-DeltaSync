// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// createNoWindow er Windows' CREATE_NO_WINDOW-flag. Når en GUI-app (uden eget
// konsolvindue) starter et konsol-program, opretter Windows ellers et nyt
// konsolvindue for barne-processen — det giver et synligt "blink" hver gang.
// Flaget undertrykker det vindue.
const createNoWindow = 0x08000000

// hideConsole sørger for at CLI-subprocessen startes uden et konsolvindue.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
