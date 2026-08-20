// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// cli er en tynd wrapper omkring keepass-deltasync-kommandolinjeprogrammet.
// GUI'en taler ALDRIG selv med serveren og rører hverken krypto eller config —
// præcis som den eksisterende terminal-menu (tui) gør den alt ved at kalde
// CLI'en som en subproces. Det holder de to projekter adskilt og betyder at
// kryptokoden kun findes ét sted.
type cli struct {
	path string // sti til keepass-deltasync(.exe)
}

// binaryName er filnavnet på CLI'en for det aktuelle OS.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "keepass-deltasync.exe"
	}
	return "keepass-deltasync"
}

// locateCLI leder efter CLI'en på de mest sandsynlige steder, i rækkefølge:
//  1. en eksplicit sti gemt i gui.json (brugeren har valgt den)
//  2. ved siden af selve GUI-programmet (anbefalet bundling)
//  3. i systemets PATH
//
// Returnerer tom streng hvis intet findes — så viser UI'en en "find programmet"-
// dialog.
func locateCLI(saved string) string {
	if saved != "" && fileExists(saved) {
		return saved
	}
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), binaryName())
		if fileExists(cand) {
			return cand
		}
	}
	if p, err := exec.LookPath(binaryName()); err == nil {
		return p
	}
	return ""
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// result samler udfaldet af en CLI-kørsel.
type result struct {
	Stdout string
	Stderr string
	Err    error
}

// Combined giver den fulde tekst til visning i aktivitetsloggen.
func (r result) Combined() string {
	var b strings.Builder
	if r.Stdout != "" {
		b.WriteString(r.Stdout)
	}
	if r.Stderr != "" {
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteString("\n")
		}
		b.WriteString(r.Stderr)
	}
	return strings.TrimRight(b.String(), "\n")
}

// run kører CLI'en med de givne argumenter og en valgfri stdin (til
// --password-stdin). En tom stdin betyder ingen stdin.
func (c *cli) run(ctx context.Context, stdin string, args ...string) result {
	cmd := exec.CommandContext(ctx, c.path, args...)
	hideConsole(cmd) // undgå et blinkende konsolvindue på Windows
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return result{Stdout: out.String(), Stderr: errb.String(), Err: err}
}

// status spejler `keepass-deltasync status`. enrolled er false hvis CLI'en
// melder "not enrolled" — det er sådan vi beslutter om guiden skal vises.
type status struct {
	Enrolled   bool
	Server     string
	User       string
	Device     string
	LastSeen   string
	ConfigPath string
	Raw        string
}

var notEnrolledRe = regexp.MustCompile(`(?i)not enrolled`)

func (c *cli) status(ctx context.Context) status {
	r := c.run(ctx, "", "status")
	combined := r.Combined()
	s := status{Raw: combined}
	if r.Err != nil || notEnrolledRe.MatchString(combined) {
		s.Enrolled = false
		return s
	}
	s.Enrolled = true
	for _, line := range strings.Split(r.Stdout, "\n") {
		k, v, ok := splitKV(line)
		if !ok {
			continue
		}
		switch k {
		case "Server":
			s.Server = v
		case "User":
			s.User = v
		case "Device":
			s.Device = v
		case "Last seen":
			s.LastSeen = v
		case "Config file":
			s.ConfigPath = v
		}
	}
	return s
}

// usernameOnly trækker brugernavnet ud af status-feltet "hans (uuid)" → "hans".
func usernameOnly(userField string) string {
	if i := strings.Index(userField, " ("); i > 0 {
		return userField[:i]
	}
	return strings.TrimSpace(userField)
}

// splitKV deler "Key:   value" op. Returnerer ok=false for linjer uden kolon.
func splitKV(line string) (key, val string, ok bool) {
	i := strings.Index(line, ":")
	if i < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:]), true
}

// database er én række fra `keepass-deltasync databases`.
type database struct {
	Name      string
	ID        string
	Created   string
	LocalPath string
	Bound     bool // markør '*' = bundet lokalt, klar til sync
}

var multiSpace = regexp.MustCompile(` {2,}`)

