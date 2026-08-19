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

1. **Register the host** (once per machine, and again after moving the binary):

   ```
   keepass-deltasync install-browser-host
   ```

   Add `--dry-run` first if you want to see exactly what it writes.

2. **Load the extension.** Until it is signed, use a temporary install:
   open `about:debugging#/runtime/this-firefox` → *Load Temporary Add-on…* →
   pick `extension/manifest.json`. Temporary add-ons disappear when Firefox
   restarts.

3. **Restart Firefox** so it picks up the native messaging manifest.

## Use

- **Address bar**: type `kp`, a space, then your search. Pick a suggestion and
  press Enter.
- **Toolbar button** or **Alt+Shift+K**: opens the popup. Arrow keys move,
  Enter opens in the current tab, Ctrl/Cmd+Enter and middle-click open a new
  tab.
- **Lock** in the popup drops the cached index and tells the host to wipe any
  password it holds.

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
restart Firefox.

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

The extension is not signed yet. The extension ID `keepass-deltasync@hb-b.dk`
is baked into both `manifest.json` and the native messaging manifest's
`allowed_extensions`, so it has to stay stable — see the open questions in the
design document.
