// SPDX-License-Identifier: GPL-3.0-or-later

// Package passwd centraliserer indlæsning af masterpasswords. Interaktiv
// indtastning bruger x/term til skjult input; --password-stdin-flow læser
// præcis én linje fra stdin og er beregnet til scripting.
//
// Designprincipper:
//   - Aldrig log eller print passwords
//   - Returnér []byte (ikke string) så caller kan zero'e bagefter
//   - Empty-input afvises eksplicit — fanger pipe-fejl
package passwd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Read returnerer brugerens masterpassword. Hvis fromStdin er true læses én
// linje fra stdin (uden prompt — for scripting). Ellers vises en skjult prompt
// hvis stdin er en terminal, eller én linje læses fra stdin hvis ikke.
func Read(prompt string, fromStdin bool) ([]byte, error) {
	if fromStdin {
		return readLine(os.Stdin)
	}

	// Hvis vi er attached til en terminal, skjul tasterne mens brugeren skriver.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, prompt)
		pw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr) // newline efter skjult input
		if err != nil {
			return nil, fmt.Errorf("read password: %w", err)
		}
		if len(pw) == 0 {
			return nil, errors.New("empty password")
		}
		return pw, nil
	}

	// Ikke en TTY (piped stdin uden --password-stdin) — læs en linje med advarsel.
	fmt.Fprintln(os.Stderr, "stdin is not a terminal; reading password from stdin (use --password-stdin to silence this notice)")
	return readLine(os.Stdin)
}

// readLine læser indtil første '\n', stripper CR/LF og trailing whitespace.
// Empty input afvises — det er typisk symptom på en pipe-fejl.
func readLine(r io.Reader) ([]byte, error) {
	br := bufio.NewReader(r)
	line, err := br.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	line = bytes.TrimRight(line, "\r\n")
	if s := strings.TrimRight(string(line), " \t"); s != "" {
		return []byte(s), nil
	}
	return nil, errors.New("empty password on stdin")
}

// Zero overskriver byte-slicen med 0'er. Best-effort scrubbing.
func Zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