// databases spejler `keepass-deltasync databases` og parser tabwriter-tabellen.
// Tabellen ser sådan ud (kolonner adskilt af ≥2 mellemrum):
//
//	   NAME       ID        CREATED               LOCAL PATH
//	*  personal   2f3a…     2026-06-01T09:00:00Z  C:\Users\…\my.kdbx
//	?  shared     9c1b…     2026-06-02T10:00:00Z  (not bound locally)
func (c *cli) databases(ctx context.Context) ([]database, result) {
	r := c.run(ctx, "", "databases")
	if r.Err != nil {
		return nil, r
	}
	var out []database
	for _, line := range strings.Split(r.Stdout, "\n") {
		t := strings.TrimRight(line, "\r\n")
		if t == "" {
			continue
		}
		// Spring header og info-linjer over.
		if strings.Contains(t, "NAME") && strings.Contains(t, "LOCAL PATH") {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(t), "(") {
			continue
		}
		marker := ""
		rest := t
		switch {
		case strings.HasPrefix(t, "*"):
			marker, rest = "*", strings.TrimPrefix(t, "*")
		case strings.HasPrefix(strings.TrimSpace(t), "?"):
			marker = "?"
			rest = strings.TrimPrefix(strings.TrimSpace(t), "?")
		}
		fields := multiSpace.Split(strings.TrimSpace(rest), 4)
		if len(fields) < 2 {
			continue
		}
		db := database{Name: fields[0], Bound: marker == "*"}
		if len(fields) >= 2 {
			db.ID = fields[1]
		}
		if len(fields) >= 3 {
			db.Created = fields[2]
		}
		if len(fields) >= 4 {
			db.LocalPath = fields[3]
		}
		out = append(out, db)
	}
	return out, r
}

// member er én række fra `keepass-deltasync shares <db>` — et medlem (ejer eller
// member) der er koblet til databasen.
type member struct {
	Role        string
	Username    string
	DisplayName string
	AddedAt     string
}

// shares spejler `keepass-deltasync shares <db-name>` og parser tabwriter-tabellen:
//
//	ROLE    USERNAME  DISPLAY NAME      ADDED AT
//	owner   hans      Hans Bjørck-Baun  2026-06-04 14:50:36…
//
// Kun ejeren kan kalde dette; for andre (eller en database der ikke er bundet
// lokalt) returnerer CLI'en en fejl, som vises i panelet.
func (c *cli) shares(ctx context.Context, name string) ([]member, result) {
	r := c.run(ctx, "", "shares", name)
	if r.Err != nil {
		return nil, r
	}
	var out []member
	for _, line := range strings.Split(r.Stdout, "\n") {
		t := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(t) == "" {
			continue
		}
		if strings.Contains(t, "ROLE") && strings.Contains(t, "USERNAME") {
			continue // header
		}
		fields := multiSpace.Split(strings.TrimSpace(t), -1)
		if len(fields) < 3 {
			continue
		}
		// role user [display…] added — added er sidste felt, display er det
		// (evt. tomme) felt derimellem. Interne enkelt-mellemrum i et visningsnavn
		// splittes ikke, så display er højst ét felt.
		m := member{
			Role:        fields[0],
			Username:    fields[1],
			AddedAt:     fields[len(fields)-1],
			DisplayName: strings.Join(fields[2:len(fields)-1], " "),
		}
		out = append(out, m)
	}
	return out, r
}

// device er én række fra `keepass-deltasync devices` — en enhed der er tilmeldt
// på kontoen. Current markerer den enhed denne klient kører på.
type device struct {
	Name     string
	ID       string
	Enrolled string
	LastSeen string
	Current  bool
}

// devices spejler `keepass-deltasync devices` og parser tabwriter-tabellen:
//
//	   NAME      ID        ENROLLED              LAST SEEN
//	*  FrontPos  17294c43… 2026-06-04 14:50:36…  2026-06-16 12:05:02…
//
// Enheder hører til KONTOEN (ikke en enkelt database) — den aktuelle enhed
// markeres med '*' i terminalen og med Current her.
func (c *cli) devices(ctx context.Context) ([]device, result) {
	r := c.run(ctx, "", "devices")
	if r.Err != nil {
		return nil, r
	}
	var out []device
	for _, line := range strings.Split(r.Stdout, "\n") {
		t := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(t) == "" {
			continue
		}
		if strings.Contains(t, "NAME") && strings.Contains(t, "LAST SEEN") {
			continue // header
		}
		if strings.HasPrefix(strings.TrimSpace(t), "(") {
			continue // f.eks. "(no devices enrolled)"
		}
		current := strings.HasPrefix(t, "*")
		rest := strings.TrimSpace(strings.TrimPrefix(t, "*"))
		fields := multiSpace.Split(rest, 4)
		if len(fields) < 2 {
			continue
		}
		d := device{Name: fields[0], Current: current}
		if len(fields) >= 2 {
			d.ID = fields[1]
		}
		if len(fields) >= 3 {
			d.Enrolled = fields[2]
		}
		if len(fields) >= 4 {
			d.LastSeen = fields[3]
		}
		out = append(out, d)
	}
	return out, r
}

