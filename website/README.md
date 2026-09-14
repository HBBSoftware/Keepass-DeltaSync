# deltasync-website

Website for [DeltaSync](https://gitlab.com/Star95/keepass-deltasync) — privacy-first KeePass sync.

Live at <https://deltasync.org/>.

## Contents

**English (primary, served at root):**
- `index.html` — landing page with key features and quick start
- `architecture.html` — principle diagrams (crypto stack, sync flow, multi-user sharing)
- `getting-started.html` — user and admin guide
- `deploy-server.html` — beginner-friendly, step-by-step server deployment guide (upload, setup.php, first user)
- `faq.html` — questions that come up in practice (devices, bindings, the stored master password, sharing, the desktop app, the Firefox extension)
- `firefox.html` — install page for the Firefox extension, written to be the add-on listing's homepage: it covers both the DeltaSync-server route and the standalone one, and assumes nothing about the reader having a server
- `chrome.html` — the same for Edge and Chrome. The popup's setup buttons link here with `#host` and `#standalone`, so those two anchors must stay
- `privacy.html` — privacy policy for the browser extension, linked from the store listings

**Dansk (under `/da/`):**
- `da/index.html` · `da/architecture.html` · `da/getting-started.html` · `da/deploy-server.html` · `da/faq.html` · `da/firefox.html` · `da/chrome.html` · `da/privacy.html` — danske oversættelser

**Shared:**
- `style.css` — design tokens + base styles (light + dark via `prefers-color-scheme`)
- `assets/` — logo (SVG master) + PNG favicons in various sizes

## Deployment

`python3 website/deploy.py` uploader over SFTP, og CI kalder det samme script
når `website/**` ændrer sig på `main` (jobbet `publish:website`). Det kræver
`DEPLOY_SFTP_USER` og `DEPLOY_SFTP_PASSWORD` som maskerede CI-variabler; vært,
sti og værtsnøgle har indbyggede standarder i scriptet.

Vær opmærksom på **hvor filerne lander**. Siden deles med DeltaSync-serverens
PHP-app: `index.html` og `style.css` ligger i samme mappe som `admin.html`,
`index.php` og `.htaccess`, og de tre sidste kommer fra `server/public/`, ikke
herfra. Derfor spejler `deploy.py` ikke og sletter aldrig — den skriver kun de
filer den får. En spejling ville fjerne API'et.

`.htaccess` hører altså til `server/public/`. Den router til `index.php` når
der ikke findes en konkret fil, og den sætter cache-headere: `no-cache` på
HTML, CSS og JS så en rettelse slår igennem med det samme, en uge på billeder
og skrifttyper.

## Lokal preview

Med Python 3:

```sh
python3 -m http.server 8080
# Åbn http://localhost:8080/ i browser
```

Eller med Go:

```sh
go run -mod=mod -exec gowebserver . 8080
```

Eller `npx serve .` med Node.js.

## Logo + branding

Logo'et i `assets/logo.svg` er master-versionen. PNG-favicons og app-ikoner er afledt fra samme design. Branden er copyright Hans Bjørck-Braun og ikke dækket af CC-licensen — kontakt for genbrug.

## Licens

CC-BY-SA-4.0 for HTML, CSS, SVG-diagrammer og prosaen. Se [LICENSE](LICENSE).
