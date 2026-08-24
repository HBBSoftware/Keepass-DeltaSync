# AMO submission texts

The two free-text fields AMO asks for on every upload, kept here so the next
submission does not start from a blank page. Both are **English** — the listing
has no other locale. Update the *Release Notes* per version; the *Notes to
Reviewer* only need the "What changed" paragraph rewritten.

Timings so far: 0.1.1 was uploaded 2026-08-20 and reviewed three days later;
0.2.0 was uploaded 2026-08-24 and went public **79 seconds** after upload. A
listed update is signed and published as soon as the automated validation
passes — the human review follows afterwards, and can still reject.

Reminders that are not fields:

- Bump `manifest.json` first. AMO burns a version number permanently, even if
  the version is deleted.
- "Do You Need to Submit Source Code?" → **No.** No build step, no
  minification: `package.sh` zips the seven listed files verbatim.
- **Run `extension/package.sh` immediately before uploading, and upload the
  file it just wrote.** `dist/` is git-ignored scratch: it can hold a build
  from an older working tree, and it did. What went out as 0.2.0 was built
  before the setup buttons were pointed at `firefox.html`, so the published
  add-on opens the repository's markdown page instead — see 0.2.1 below.
  Verify before uploading: `grep SETUP_URL` on the popup.js inside the .xpi.

---

## Release Notes — 0.2.1

```
0.2.0 shipped a package built before the setup buttons were given their final
address. Both of them opened the project's repository page, and neither jumped
to the part that answers the problem you were actually looking at.

They now open the setup guide itself, at the section for the dead end you hit:
one for a host Firefox cannot reach, one for a database that has not been
registered yet.

Nothing else differs from 0.2.0.
```

## Notes to Reviewer — 0.2.1

```
WHAT CHANGED SINCE 0.2.0

One constant in popup.js. 0.2.0 was packaged from an older working tree, so
its two setup buttons opened
https://gitlab.com/Star95/keepass-deltasync/-/blob/main/docs/install-browser.md
— the same instructions, but as a repository file and with no anchor for the
specific dead end. They now open
https://deltasync.bjoerck-braun.dk/firefox.html at "#host" or "#standalone".

Both URLs are hard-coded constants; nothing decides them at runtime, and no
remote page is loaded into the extension. popup.js is the only file that
differs from 0.2.0.

TESTING IT WITHOUT INSTALLING ANYTHING

The buttons appear exactly when no native host is present, so this needs no
external software: install the add-on, open the popup (Alt+Shift+K), and it
says "Firefox cannot reach the keepass-deltasync host" with a "How to set it
up" button. Click it — it opens the guide at #host in a new tab. That is the
whole change.

WHAT THE ADD-ON DOES

It cannot read a KeePass database itself. It talks over native messaging to
dk.hbb.keepass_deltasync, a subcommand of the separately installed
keepass-deltasync binary (GPL-3.0, same repository), which opens the user's
local .kdbx through keepassxc-cli and returns only uuid, title, URLs and group
path — an explicit allow-list in the host, so nothing else can be sent. No
passwords, usernames, notes or attachments reach the browser, and the
masterpassword is read from the OS keyring by the host rather than passing
through Firefox.

To test it end to end, with no account and no server:

  keepassxc-cli db-create -p test.kdbx
  keepassxc-cli add -u alice --url https://example.org -g -L 16 -l -U -n       test.kdbx "Example site"
  keepass-deltasync add-local test ./test.kdbx --save-password
  keepass-deltasync install-browser-host     (then restart Firefox)

Then open the popup and type "exa": Enter opens the entry's site.
"keepass-deltasync browser-host --probe test" prints exactly the JSON the
extension would receive, without Firefox in the picture.

PERMISSIONS

nativeMessaging reaches the host. storage is used solely as
browser.storage.session for the index, dropped when Firefox closes or the user
presses Lock — nothing on disk. No host permissions, no content scripts, no
remote code. Sources, unminified and with no build step:
https://gitlab.com/Star95/keepass-deltasync
```

