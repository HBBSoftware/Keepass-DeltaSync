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
  init <name> <local.kdbx>    Register a local .kdbx for syncing
  init-shared <remote> <path> Bootstrap a local .kdbx for a database shared with you
  push <name>                 Upload all entries from a local .kdbx
  pull <name>                 Fetch server entries and merge into local .kdbx
  sync <name>                 Pull, then push entries modified since last sync
  daemon                      Foreground sync loop: fsnotify + polling for all (or --db) databases
  versions <name> <uuid>      List server-stored versions of an entry
  restore <name> <uuid> <n>   Roll an entry back to version n (1, 2 or 3)
  share <name> <username>     Share an owned database with another user (v2)
  unshare <name> <username>   Remove a member (or yourself) from a database
  shares <name>               List members of a database (owner only)
  status                      Show current enrollment + last-seen info
  devices                     List all enrolled devices for this user
  databases                   List registered databases (local + server)
  log                         Show this user's recent audit-log activity
  admin <subcommand>          Admin commands (token-sql, user-create, user-list, ...)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "enroll":
		exitOnError(runEnroll(os.Args[2:]))
	case "init":
		exitOnError(runInit(os.Args[2:]))
	case "init-shared":
		exitOnError(runInitShared(os.Args[2:]))
	case "push":
		exitOnError(runPush(os.Args[2:]))
	case "pull":
		exitOnError(runPull(os.Args[2:]))
	case "sync":
		exitOnError(runSync(os.Args[2:]))
	case "daemon":
		exitOnError(runDaemon(os.Args[2:]))
	case "versions":
		exitOnError(runVersions(os.Args[2:]))
	case "restore":
		exitOnError(runRestore(os.Args[2:]))
	case "share":
		exitOnError(runShare(os.Args[2:]))
	case "unshare":
		exitOnError(runUnshare(os.Args[2:]))
	case "shares":
		exitOnError(runShares(os.Args[2:]))
	case "status":
		exitOnError(runStatus(os.Args[2:]))
	case "devices":
		exitOnError(runDevices(os.Args[2:]))
	case "databases":
		exitOnError(runDatabases(os.Args[2:]))
	case "log":
		exitOnError(runLog(os.Args[2:]))
	case "admin":
		exitOnError(runAdmin(os.Args[2:]))
	case "sync-test":
		// Skjult diagnostik-kommando. Slettes når full sync er færdig.
		exitOnError(runSyncTest(os.Args[2:]))
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
