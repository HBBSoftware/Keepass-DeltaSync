# keepass-deltasync — Chrome and Edge extension

Search your KeePass entries from Chrome or Edge and open the entry's website.

This is the Chromium port of the Firefox extension in [`../extension`](../extension).
It is the same extension, in the same two ways it matters: it never sees a
secret beyond the entry index, and it never talks to a server. Filling in
credentials stays with KeePassXC-Browser.

## What is in here, and what is not

The popup, the search and the background logic are **not** copied. They are
read out of `../extension` when the package is built, so the two browsers
cannot drift apart. This directory holds only what Chromium needs differently:

| File | Why it exists |
|------|---------------|
| `manifest.json` | Chromium wants a service worker and raster icons; Firefox wants neither |
| `compat.js` | Chromium has no `browser` namespace, and answers a message differently |
| `sw.js` | The service worker entry point; Firefox lists its background scripts in the manifest instead |
| `icons/*.png` | Chrome and Edge reject an SVG icon |
| `make-icons.py` | Redraws those PNGs from `../extension/icon.svg` |
| `package.sh` | Assembles the package and substitutes the few strings that say "Firefox" |
| `dev-key.pub` | Fixes the extension's ID while testing, so it is the same on every machine |
| `smoke-test.mjs` | Runs the shared background code against a fake `chrome` |

The one difference worth knowing about is in `compat.js`: Firefox lets a
message listener answer by returning a promise, and Chromium does not. Without
that bridge every popup would open empty.

## Build it

```sh
./package.sh --dev      # -> build/unpacked/, with a fixed extension ID
./package.sh            # -> ../dist/keepass-deltasync-chromium-<version>.zip
node smoke-test.mjs     # optional, needs node; catches a missing shim call
```

`package.sh` always leaves an unpacked tree in `build/unpacked/`. That is the
folder to point *Load unpacked* at. Use `--dev` for that tree, for the reason
under [Install it](#install-it) below; it changes nothing in the zip.

The zip is byte-reproducible: two builds of the same commit give the same
file, so anyone can check that the package matches the sources.

If `package.sh` stops with "the Firefox extension changed", a string it
substitutes has been edited in `../extension`. Look at `SUBSTITUTIONS` in
`package.sh` and put the new wording there. The build fails on purpose rather
than quietly sending a Chrome user to a Firefox page.

## Install it

### Load the extension

1. Build it with `./package.sh --dev`.
2. Open `chrome://extensions` (or `edge://extensions`) and turn on
   **Developer mode**.
3. **Load unpacked**, and pick `extension-chromium/build/unpacked`.

### Set up the native host

The extension talks to the `keepass-deltasync` client through native
messaging, and the client writes the manifest that lets the browser find it.
That manifest has to name the extension, so the browser needs an ID first:

```sh
keepass-deltasync install-browser-host --extension-id iocbdcgjepgmakdgfnhlanbncnfmgeof
```

Restart the browser afterwards. It reads the manifest at startup.

That ID is not a secret and not a placeholder: it is what Chromium derives
from `dev-key.pub`, and `package.sh --dev` prints the whole command after a
build. The same ID works on every machine and in both browsers.

Without `--dev` the browser makes up an ID from the absolute path of the
folder, so it changes between machines and has to be read off
`chrome://extensions` each time. Either way there is no way to skip the ID:
Chromium's native messaging manifest identifies callers by ID, and an empty
list means nobody may call — not everybody.

`--dry-run` prints what would be written without touching anything.
`--extension-id` may be repeated, which is what you want when one machine runs
both the unpacked build and the store build.

**Why a key at all, and where is the private half?** Chromium derives an
extension's ID from a public key: the first 128 bits of its SHA-256, written
with the letters a to p. Signing is the store's job, so the private half was
never needed for this and was thrown away when `dev-key.pub` was generated.
The key is only in the unpacked tree — the store makes its own at upload, and
the published IDs then belong in `chromiumExtensionIDs` in
`client/cmd/keepass-deltasync/browser_install.go`, after which no flag is
needed for the published extension. The development ID stays out of that list
on purpose: this key is public, so anyone could build an extension carrying
the same ID, and the client should not trust it unless its owner asks for it
by hand.

### Add a local database

The extension searches a local `.kdbx` file. Register one with the client:

```sh
keepass-deltasync add-local ~/Passwords.kdbx
```

No server and no account are involved. The host reads that file and nothing
else, and the index it hands the extension holds titles, URLs and group paths
— never a password.

## Where it runs

| | Manifest goes to |
|---|---|
| Windows | a file in `%LOCALAPPDATA%\keepass-deltasync`, found through `HKCU\Software\Google\Chrome` or `...\Microsoft\Edge` |
| Linux | `~/.config/google-chrome/NativeMessagingHosts/`, `chromium` and `microsoft-edge` likewise |
| macOS | `~/Library/Application Support/Google/Chrome/NativeMessagingHosts/`, Edge and Chromium likewise |

A snap or flatpak Chromium is not covered. Those sandboxes need the same
special handling the Firefox variants get, and none of it has been tried.

## One behavioural difference from Firefox

Chromium shuts the service worker down when it has been idle, which closes the
port and ends the host process with it. The host's fifteen-minute idle unlock
goes with it.

In practice it shows up rarely: the entry index lives in session storage, so a
search still answers without the host. The cost lands on a refresh, where the
database is unlocked again from the keyring — a few seconds of key derivation,
with no prompt.

## Publishing

Neither store takes an unsigned package the way Firefox does; both sign at
upload. The zip from `package.sh` is what gets uploaded.

- **Chrome Web Store** needs a developer account with a one-off fee.
- **Edge Add-ons** is free and takes the same zip.

Tag a release as `extension-chromium/vX.Y.Z` after bumping `version` in
`manifest.json`. CI builds the zip and attaches it to a GitLab Release; it
refuses to build if the tag and the manifest disagree. See
[`../VERSIONING.md`](../VERSIONING.md).

## Also read

- [`../docs/browser-extension.md`](../docs/browser-extension.md) — the design,
  the protocol between extension and host, and what the index may contain
- [`../extension/README.md`](../extension/README.md) — the Firefox extension,
  whose troubleshooting section applies here too