---

## Submitted earlier — kept as the template

## Release Notes — 0.2.0

```
When the popup cannot do anything useful, it now says why and points somewhere.

Two dead ends used to print a raw command line and leave it at that. If Firefox
cannot reach the keepass-deltasync host, the popup now explains that the host is
installed once, outside the browser, and offers a button to the setup guide. If
the host is running but no database has been registered yet, that is a different
problem, and it gets its own button.

The guide lives on the web rather than inside the add-on on purpose: the parts
that go stale fastest are the paths each Firefox packaging looks in, and those
can now be corrected without shipping a new version.

Nothing else changed. No new permissions, still no host permissions, and still
nothing but titles, URLs and group paths ever reaches the browser.
```

---

## Notes to Reviewer — 0.2.0

```
WHAT CHANGED SINCE 0.1.1

Only the popup's two dead-end states. Both used to print a raw CLI command;
they now show one sentence of explanation and a button that opens the setup
guide in a new tab (browser.tabs.create). The two target URLs are hard-coded
constants in popup.js — SETUP_URL + "#host" and + "#standalone". Nothing
decides them at runtime and no remote page is loaded into the extension.

No new files, no new permissions. The diff is ~90 lines across popup.js,
popup.html, popup.css and the version field in manifest.json.

TESTING THE CHANGE WITHOUT INSTALLING ANYTHING

The changed states are exactly the states you get when no native host is
present, so the new behaviour needs no external software at all:

  1. Install the add-on and open the popup — toolbar button, or Alt+Shift+K.
  2. Since the native messaging host is not registered, the popup says "Firefox
     cannot reach the keepass-deltasync host" and shows a "How to set it up"
     button.
  3. Click it. It opens https://deltasync.bjoerck-braun.dk/firefox.html#host in
     a new tab. That is the entire feature.

The second dead end (host running, no database registered) needs the host; it
is the last step of the full test below.

FULL TEST — NO ACCOUNT, NO SERVER, NO NETWORK (~5 MINUTES)

The add-on cannot read a .kdbx itself. It talks over native messaging to
dk.hbb.keepass_deltasync, which is a subcommand of the separately installed
keepass-deltasync binary (GPL-3.0, same repository as the sources). Searching
needs no account and no server — that is what the add-local command below is
for.

Prerequisites: KeePassXC installed (the host shells out to its keepassxc-cli),
and the keepass-deltasync binary from
https://gitlab.com/Star95/keepass-deltasync/-/releases (tarball for Linux and
macOS, zip or installer for Windows). Put it somewhere permanent before step 3:
the registration writes down the path it was run from. On macOS, clear the
download quarantine once: xattr -d com.apple.quarantine keepass-deltasync

  1. A throwaway database, if you do not have one to spare:

       keepassxc-cli db-create -p test.kdbx
       keepassxc-cli add -u alice --url https://example.org -g -L 16 -l -U -n \
           test.kdbx "Example site"

  2. Register it for search only. Nothing is uploaded anywhere; add-local
     writes a local config entry and nothing else:

       keepass-deltasync add-local test ./test.kdbx --save-password

     --save-password puts the masterpassword in the OS keyring. Leave it out
     and the popup asks for it instead, and forgets it again after 15 idle
     minutes.

  3. Register the native messaging host, then restart Firefox:

       keepass-deltasync install-browser-host

     It prints every manifest it writes; --dry-run shows them without writing.
     On Windows it touches only HKCU and %LOCALAPPDATA% — no administrator
     rights. On Linux it covers a packaged, a snap and a flatpak Firefox; a
     flatpak Firefox additionally needs

       flatpak override --user --talk-name=org.freedesktop.Flatpak org.mozilla.firefox

  4. Open the popup (Alt+Shift+K) and type "exa". "Example site" appears.
     Enter opens https://example.org in the current tab, Ctrl+Enter in a new
     one. Typing "kp exa" in the address bar does the same thing.

  5. For the other changed state: run "keepass-deltasync forget test" and
     reopen the popup. The host answers, but no database is registered, so you
     get the "How to add a database" button. It opens the #standalone anchor of
     the same guide.

VERIFY THE DATA BOUNDARY WITHOUT TAKING OUR WORD FOR IT

  keepass-deltasync browser-host --probe test

prints on stdout exactly the JSON the extension would receive: uuid, title,
URLs and group path. The host builds that from an explicit allow-list, so no
other field can be sent even by mistake. Passwords, usernames, notes and
attachments never leave the host, and the masterpassword is read from the OS
keyring by the host rather than passing through Firefox.

PERMISSIONS

  nativeMessaging — the only way to reach the host described above.
  storage        — used solely as browser.storage.session for the search
                   index. It is dropped when Firefox closes and when the user
                   presses Lock in the popup; nothing is written to disk.

No host permissions, no content scripts, no remote code, no analytics, and
data_collection_permissions is ["none"]. The only navigation the add-on
performs is tabs.update/tabs.create to an http(s) URL taken from the user's own
database, or to the fixed setup-guide URL above. Favicons are deliberately not
fetched: each fetch would tell a site that the user holds an account there.

SOURCES

  https://gitlab.com/Star95/keepass-deltasync — extension/ is this add-on,
  docs/browser-extension.md documents the native messaging protocol and its
  security boundaries.

The .xpi contains the seven source files byte-for-byte; package.sh only zips
them, with fixed timestamps, so the archive is reproducible from the tag.
```

