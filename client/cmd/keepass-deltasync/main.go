// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
)

const usage = `keepass-deltasync — sync KeePass databases via a Delta-Sync server.

Usage:
  keepass-deltasync <command> [arguments]

Commands:
  enroll <enrollment-token>   Register this device with the server
  status                      Show current enrollment + last-seen info
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "enroll":
		exitOnError(runEnroll(os.Args[2:]))
	case "status":
		exitOnError(runStatus(os.Args[2:]))
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}

func exitOnError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "error: %s\n", err)
	os.Exit(1)
}