// logEntry er én række fra serverens audit-log (`keepass-deltasync log`).
type logEntry struct {
	Time  string // OccurredAt, server-tidsstempel (ISO/space-format)
	Level string // info / warn / error …
	Event string // event-type, fx "sync.push", "device.enroll"
	OK    bool   // Success
	IP    string // klient-IP (kan være tom)
}

// logEntries spejler `keepass-deltasync log [--since DUR] [--limit N]` og parser
// tabwriter-tabellen. since er en Go-duration ("24h", "168h"); tom = intet
// filter. Tabellen ser sådan ud (kolonner adskilt af ≥2 mellemrum):
//
//	TIME                  LEVEL  EVENT       OK    IP
//	2026-06-19T07:46:16Z  info   sync.push   OK    1.2.3.4
//
// Loggen ligger på SERVEREN (audit, op til 30 dages historik) — derfor viser
// den aktivitet på tværs af alle enheder, ikke kun denne klient.
func (c *cli) logEntries(ctx context.Context, since string, limit int) ([]logEntry, result) {
	args := []string{"log"}
	if since != "" {
		args = append(args, "--since", since)
	}
	if limit > 0 {
		args = append(args, "--limit", strconv.Itoa(limit))
	}
	r := c.run(ctx, "", args...)
	if r.Err != nil {
		return nil, r
	}
	var out []logEntry
	for _, line := range strings.Split(r.Stdout, "\n") {
		t := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(t) == "" {
			continue
		}
		if strings.Contains(t, "TIME") && strings.Contains(t, "LEVEL") {
			continue // header
		}
		if strings.HasPrefix(strings.TrimSpace(t), "(") {
			continue // "(no log entries)"
		}
		// TIME kan indeholde ét internt mellemrum (dato + tid); multiSpace
		// splitter kun på ≥2 mellemrum, så tidsstemplet forbliver ét felt.
		fields := multiSpace.Split(strings.TrimSpace(t), -1)
		if len(fields) < 4 {
			continue
		}
		e := logEntry{Time: fields[0], Level: fields[1], Event: fields[2], OK: fields[3] == "OK"}
		if len(fields) >= 5 {
			e.IP = fields[4]
		}
		out = append(out, e)
	}
	return out, r
}

// enroll spejler `keepass-deltasync enroll --server URL [--device-name N] token`.
func (c *cli) enroll(ctx context.Context, serverURL, deviceName, token string) result {
	args := []string{"enroll", "--server", serverURL}
	if deviceName != "" {
		args = append(args, "--device-name", deviceName)
	}
	args = append(args, token)
	return c.run(ctx, "", args...)
}

// initDB spejler `keepass-deltasync init <name> <local.kdbx>`.
func (c *cli) initDB(ctx context.Context, name, kdbxPath string) result {
	return c.run(ctx, "", "init", name, kdbxPath)
}

// sync spejler `keepass-deltasync sync --password-stdin <name>`. Masterpasswordet
// sendes via stdin, så det aldrig optræder på kommandolinjen eller i procestabellen.
//
// VIGTIGT: flaget SKAL stå før <name>. CLI'ens flag-pakke holder op med at læse
// flag ved det første positionelle argument, så `sync <name> --password-stdin`
// ville opfatte flaget som et ekstra argument og fejle.
func (c *cli) sync(ctx context.Context, name, masterPassword string) result {
	return c.run(ctx, masterPassword+"\n", "sync", "--password-stdin", name)
}

// share spejler `keepass-deltasync share --password-stdin <db> <username>`.
// Masterpasswordet sendes via stdin — serveren bruger det IKKE, men klienten
// skal bruge det lokalt til at wrappe database-nøglen til modtagerens enhed.
// Flaget SKAL stå før de positionelle argumenter (flag-pakke-reglen).
func (c *cli) share(ctx context.Context, db, username, masterPassword string) result {
	return c.run(ctx, masterPassword+"\n", "share", "--password-stdin", db, username)
}

// unshare spejler `keepass-deltasync unshare <db> <username>` — fjerner et medlem
// (eller lader medlemmet selv forlade). Intet password.
func (c *cli) unshare(ctx context.Context, db, username string) result {
	return c.run(ctx, "", "unshare", db, username)
}

