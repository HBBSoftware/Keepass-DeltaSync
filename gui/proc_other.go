// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package main

import "os/exec"

// hideConsole er en no-op uden for Windows — der findes ikke et konsolvindue at
// skjule, og CLI'en arver bare GUI-processens standard-streams.
func hideConsole(cmd *exec.Cmd) {}
