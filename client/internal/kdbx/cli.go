// SPDX-License-Identifier: GPL-3.0-or-later

// Package kdbx wraps the external keepassxc-cli binary. We don't parse
// .kdbx files directly — instead we shell out to keepassxc-cli (which is
// part of any KeePassXC desktop install) for export and later merge.
//
// Binary location strategy (first hit wins):
//
//  1. Path passed to NewCLI
//  2. Env var KEEPASSXC_CLI
//  3. `keepassxc-cli` in PATH
//  4. Windows: C:\Program Files\KeePassXC\keepassxc-cli.exe
//  5. Windows: C:\Program Files (x86)\KeePassXC\keepassxc-cli.exe
package kdbx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// CLI wraps a discovered keepassxc-cli binary.
type CLI struct {
	binary string
}

// NewCLI locates the keepassxc-cli binary. If override is non-empty it is
// used verbatim (after stat-check). Otherwise we consult env, PATH, and
// well-known Windows install locations.
func NewCLI(override string) (*CLI, error) {
	if override != "" {
		if _, err := os.Stat(override); err != nil {
			return nil, fmt.Errorf("--keepassxc-cli path: %w", err)
		}
		return &CLI{binary: override}, nil
	}

	if env := strings.TrimSpace(os.Getenv("KEEPASSXC_CLI")); env != "" {
		if _, err := os.Stat(env); err != nil {
			return nil, fmt.Errorf("$KEEPASSXC_CLI: %w", err)
		}
		return &CLI{binary: env}, nil
	}

	if p, err := exec.LookPath("keepassxc-cli"); err == nil {
		return &CLI{binary: p}, nil
	}

	if runtime.GOOS == "windows" {
		for _, p := range []string{
			`C:\Program Files\KeePassXC\keepassxc-cli.exe`,
			`C:\Program Files (x86)\KeePassXC\keepassxc-cli.exe`,
		} {
			if _, err := os.Stat(p); err == nil {
				return &CLI{binary: p}, nil
			}
		}
	}

	return nil, errors.New("keepassxc-cli not found. Install KeePassXC (https://keepassxc.org/), or set $KEEPASSXC_CLI / --keepassxc-cli to the binary path")
}

// Binary returns the resolved keepassxc-cli path. Useful for error messages.
func (c *CLI) Binary() string { return c.binary }

// Export runs `keepassxc-cli export -f xml -q <path>` with the password piped
// on stdin and returns the XML bytes. A wrong password surfaces as a non-zero
// exit code with the cli's error on stderr.
func (c *CLI) Export(ctx context.Context, kdbxPath string, password []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.binary, "export", "-f", "xml", "-q", kdbxPath)

	// keepassxc-cli reads the password as a single line from stdin in -q mode.
	stdin := bytes.NewBuffer(append(password, '\n'))
	cmd.Stdin = stdin

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Strip likely-trailing newline so the error reads cleanly.
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("keepassxc-cli export failed: %s", msg)
	}

	xml := stdout.Bytes()
	if len(xml) == 0 {
		return nil, errors.New("keepassxc-cli export returned empty output")
	}
	return xml, nil
}