// initBind spejler `keepass-deltasync init --bind <uuid> <name> <path>` — binder
// en EKSISTERENDE lokal .kdbx til en database der allerede findes på serveren
// (det normale "2. enhed, egen database"-flow). Flaget SKAL stå før de
// positionelle argumenter (flag-pakke-reglen).
func (c *cli) initBind(ctx context.Context, name, localPath, uuid string) result {
	return c.run(ctx, "", "init", "--bind", uuid, name, localPath)
}

// initShared spejler `keepass-deltasync init-shared --password-stdin <remote> <path>`.
// newPassword er et NYT lokalt password til den lokale .kdbx-kopi (uafhængigt af
// ejerens password).
func (c *cli) initShared(ctx context.Context, remoteName, localPath, newPassword string) result {
	return c.run(ctx, newPassword+"\n", "init-shared", "--password-stdin", remoteName, localPath)
}

// push spejler `keepass-deltasync push --password-stdin <name>` — ensrettet
// upload (som sync, men uden at pulle først). Masterpasswordet bruges lokalt til
// at læse den lokale .kdbx og sendes via stdin. Flaget SKAL stå før <name>.
func (c *cli) push(ctx context.Context, name, masterPassword string) result {
	return c.run(ctx, masterPassword+"\n", "push", "--password-stdin", name)
}

// pull spejler `keepass-deltasync pull --password-stdin <name>` — ensrettet
// download/merge fra serveren. Flaget SKAL stå før <name>.
func (c *cli) pull(ctx context.Context, name, masterPassword string) result {
	return c.run(ctx, masterPassword+"\n", "pull", "--password-stdin", name)
}

// deleteDatabase spejler `keepass-deltasync delete-database <id-eller-navn>` —
// sletter databasen PERMANENT på serveren (entries, versioner, delinger og
// historik; CASCADE). Kun ejeren kan slette. target er helst UUID'et (virker
// også for databaser der ikke er bundet lokalt), med navn som fallback. Den
// lokale .kdbx-fil røres aldrig.
func (c *cli) deleteDatabase(ctx context.Context, target string) result {
	return c.run(ctx, "", "delete-database", target)
}

// adminUser er én række fra `keepass-deltasync admin user-list`.
type adminUser struct {
	Username  string
	Display   string
	Status    string // "active" / "disabled"
	Devices   string // antal enheder (som tekst, direkte fra tabellen)
	Databases string // antal databaser
	Created   string // oprettelsesdato (YYYY-MM-DD)
	Disabled  bool
}

// adminUserList spejler `admin user-list --admin-token <tok>` og parser tabellen:
//
//	USERNAME  DISPLAY  STATUS  DEVICES  DATABASES  CREATED
//	hans      Hans B.  active  2        3          2026-06-01
func (c *cli) adminUserList(ctx context.Context, adminToken string) ([]adminUser, result) {
	r := c.run(ctx, "", "admin", "user-list", "--admin-token", adminToken)
	if r.Err != nil {
		return nil, r
	}
	var out []adminUser
	for _, line := range strings.Split(r.Stdout, "\n") {
		t := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(t) == "" {
			continue
		}
		if strings.Contains(t, "USERNAME") && strings.Contains(t, "STATUS") {
			continue // header
		}
		if strings.HasPrefix(strings.TrimSpace(t), "(") {
			continue // "(no users)"
		}
		f := multiSpace.Split(strings.TrimSpace(t), -1)
		if len(f) < 6 {
			continue
		}
		out = append(out, adminUser{
			Username: f[0], Display: f[1], Status: f[2],
			Devices: f[3], Databases: f[4], Created: f[5],
			Disabled: f[2] == "disabled",
		})
	}
	return out, r
}

// adminUserCreate spejler `admin user-create <username> [--display-name N] [--server URL] --admin-token <tok>`.
// server er normalt tom (URL'en læses fra config.toml); den avancerede tilmelding
// sender den eksplicit, da den kører på en maskine der endnu ikke er enrolled.
func (c *cli) adminUserCreate(ctx context.Context, server, adminToken, username, displayName string) result {
	args := []string{"admin", "user-create", username, "--admin-token", adminToken}
	if server != "" {
		args = append(args, "--server", server)
	}
	if displayName != "" {
		args = append(args, "--display-name", displayName)
	}
	return c.run(ctx, "", args...)
}

