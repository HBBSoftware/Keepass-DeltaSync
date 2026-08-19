// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx"
	"gitlab.com/Star95/keepass-deltasync/client/internal/keyring"
	"gitlab.com/Star95/keepass-deltasync/client/internal/passwd"
)

// Native messaging-host til Firefox-udvidelsen. Se docs/browser-extension.md
// for designet. Kort fortalt: udvidelsen kan søge i entries og navigere til
// deres URL, men får aldrig hemmeligheder at se — udfyldning af credentials
// er KeePassXC-Browsers job.
//
// Hosten kontakter ALDRIG serveren. Den læser den lokale .kdbx og intet andet.

const (
	// hostName er navnet i native-manifestet. Firefox kræver [a-z0-9_.]+.
	hostName = "dk.hbb.keepass_deltasync"

	// browserExtensionID skal matche browser_specific_settings.gecko.id i
	// extension/manifest.json. Ændres det ét sted, skal det ændres begge.
	browserExtensionID = "keepass-deltasync@hb-b.dk"

	// maxIncoming er vores egen grænse for en besked fra udvidelsen. Alt
	// derover er enten en fejl eller en desynkroniseret framing, og i begge
	// tilfælde er det rigtige at give op frem for at allokere.
	maxIncoming = 1 << 20

	// pageBytes er målstørrelsen for én indeks-side. Firefox tillader højst
	// 1 MB pr. besked FRA applikationen, så store databaser hentes i sider.
	pageBytes = 512 * 1024

	exportTimeout   = 2 * time.Minute
	watchDebounce   = 2 * time.Second
	defaultIdleLock = 15 * time.Minute
)

// hostRequest er én besked fra udvidelsen.
type hostRequest struct {
	ID     int    `json:"id"`
	Cmd    string `json:"cmd"`
	DB     string `json:"db,omitempty"`
	Offset int    `json:"offset,omitempty"`

	// Password bruges kun når databasen ikke har en entry i OS-keyringen.
	// Normalvejen er at hosten selv henter passwordet, så det aldrig
	// passerer browserprocessen.
	Password string `json:"password,omitempty"`
}

