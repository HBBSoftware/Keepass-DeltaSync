# Setting up KeePass search in Firefox

The in-repo copy of the setup guide. The extension's setup buttons point at
[deltasync.bjoerck-braun.dk/firefox.html](https://deltasync.bjoerck-braun.dk/firefox.html)
(anchors `#standalone` and `#host`), which carries the same content — the text
lives on the web so it can be corrected without shipping a new add-on. Keep the
two in step.

It covers the whole path from nothing to a working search box.

Two things decide how much you have to do:

- **You only want to search** your own local `.kdbx` and open the right site —
  no accounts, no server, nothing to host. That is this page, and it is three
  steps.
- **You also want to sync** the database between your machines and your phone.
  That is the Delta-Sync server, and it starts at
  [deltasync.bjoerck-braun.dk](https://deltasync.bjoerck-braun.dk/). Come back
  here afterwards for the Firefox part — everything below still applies, except
  that you will have registered your database with `init` instead of
  `add-local`.

The extension never sees your passwords either way. It receives titles, URLs
and group paths, and nothing else; filling in credentials stays
[KeePassXC-Browser](https://github.com/keepassxreboot/keepassxc-browser)'s job.

---

## What you need

| | Why |
|---|---|
| **KeePassXC** | The host opens your database with `keepassxc-cli`, which ships with it. [keepassxc.org](https://keepassxc.org/) |
| **The `keepass-deltasync` binary** | The extension cannot read a `.kdbx` itself. This program does, and hands Firefox a title-and-URL index. |
| **Firefox 142 or newer** | |

---

## 1. Get the binary

Download it from the
[releases page](https://gitlab.com/Star95/keepass-deltasync/-/releases) and
**put it somewhere permanent before step 2** — the registration writes down the
path you install from, so a binary that later moves out of `~/Downloads` stops
working.

- **Windows** — the
  [installer](https://gitlab.com/Star95/keepass-deltasync/-/releases) puts both
  the GUI and the command-line program in `Program Files`. If you only want the
  command-line program, take the `.zip` instead and unpack it somewhere stable.
- **Linux** — unpack the `.tar.gz` and move the binary to `~/bin` or
  `/usr/local/bin`. Avoid `~/.local/bin` if your Firefox is a snap: a confined
  Firefox cannot reach dot-directories in your home.
- **macOS** — unpack the `.tar.gz` and move the binary to `/usr/local/bin`.
  macOS quarantines anything downloaded with a browser, and Firefox launches
  the host without a dialog you could click, so clear the flag once:

  ```
  xattr -d com.apple.quarantine /usr/local/bin/keepass-deltasync
  ```

Only `linux/amd64`, `darwin/amd64`, `darwin/arm64` and `windows/amd64` are
built. On anything else — ARM Linux, for instance — build from source; it is a
plain `go build`.

## Prefer a menu?

Steps 2 and 3 both exist in the built-in menu, so you do not have to remember
any of it:

```
keepass-deltasync tui
```

Pick **Firefox search**. It is there whether or not you have an account. The
rest of this page spells the same steps out as commands.

## 2. Point the program at your database

```
keepass-deltasync add-local mydb ~/Documents/passwords.kdbx
```

`mydb` is just a label; it shows up in the popup when more than one database is
unlocked. Nothing is uploaded anywhere — `add-local` writes a local note that
this file exists, and that is all it does.

Add `--save-password` if you want the masterpassword stored in your OS keyring.
Without it the popup asks for it, and forgets it again after 15 idle minutes.
The password is checked before it is stored, so a typo fails here rather than
the next time you search.

If you *are* syncing with a server, you will have run
`keepass-deltasync init` instead. Do not run both for the same file.

## 3. Register the host with Firefox

```
keepass-deltasync install-browser-host
```

Then **restart Firefox**. That is what makes it pick up the registration.

The command prints every Firefox it registered with. Add `--dry-run` first if
you want to see exactly what it would write, without writing it.

On Windows this touches only `HKCU` and `%LOCALAPPDATA%` — no administrator
rights. If you installed via the installer, `keepass-deltasync` is not on your
`PATH`; run it from its own directory, or use the full path in quotes:

```
"C:\Program Files\KeePass Delta-Sync\keepass-deltasync.exe" install-browser-host
```

### Linux: there is more than one Firefox

Which Firefox you have decides where the registration goes, and the command
handles all three by itself — but they are not equally well off:

- **From your distribution's package or Mozilla's tarball** — works as
  described above.
- **Snap** (the default on Ubuntu 22.04 and newer) — registered automatically.
  Snap-confined Firefox can only reach files outside dot-directories in your
  home, so keep the binary in a plain directory like `~/bin`.
- **Flatpak** — registered automatically, but the sandbox has to be allowed to
  reach the host program. Run this once:

  ```
  flatpak override --user --talk-name=org.freedesktop.Flatpak org.mozilla.firefox
  ```

Not sure which one you have? `snap list firefox` and `flatpak list | grep -i
firefox` answer it. Installing a different Firefox later means running
`install-browser-host` again.

## 4. Install the extension

Install **DeltaSync — KeePass search & go** from addons.mozilla.org:

[addons.mozilla.org/firefox/addon/deltasync-keepass-search-go](https://addons.mozilla.org/firefox/addon/deltasync-keepass-search-go/)

It asks for two permissions — native messaging, to reach the host from step 3,
and storage. It requests no host permissions and runs no content scripts, so it
cannot see any page you visit.

To run a build from this repository instead, load it temporarily: open
`about:debugging#/runtime/this-firefox` → *Load Temporary Add-on…* → pick
`extension/manifest.json`. The ID is fixed, so a copy installed from
addons.mozilla.org has to be removed first, and temporary add-ons disappear
when Firefox restarts.

---

## Using it

- **Address bar** — type `kp`, a space, then your search. Pick a suggestion and
  press Enter.
- **Toolbar button** or **Alt+Shift+K** — opens the popup. Arrow keys move,
  Enter opens in the current tab, Ctrl/Cmd+Enter and middle-click open a new
  tab.
- **Lock** in the popup drops the index and tells the host to forget any
  password it is holding.

Search matches on host name, title and group, and host name ranks highest —
that is usually what you remember.

---

## When it does not work

**"cannot start the native host"** — Firefox cannot find or launch the host.
In order of likelihood:

1. You did not restart Firefox after step 3.
2. The binary moved since you registered it. Run `install-browser-host` again.
3. You have a Firefox variant that was not installed at the time — a snap or
   flatpak installed after the fact. Run `install-browser-host` again; it
   prints what it registered with, and that list is the answer to whether it
   found yours.
4. Flatpak, without the `flatpak override` from step 3.
5. macOS, with the quarantine flag still set.

**"No databases are registered"** — the host is running and answering, so
steps 1 and 3 are fine. Step 2 has not happened yet: run `add-local`.

**`keepassxc-cli not found`** — KeePassXC is not installed, or it is installed
in a way that does not put `keepassxc-cli` on your `PATH` (a flatpak or snap
KeePassXC does not). Point at it explicitly:

```
keepass-deltasync add-local mydb ~/passwords.kdbx --keepassxc-cli /path/to/keepassxc-cli
```

**Nothing changes after the database is edited** — the host watches the file
and re-indexes by itself. If the masterpassword is not in your keyring and the
idle lock has already dropped it, the refresh is skipped and the old index
stays. Unlock again from the popup, or use `--save-password`.

**Check what the extension would see, without involving Firefox:**

```
keepass-deltasync browser-host --probe mydb
```

That prints the index as plain JSON: exactly the titles, URLs and group paths
that would reach the extension, and nothing more.

---

## What is not indexed

- Entries in the recycle bin.
- Entries in groups where searching is disabled in KeePassXC, including
  subgroups that inherit the setting.
- Values that cannot be navigated to — `{REF:...}` placeholders, `cmd://`
  targets, and anything that is not `http`/`https`. Those entries still appear
  in results, just without a link.

Additional URLs stored by KeePassXC as `KP2A_URL_*` custom fields *are*
indexed, and an entry is searchable on every one of them. Protected custom
fields are skipped even when they look like a URL.

---

## Removing it again

```
keepass-deltasync uninstall-browser-host   # unregister from every Firefox variant
keepass-deltasync forget mydb              # drop the database registration
```

Neither touches your `.kdbx`.