// adminUserEnrollment spejler `admin user-enrollment <username> [--server URL] --admin-token <tok>`.
// server er normalt tom (læses fra config.toml); avanceret tilmelding sender den
// eksplicit på en frisk maskine.
func (c *cli) adminUserEnrollment(ctx context.Context, server, adminToken, username string) result {
	args := []string{"admin", "user-enrollment", username, "--admin-token", adminToken}
	if server != "" {
		args = append(args, "--server", server)
	}
	return c.run(ctx, "", args...)
}

// parseEnrollToken trækker enrollment-tokenet ud af outputtet fra `admin
// user-create` og `admin user-enrollment`. Begge kommandoer afslutter med en
// linje på formen
//
//	keepass-deltasync enroll --server <url> <token>
//
// hvor tokenet er sidste felt — et stabilt anker på tværs af begge outputs.
// Returnerer tom streng hvis ingen sådan linje findes.
func parseEnrollToken(stdout string) string {
	for _, line := range strings.Split(stdout, "\n") {
		t := strings.TrimSpace(strings.TrimRight(line, "\r\n"))
		if strings.Contains(t, "enroll --server ") {
			f := strings.Fields(t)
			if len(f) > 0 {
				return f[len(f)-1]
			}
		}
	}
	return ""
}

// adminSetDisabled spejler `admin user-disable|user-enable <username> --admin-token <tok>`.
func (c *cli) adminSetDisabled(ctx context.Context, adminToken, username string, disabled bool) result {
	sub := "user-enable"
	if disabled {
		sub = "user-disable"
	}
	return c.run(ctx, "", "admin", sub, username, "--admin-token", adminToken)
}

// adminUserDelete spejler `admin user-delete <username> --yes --admin-token <tok>`.
// --yes springer CLI'ens interaktive stdin-bekræftelse over (som GUI'en ikke kan
// besvare); vi bekræfter i stedet i en dialog FØR dette kald.
func (c *cli) adminUserDelete(ctx context.Context, adminToken, username string) result {
	return c.run(ctx, "", "admin", "user-delete", username, "--yes", "--admin-token", adminToken)
}

// adminTokenSQL spejler `admin token-sql` — printer SQL til at oprette en frisk
// admin-token (kræver ingen token selv).
func (c *cli) adminTokenSQL(ctx context.Context) result {
	return c.run(ctx, "", "admin", "token-sql")
}

// entryVersion er én række fra `keepass-deltasync versions <name> <uuid>`.
type entryVersion struct {
	Num      string // "1".."3"
	State    string // "current"/"previous"/"oldest" (+ evt. " (deleted)")
	Modified string
	Created  string
}

// versions spejler `versions <name> <entry-uuid>` og parser tabellen:
//
//	VER  STATE     MODIFIED              CREATED
//	3    current   2026-06-19T05:52:27Z  2026-06-18T10:00:00Z
func (c *cli) versions(ctx context.Context, name, entryUUID string) ([]entryVersion, result) {
	r := c.run(ctx, "", "versions", name, entryUUID)
	if r.Err != nil {
		return nil, r
	}
	var out []entryVersion
	for _, line := range strings.Split(r.Stdout, "\n") {
		t := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(t) == "" {
			continue
		}
		if strings.Contains(t, "VER") && strings.Contains(t, "STATE") {
			continue // header
		}
		if strings.HasPrefix(strings.TrimSpace(t), "(") {
			continue // "(no versions …)"
		}
		f := multiSpace.Split(strings.TrimSpace(t), -1)
		if len(f) < 4 {
			continue
		}
		out = append(out, entryVersion{Num: f[0], State: f[1], Modified: f[2], Created: f[3]})
	}
	return out, r
}

// restore spejler `restore <name> <entry-uuid> <version-num>` — promoverer en
// gammel version til ny nyeste server-side. Brugeren skal sync'e bagefter.
func (c *cli) restore(ctx context.Context, name, entryUUID, versionNum string) result {
	return c.run(ctx, "", "restore", name, entryUUID, versionNum)
}

// withTimeout giver en context med en fornuftig grænse til en CLI-kørsel.
// Sync kan tage tid (merge via keepassxc-cli), så den får rigeligt.
func withTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// describeErr giver en kort, læsbar fejltekst når en kørsel fejler.
func describeErr(r result) string {
	if r.Err == nil {
		return ""
	}
	msg := strings.TrimSpace(r.Stderr)
	if msg == "" {
		msg = r.Err.Error()
	}
	return fmt.Sprintf("%s", msg)
}