// hostResponse er både svar på en request (ID sat) og uopfordrede events
// (Event sat, ID = 0).
type hostResponse struct {
	ID      int    `json:"id,omitempty"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Event   string `json:"event,omitempty"`
	Version string `json:"version,omitempty"`

	// NeedPassword fortæller udvidelsen at den skal vise password-feltet:
	// der er ingen keyring-entry for databasen.
	NeedPassword bool `json:"need_password,omitempty"`

	DB         string       `json:"db,omitempty"`
	Databases  []dbStatus   `json:"databases,omitempty"`
	Entries    []indexEntry `json:"entries,omitempty"`
	Count      int          `json:"count"`
	Offset     int          `json:"offset"`
	Next       int          `json:"next"`
	Generation int64        `json:"generation,omitempty"`
}

// dbStatus er hvad udvidelsen får at vide om en registreret database.
// Bemærk at den lokale filsti IKKE er med — browseren har intet at bruge den
// til, og den er unødvendig information at eksponere.
type dbStatus struct {
	Name     string `json:"name"`
	Unlocked bool   `json:"unlocked"`
	Count    int    `json:"count"`
}

// browserHost holder sessionens tilstand. Indekset (titel + URL) er ikke
// hemmeligt keymateriale, men det er følsom metadata, så det ryddes ved
// `lock` og lever kun i denne proces.
type browserHost struct {
	cfg      *config.Config
	cli      *kdbx.CLI
	idleLock time.Duration

	mu         sync.Mutex
	index      map[string][]indexEntry
	generation int64
	watching   map[string]bool

	// fallbackPW holder masterpasswords for databaser UDEN keyring-entry.
	// Den vej er undtagelsen; med keyring holder hosten intet hemmeligt
	// mellem to unlocks. Nulstilles af idle-timeren og af `lock`.
	fallbackPW map[string][]byte
	pwTimers   map[string]*time.Timer

	writeMu sync.Mutex
	out     io.Writer
}

func runBrowserHost(args []string) error {
	fs := flag.NewFlagSet("browser-host", flag.ContinueOnError)
	cliPath := fs.String("keepassxc-cli", "", "path to keepassxc-cli binary (overrides auto-detection)")
	idle := fs.Duration("idle-lock", defaultIdleLock, "zero a password held in memory after this idle period (only used when the database has no OS keyring entry)")
	probe := fs.String("probe", "", "diagnostic: build the index for this database, print it as JSON, and exit (no browser involved)")
	pwStdin := fs.Bool("password-stdin", false, "with --probe: read masterpassword from stdin instead of prompting")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync browser-host [--probe NAME] [--idle-lock DURATION] [--keepassxc-cli PATH]")
		fmt.Fprintln(fs.Output(), "\nSpeaks Firefox' native messaging protocol on stdin/stdout. Normally launched")
		fmt.Fprintln(fs.Output(), "by Firefox itself — see `keepass-deltasync install-browser-host`.")
		fs.PrintDefaults()
	}
	// Firefox starter hosten med manifest-stien som argument. Den er
	// uinteressant for os, men må ikke få flag-parsing til at fejle — og
	// det gør den ikke, for flag stopper ved første ikke-flag.
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cli, err := kdbx.NewCLI(*cliPath)
	if err != nil {
		return err
	}

	h := &browserHost{
		cfg:        cfg,
		cli:        cli,
		idleLock:   *idle,
		index:      make(map[string][]indexEntry),
		watching:   make(map[string]bool),
		fallbackPW: make(map[string][]byte),
		pwTimers:   make(map[string]*time.Timer),
	}
	defer h.zeroAll()

	if *probe != "" {
		return h.runProbe(*probe, *pwStdin)
	}
	return h.serve(os.Stdin, os.Stdout)
}

// serve kører beskedløkken indtil Firefox lukker stdin.
func (h *browserHost) serve(in io.Reader, out io.Writer) error {
	h.out = out
	for {
		msg, err := readNativeMessage(in)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var req hostRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			h.emit(hostResponse{Error: fmt.Sprintf("malformed request: %v", err)})
			continue
		}
		resp := h.handle(req)
		resp.ID = req.ID
		h.emit(resp)
	}
}

func (h *browserHost) handle(req hostRequest) hostResponse {
	switch req.Cmd {
	case "status":
		return h.status()
	case "unlock":
		return h.unlock(req)
	case "index":
		return h.indexPage(req)
	case "lock":
		h.zeroAll()
		return hostResponse{OK: true}
	default:
		return hostResponse{Error: fmt.Sprintf("unknown command %q", req.Cmd)}
	}
}

func (h *browserHost) status() hostResponse {
	h.mu.Lock()
	defer h.mu.Unlock()

	dbs := make([]dbStatus, 0, len(h.cfg.Databases))
	for i := range h.cfg.Databases {
		name := h.cfg.Databases[i].Name
		idx, unlocked := h.index[name]
		dbs = append(dbs, dbStatus{Name: name, Unlocked: unlocked, Count: len(idx)})
	}
	return hostResponse{OK: true, Databases: dbs, Version: version, Generation: h.generation}
}

// unlock åbner databasen, bygger indekset og smider nøglematerialet væk igen.
//
// Rækkefølgen er bevidst: password hentes så sent som muligt, XML-eksporten
// nulstilles så tidligt som muligt, og svaret indeholder kun et tal (antal
// entries) — selve indekset hentes bagefter med `index`, side for side.
func (h *browserHost) unlock(req hostRequest) hostResponse {
	db := h.findDB(req.DB)
	if db == nil {
		return hostResponse{Error: fmt.Sprintf("database %q is not registered locally", req.DB)}
	}

	pw, callerOwns, err := h.acquirePassword(db, req.Password)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return hostResponse{
				DB:           db.Name,
				NeedPassword: true,
				Error:        "no masterpassword stored in the OS keyring for this database",
			}
		}
		return hostResponse{DB: db.Name, Error: err.Error()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), exportTimeout)
	defer cancel()
	xmlBytes, exportErr := h.cli.Export(ctx, db.LocalPath, pw)
	if callerOwns {
		passwd.Zero(pw)
	}
	if exportErr != nil {
		// Typisk et forkert password. Smid en gemt fallback væk, ellers
		// ville udvidelsen sidde fast på den forkerte værdi.
		h.forgetPassword(db.Name)
		return hostResponse{DB: db.Name, Error: exportErr.Error()}
	}

	idx, err := buildIndex(db.Name, xmlBytes)
	zeroBytes(xmlBytes)
	if err != nil {
		return hostResponse{DB: db.Name, Error: err.Error()}
	}

	h.mu.Lock()
	h.index[db.Name] = idx
	h.generation++
	gen := h.generation
	h.mu.Unlock()

	h.startWatch(db.Name, db.LocalPath)

	return hostResponse{OK: true, DB: db.Name, Count: len(idx), Generation: gen}
}

func (h *browserHost) indexPage(req hostRequest) hostResponse {
	db := h.findDB(req.DB)
	if db == nil {
		return hostResponse{Error: fmt.Sprintf("database %q is not registered locally", req.DB)}
	}

	h.mu.Lock()
	idx, ok := h.index[db.Name]
	gen := h.generation
	h.mu.Unlock()
	if !ok {
		return hostResponse{DB: db.Name, Error: "database is locked — send `unlock` first"}
	}

	page, next, err := paginate(idx, req.Offset, pageBytes)
	if err != nil {
		return hostResponse{DB: db.Name, Error: err.Error()}
	}
	return hostResponse{
		OK:         true,
		DB:         db.Name,
		Entries:    page,
		Count:      len(idx),
		Offset:     req.Offset,
		Next:       next,
		Generation: gen,
	}
}

// findDB slår en database op på navn. Tomt navn betyder "den eneste", hvilket
// dækker det normale tilfælde med præcis én database.
func (h *browserHost) findDB(name string) *config.Database {
	if name == "" {
		if len(h.cfg.Databases) == 1 {
			return &h.cfg.Databases[0]
		}
		return nil
	}
	return h.cfg.FindDatabase(name)
}

// acquirePassword returnerer masterpasswordet for databasen.
//
// Prioriteten er: (1) et password udvidelsen lige har sendt, (2) et vi
// allerede har gemt for en keyring-løs database, (3) OS-keyringen. Kun
// keyring-vejen giver callerOwns=true — de to andre peger ind i vores egen
// cache, som idle-timeren ejer.
func (h *browserHost) acquirePassword(db *config.Database, provided string) (pw []byte, callerOwns bool, err error) {
	if provided != "" {
		// Go-strings kan ikke nulstilles; kopien fra JSON-parsingen lever
		// til GC tager den. Vores egen kopi kan vi til gengæld styre, og
		// den er den eneste der overlever kaldet.
		h.rememberPassword(db.Name, []byte(provided))
		return h.cachedPassword(db.Name), false, nil
	}
	if cached := h.cachedPassword(db.Name); cached != nil {
		return cached, false, nil
	}
	stored, err := keyring.Get(db.RemoteID)
	if err != nil {
		return nil, false, err
	}
	return stored, true, nil
}

func (h *browserHost) cachedPassword(name string) []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	pw := h.fallbackPW[name]
	if pw != nil {
		h.resetIdleLocked(name)
	}
	return pw
}

func (h *browserHost) rememberPassword(name string, pw []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old := h.fallbackPW[name]; old != nil {
		passwd.Zero(old)
	}
	h.fallbackPW[name] = pw
	h.resetIdleLocked(name)
}

// resetIdleLocked (kaldes med h.mu holdt) starter idle-timeren forfra.
func (h *browserHost) resetIdleLocked(name string) {
	if t := h.pwTimers[name]; t != nil {
		t.Stop()
	}
	h.pwTimers[name] = time.AfterFunc(h.idleLock, func() { h.forgetPassword(name) })
}

// forgetPassword nulstiller et cachet password. Indekset bevares — det er
// metadata, ikke nøglemateriale, og udvidelsen skal kunne blive ved med at
// søge. `lock` rydder begge dele.
func (h *browserHost) forgetPassword(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if pw := h.fallbackPW[name]; pw != nil {
		passwd.Zero(pw)
		delete(h.fallbackPW, name)
	}
	if t := h.pwTimers[name]; t != nil {
		t.Stop()
		delete(h.pwTimers, name)
	}
}

func (h *browserHost) zeroAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for name, pw := range h.fallbackPW {
		passwd.Zero(pw)
		delete(h.fallbackPW, name)
	}
	for name, t := range h.pwTimers {
		t.Stop()
		delete(h.pwTimers, name)
	}
	for name := range h.index {
		delete(h.index, name)
	}
	h.generation++
}

// startWatch holder øje med den lokale .kdbx og sender `changed` til
// udvidelsen når filen er blevet skrevet — typisk fordi en sync lige er
// landet. Udvidelsen svarer med et nyt `unlock`, som for keyring-databaser
// er lydløst.
func (h *browserHost) startWatch(name, path string) {
	h.mu.Lock()
	if h.watching[name] {
		h.mu.Unlock()
		return
	}
	h.watching[name] = true
	h.mu.Unlock()

	go h.watch(name, path)
}

func (h *browserHost) watch(name, path string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] fsnotify unavailable (%v) — change notifications disabled\n", name, err)
		return
	}
	defer watcher.Close()

	// Vi ser på mappen, ikke filen: gemmeoperationer erstatter typisk
	// .kdbx'en via rename, hvilket river en fil-watch med sig.
	if err := watcher.Add(filepath.Dir(path)); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] cannot watch %s (%v) — change notifications disabled\n", name, filepath.Dir(path), err)
		return
	}
	base := filepath.Base(path)

	var pending *time.Timer
	var fire <-chan time.Time
	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				return
			}
			if filepath.Base(ev.Name) != base {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			if pending == nil {
				pending = time.NewTimer(watchDebounce)
			} else {
				pending.Reset(watchDebounce)
			}
			fire = pending.C
		case <-fire:
			pending, fire = nil, nil
			h.emit(hostResponse{OK: true, Event: "changed", DB: name})
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "[%s] watcher error: %v\n", name, err)
		}
	}
}

// runProbe er stdio-harnesset fra fase 1: byg indekset og skriv det som
// almindelig JSON, uden native messaging-framing og uden browser. Bruges til
// at verificere filtrering og URL-udtræk mod en rigtig database.
func (h *browserHost) runProbe(name string, pwStdin bool) error {
	db := h.findDB(name)
	if db == nil {
		return fmt.Errorf("database %q is not registered locally", name)
	}

	pw, err := keyring.Get(db.RemoteID)
	if errors.Is(err, keyring.ErrNotFound) {
		fmt.Fprintf(os.Stderr, "No keyring entry for %s — the extension would prompt here.\n", db.Name)
		pw, err = passwd.Read(fmt.Sprintf("Masterpassword for %s: ", db.Name), pwStdin)
	}
	if err != nil {
		return err
	}
	defer passwd.Zero(pw)

	ctx, cancel := context.WithTimeout(context.Background(), exportTimeout)
	defer cancel()
	xmlBytes, err := h.cli.Export(ctx, db.LocalPath, pw)
	if err != nil {
		return err
	}
	idx, err := buildIndex(db.Name, xmlBytes)
	zeroBytes(xmlBytes)
	if err != nil {
		return err
	}

	out, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", out)
	fmt.Fprintf(os.Stderr, "\n%d searchable entries indexed from %s.\n", len(idx), db.Name)
	return nil
}

// emit skriver én besked. Både beskedløkken og fs-watcherne skriver, så
// serialiseringen skal være låst — en halvskrevet besked ville desynkronisere
// framingen permanent.
func (h *browserHost) emit(resp hostResponse) {
	body, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot marshal response: %v\n", err)
		return
	}

	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	var hdr [4]byte
	binary.NativeEndian.PutUint32(hdr[:], uint32(len(body)))
	if _, err := h.out.Write(hdr[:]); err != nil {
		return
	}
	if _, err := h.out.Write(body); err != nil {
		return
	}
	if f, ok := h.out.(*os.File); ok {
		_ = f.Sync()
	}
}

// readNativeMessage læser én længdeprefixet besked. Firefox bruger værtens
// egen byte-orden på de fire længdebytes — ikke network byte order.
func readNativeMessage(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.NativeEndian.Uint32(hdr[:])
	if n == 0 {
		return nil, errors.New("native message with zero length")
	}
	if n > maxIncoming {
		return nil, fmt.Errorf("native message of %d bytes exceeds the %d byte limit", n, maxIncoming)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// zeroBytes overskriver en buffer der har indeholdt klartekst fra databasen.
// XML-eksporten rummer ALLE felter, inklusive passwords, så den skal ikke
// ligge og flyde længere end indekseringen tager.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