---

## Notes to Reviewer — short version

The full text above is ~5000 characters. If the field refuses it, this one is
~2200 and keeps the two things a reviewer actually needs: what changed, and how
to see it without installing anything.

```
WHAT CHANGED SINCE 0.1.1

Only the popup's two dead-end states. Both used to print a raw CLI command;
they now show one sentence of explanation and a button that opens the setup
guide in a new tab (browser.tabs.create). The two target URLs are hard-coded
constants in popup.js — SETUP_URL + "#host" and + "#standalone". Nothing
decides them at runtime, and no remote page is loaded into the extension. No
new files, no new permissions; ~90 lines across popup.js, popup.html,
popup.css and the version field.

TESTING IT WITHOUT INSTALLING ANYTHING

The changed states are exactly the states you get with no native host present:
install the add-on, open the popup (Alt+Shift+K), and it says "Firefox cannot
reach the keepass-deltasync host" with a "How to set it up" button that opens
https://deltasync.bjoerck-braun.dk/firefox.html#host in a new tab. That is the
whole feature.

WHAT THE ADD-ON DOES

It cannot read a KeePass database itself. It talks over native messaging to
dk.hbb.keepass_deltasync, a subcommand of the separately installed
keepass-deltasync binary (GPL-3.0, same repository), which opens the user's
local .kdbx through keepassxc-cli and returns only uuid, title, URLs and group
path — an explicit allow-list in the host, so nothing else can be sent. No
passwords, usernames, notes or attachments reach the browser.

To test it end to end, with no account and no server:

  keepassxc-cli db-create -p test.kdbx
  keepassxc-cli add -u alice --url https://example.org -g -L 16 -l -U -n       test.kdbx "Example site"
  keepass-deltasync add-local test ./test.kdbx --save-password
  keepass-deltasync install-browser-host     (then restart Firefox)

Then open the popup and type "exa": Enter opens the entry's site.
"keepass-deltasync browser-host --probe test" prints exactly the JSON the
extension would receive, without Firefox in the picture.

PERMISSIONS

nativeMessaging reaches the host. storage is used solely as
browser.storage.session for the index, dropped when Firefox closes or the user
presses Lock — nothing on disk. No host permissions, no content scripts, no
remote code. Sources, unminified and with no build step:
https://gitlab.com/Star95/keepass-deltasync
```
