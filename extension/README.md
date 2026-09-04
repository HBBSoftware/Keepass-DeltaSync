# keepass-deltasync — Firefox extension

Search your KeePass entries from Firefox and open the entry's website.

**It never sees your passwords.** The extension only ever receives an entry's
title, URLs and group path — filling in credentials remains
[KeePassXC-Browser](https://github.com/keepassxreboot/keepassxc-browser)'s job.
This one just gets you to the right page so that it can do its work.

The design and its security boundaries are documented in
[`docs/browser-extension.md`](../docs/browser-extension.md).

## How it fits together

```
Firefox extension  ──native messaging──>  keepass-deltasync browser-host
                                                    │
                                             keepassxc-cli export
                                                    │
                                                <db>.kdbx
```

The extension cannot read your `.kdbx` itself. It asks the `browser-host`
subcommand of the regular `keepass-deltasync` binary, which fetches the
masterpassword from your OS keyring, builds a title-and-URL index, and wipes
the key material again. The password never passes through Firefox — unless the
database has no keyring entry, in which case the popup asks for it and the host
holds it in memory until the idle lock expires.

## Install

The full walkthrough — including the Linux and macOS pitfalls — is
[`docs/install-browser.md`](../docs/install-browser.md), mirrored at
[deltasync.bjoerck-braun.dk/firefox.html](https://deltasync.bjoerck-braun.dk/firefox.html),
which is where the popup's setup buttons point. The short version:

1. **Register a database.** A server is not required: `add-local` registers a
   `.kdbx` for search only, and nothing about it is uploaded anywhere.

   ```
   keepass-deltasync add-local mydb ~/Documents/passwords.kdbx
   ```

   Add `--save-password` to keep the masterpassword in the OS keyring, so the
   popup does not ask for it. If you sync the database with a server, you will
   have run `init` instead — that works the same for search.

2. **Register the host** (once per machine, and again after moving the binary):

   ```
   keepass-deltasync install-browser-host
   ```

   Add `--dry-run` first if you want to see exactly what it writes. On Linux
   this covers a packaged, a snap and a flatpak Firefox, and the command prints
   which ones it found. A flatpak Firefox additionally needs
   `flatpak override --user --talk-name=org.freedesktop.Flatpak org.mozilla.firefox`.

3. **Install the extension** from
   [addons.mozilla.org](https://addons.mozilla.org/firefox/addon/deltasync-keepass-search-go/).
   To run *this* copy instead, load it temporarily: open
   `about:debugging#/runtime/this-firefox` → *Load Temporary Add-on…* → pick
   `extension/manifest.json`. The ID is fixed, so remove an installed copy
   first; temporary add-ons disappear when Firefox restarts.

4. **Restart Firefox** so it picks up the native messaging manifest.

## Use

- **Address bar**: type `kp`, a space, then your search. Pick a suggestion and
  press Enter.
- **Toolbar button** or **Alt+Shift+K**: opens the popup. Arrow keys move,
  Enter opens in the current tab, Ctrl/Cmd+Enter and middle-click open a new
  tab.
- **Lock** in the popup drops the cached index and tells the host to wipe any
  password it holds. It also stops the extension from unlocking on its own
  again until you say so — a Lock that the next popup undid would be no lock
  at all — so the next popup shows the *Unlock* button. Closing Firefox has
  the same effect: nothing about the index survives it.

The popup unlocks by itself. The masterpassword comes from the OS keyring, so
nothing is asked of you, and the search field is ready as soon as the index is
built — a second or two, since opening the database runs Argon2. The *Unlock*
button only appears when a click actually decides something: after a **Lock**,
or when the attempt failed. In that last case the popup says why, and for a
database with no keyring entry it goes straight to the password field.

The address bar warms the index the same way: search with `kp` while nothing is
unlocked and the first search comes up empty, but the next keystroke has the
entries.

Search matches on host name, title and group. Hostname matches rank highest,
since that is usually what you remember.

**Entries with several URLs** — KeePassXC' *Additional URLs*, stored as
`KP2A_URL_*` custom attributes — are searchable on every one of them. Such an
entry takes a single row carrying a small `2 URLs` badge; select it and every
address unfolds beneath, best match first. Arrow down to pick another one.

Which address counts as the best depends on the search. If you matched mainly
on the title, the primary address wins — the search pointed at the entry as a
whole rather than at one of its addresses. If a specific address matched harder
than the title, that is the one you were looking for.

The address bar cannot unfold, so there each extra address gets its own
suggestion, listed after the main ones.

## What is not indexed

- Entries in the recycle bin.
- Entries in groups where you disabled searching in KeePassXC — including
  subgroups that inherit the setting.
- Values that cannot be navigated to: `{REF:...}` placeholders, `cmd://`
  targets, and anything that is not `http`/`https`. Those entries still show up
  in results, just without a link.

Additional URLs stored as `KP2A_URL_*` custom fields are indexed alongside the
main URL. Protected custom fields are skipped even when they look like a URL.

## Troubleshooting

**"cannot start the native host"** — the manifest is missing or points at a
binary that has moved. Re-run `keepass-deltasync install-browser-host` and
restart Firefox. Note what it prints: the list of Firefox variants it
registered with is the answer to whether it found *your* Firefox. A snap or
flatpak installed after the fact needs the command run again.

**Nothing happens after a sync** — the host watches the `.kdbx` and re-indexes
automatically. If the database has no keyring entry and the idle lock has
already wiped the password, the refresh is skipped and the old index stays;
unlock again from the popup.

**Check the index without involving Firefox:**

```
keepass-deltasync browser-host --probe <database>
```

This prints exactly what the extension would receive, as plain JSON.

## Distribution status

Signed and listed on addons.mozilla.org as
[DeltaSync — KeePass search & go](https://addons.mozilla.org/firefox/addon/deltasync-keepass-search-go/).
0.3.0 — the popup that unlocks by itself — is packaged and waiting to be
uploaded; 0.2.1 is the listed version. 0.2.0 went public
on 2026-08-24, 79 seconds after upload: a listed update is
signed and published as soon as automated validation passes, and the human
review happens afterwards. The first listing is the slow one — 0.1.1's review
took three days. Check `manifest.json` against the listed version anyway before
assuming a user has a given fix.

The extension ID `keepass-deltasync@hb-b.dk` is baked into both
`manifest.json` and the native messaging manifest's `allowed_extensions`, so it
has to stay stable — and now that AMO holds it, it cannot be changed at all.
