// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx"
	"gitlab.com/Star95/keepass-deltasync/client/internal/keyring"
	"gitlab.com/Star95/keepass-deltasync/client/internal/passwd"
)

// stringSliceFlag tillader gentagne --db <name> flag på kommandolinjen.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error { *s = append(*s, v); return nil }

// runDaemon kører i forgrunden og synker valgte databaser ved (a) periodisk
// polling og (b) fsnotify-events på den lokale .kdbx-fil. SIGINT/SIGTERM
// stopper graceful'ly — workers afventer en evt. igangværende sync før
// daemon afslutter.
//
// Password håndteres pr. database: vi forsøger OS keyring først (key =
// db.RemoteID), og falder tilbage til en interaktiv prompt. Med
// --store-keyring gemmes prompted passwords i keyring efter første brug, så
// senere starts kører helt non-interaktivt.
func runDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	var dbNames stringSliceFlag
	fs.Var(&dbNames, "db", "database name (kan gentages; default: alle registrerede)")
	pollInterval := fs.Duration("poll-interval", 60*time.Second, "polling-interval mellem syncs")
	debounce := fs.Duration("debounce", 3*time.Second, "fsnotify-debounce før sync trigges")
	storeKeyring := fs.Bool("store-keyring", false, "gem prompted passwords i OS keyring efter første brug")
	pwStdin := fs.Bool("password-stdin", false, "læs passwords fra stdin (én linje pr. database, i config-rækkefølgen eller --db-rækkefølgen)")
	cliPath := fs.String("keepassxc-cli", "", "path til keepassxc-cli (overrider auto-detection)")
	syncTimeout := fs.Duration("sync-timeout", 10*time.Minute, "max wall-clock pr. sync-cycle")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync daemon [--db NAME]... [--poll-interval DUR] [--debounce DUR] [--store-keyring] [--password-stdin] [--keepassxc-cli PATH] [--sync-timeout DUR]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return errors.New("daemon takes no positional arguments")
	}
	if *pollInterval < time.Second {
		return errors.New("--poll-interval must be at least 1s")
	}
	if *debounce < 100*time.Millisecond {
		return errors.New("--debounce must be at least 100ms")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Server.URL == "" || cfg.Server.DeviceToken == "" {
		return errors.New("not enrolled — run `keepass-deltasync enroll` first")
	}
	if len(cfg.Databases) == 0 {
		return errors.New("no databases registered — run `keepass-deltasync init` first")
	}

	selected, err := selectDatabases(cfg, []string(dbNames))
	if err != nil {
		return err
	}

	workers, err := setupDaemonWorkers(cfg, selected, *cliPath, *pwStdin, *storeKeyring, *pollInterval, *debounce, *syncTimeout)
	if err != nil {
		for _, w := range workers {
			w.zeroKeys()
		}
		return err
	}

	state := &daemonState{cfg: cfg}

	rootCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	fmt.Fprintf(os.Stderr, "Daemon started — watching %d database(s). Ctrl+C to stop.\n", len(workers))

	var wg sync.WaitGroup
	for _, w := range workers {
		w := w
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.run(rootCtx, state)
		}()
	}
	wg.Wait()

	for _, w := range workers {
		w.zeroKeys()
	}
	fmt.Fprintln(os.Stderr, "Daemon stopped.")
	return nil
}

// selectDatabases mapper --db-navne til pointers ind i cfg.Databases. Default
// (ingen --db) returnerer alle.
func selectDatabases(cfg *config.Config, names []string) ([]*config.Database, error) {
	if len(names) == 0 {
		out := make([]*config.Database, len(cfg.Databases))
		for i := range cfg.Databases {
			out[i] = &cfg.Databases[i]
		}
		return out, nil
	}
	out := make([]*config.Database, 0, len(names))
	for _, name := range names {
		db := cfg.FindDatabase(name)
		if db == nil {
			return nil, fmt.Errorf("database %q not found in local config", name)
		}
		out = append(out, db)
	}
	return out, nil
}

// daemonState er den state der deles mellem alle workers — config-pointeren
// og den serialiserende mutex der dækker både cfg-mutering og
// config.Save(). Vi serialiserer ALLE syncs på tværs af databaser; for v1
// med 1-3 lokale databaser er det acceptabelt og udelukker race conditions
// på cfg.Databases og dens nested EntryStates-maps under toml-encoding.
type daemonState struct {
	cfg    *config.Config
	syncMu sync.Mutex
}

// coalescer afkobler triggre fra arbejde: flere kald til request() under et
// igangværende run resulterer i præcis ÉN ekstra run efter, ikke N. Bruges
// af dbWorker så bursts af fsnotify-events ikke spawner en lang sync-kø.
type coalescer struct {
	mu      sync.Mutex
	running bool
	pending bool
}

// request markerer arbejde som ønsket. Hvis runLoop ikke kører, startes en
// goroutine; ellers sættes pending så runLoop'et laver en ekstra runde efter
// den nuværende.
func (c *coalescer) request(work func()) {
	c.mu.Lock()
	if c.running {
		c.pending = true
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()
	go c.runLoop(work)
}

func (c *coalescer) runLoop(work func()) {
	for {
		work()
		c.mu.Lock()
		if !c.pending {
			c.running = false
			c.mu.Unlock()
			return
		}
		c.pending = false
		c.mu.Unlock()
	}
}

// wait blokerer indtil enhver igangværende runLoop er færdig.
func (c *coalescer) wait() {
	for {
		c.mu.Lock()
		running := c.running
		c.mu.Unlock()
		if !running {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// dbWorker holder per-database state: keys, fsnotify-watcher-input, og
// coalescing-state for sync-triggers.
type dbWorker struct {
	cfg          *config.Config
	db           *config.Database
	cli          *kdbx.CLI
	password     []byte
	masterKey    []byte
	entryKey     []byte
	pollInterval time.Duration
	debounce     time.Duration
	syncTimeout  time.Duration

	sync coalescer

	// debounceTimer styres kun fra main worker-loopet (single-threaded), så
	// behøver ikke ekstra lås.
	debounceTimer *time.Timer
}

func (w *dbWorker) zeroKeys() {
	passwd.Zero(w.password)
	passwd.Zero(w.masterKey)
	passwd.Zero(w.entryKey)
}

// setupDaemonWorkers løser passwords for alle valgte databaser via keyring
// eller prompt, deriverer keys, og bygger workers. Returnerer workers
// undervejs så caller'en kan zero dem hvis setup fejler midtvejs.
func setupDaemonWorkers(cfg *config.Config, dbs []*config.Database, cliPath string, pwStdin, storeKeyring bool, pollInterval, debounce, syncTimeout time.Duration) ([]*dbWorker, error) {
	cli, err := kdbx.NewCLI(cliPath)
	if err != nil {
		return nil, err
	}
	for _, db := range dbs {
		if _, err := os.Stat(db.LocalPath); err != nil {
			return nil, fmt.Errorf("local kdbx %s: %w", db.LocalPath, err)
		}
	}

	workers := make([]*dbWorker, 0, len(dbs))
	for _, db := range dbs {
		pw, fromKeyring, err := resolvePassword(db, pwStdin)
		if err != nil {
			return workers, err
		}
		masterKey, entryKey, err := deriveKeys(pw, db.RemoteID)
		if err != nil {
			passwd.Zero(pw)
			return workers, err
		}
		if storeKeyring && !fromKeyring {
			if err := keyring.Set(db.RemoteID, pw); err != nil {
				fmt.Fprintf(os.Stderr, "[%s] warning: could not store password in keyring: %v\n", db.Name, err)
			} else {
				fmt.Fprintf(os.Stderr, "[%s] password stored in OS keyring.\n", db.Name)
			}
		}
		workers = append(workers, &dbWorker{
			cfg:          cfg,
			db:           db,
			cli:          cli,
			password:     pw,
			masterKey:    masterKey,
			entryKey:     entryKey,
			pollInterval: pollInterval,
			debounce:     debounce,
			syncTimeout:  syncTimeout,
		})
	}
	return workers, nil
}

// resolvePassword forsøger OS-keyring først, falder tilbage til
// interaktiv/stdin prompt. fromKeyring=true betyder at vi fandt et gemt
// password; caller'en bruger det til at afgøre om --store-keyring skal trigge
// en write (vi vil ikke konstant skrive til keyring hvis den allerede har
// værdien).
func resolvePassword(db *config.Database, pwStdin bool) (pw []byte, fromKeyring bool, err error) {
	pw, kErr := keyring.Get(db.RemoteID)
	if kErr == nil {
		fmt.Fprintf(os.Stderr, "[%s] password loaded from OS keyring.\n", db.Name)
		return pw, true, nil
	}
	if !errors.Is(kErr, keyring.ErrNotFound) {
		fmt.Fprintf(os.Stderr, "[%s] warning: keyring lookup failed (%v); falling back to prompt\n", db.Name, kErr)
	}
	prompt := fmt.Sprintf("Masterpassword for %s: ", db.Name)
	pw, err = passwd.Read(prompt, pwStdin)
	if err != nil {
		return nil, false, err
	}
	return pw, false, nil
}

// run kører worker'ens main-loop indtil ctx cancelles. Setup'er
// fsnotify-watcher på den mappe der indeholder .kdbx-filen (KeePassXC skriver
// via tmp+rename, så vi filtrerer events på basename), laver en initial sync,
// og loop'er på events/timers.
func (w *dbWorker) run(ctx context.Context, state *daemonState) {
	logf := func(format string, args ...interface{}) {
		prefix := fmt.Sprintf("[%s] ", w.db.Name)
		fmt.Fprintf(os.Stderr, prefix+format+"\n", args...)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logf("could not create fsnotify watcher: %v — worker exiting", err)
		return
	}
	defer watcher.Close()

	watchDir := filepath.Dir(w.db.LocalPath)
	if err := watcher.Add(watchDir); err != nil {
		logf("could not watch %s: %v — worker exiting", watchDir, err)
		return
	}
	baseName := filepath.Base(w.db.LocalPath)

	pollTicker := time.NewTicker(w.pollInterval)
	defer pollTicker.Stop()

	w.requestSync(ctx, state, logf)

	for {
		select {
		case <-ctx.Done():
			w.stopDebounce()
			logf("shutdown requested, waiting for in-flight sync...")
			w.sync.wait()
			return

		case <-pollTicker.C:
			w.requestSync(ctx, state, logf)

		case ev, ok := <-watcher.Events:
			if !ok {
				return
			}
			if !eventConcernsFile(ev, baseName) {
				continue
			}
			w.startDebounce(ctx, state, logf)

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logf("fsnotify error: %v", err)
		}
	}
}

// eventConcernsFile filtrerer fsnotify-events til kun dem der vedrører vores
// .kdbx-fil (basename-match) og operationer der signalerer skrivning.
// KeePassXC's atomic save på Windows giver CREATE+RENAME-sekvens; på Linux
// typisk WRITE+CHMOD. Vi inkluderer alle modify-lignende ops; debounce sørger
// for at vi ikke trigger'er midt i en multi-event-skrivning.
func eventConcernsFile(ev fsnotify.Event, baseName string) bool {
	if filepath.Base(ev.Name) != baseName {
		return false
	}
	return ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0
}

// startDebounce (re)starter debounce-timeren. Bruges fra worker-loopet ved
// hver kvalificeret event — flere events i hurtig rækkefølge (fx
// keepassxc-cli's tmp+rename-sekvens) ender med kun én sync-trigger.
func (w *dbWorker) startDebounce(ctx context.Context, state *daemonState, logf func(string, ...interface{})) {
	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
	}
	w.debounceTimer = time.AfterFunc(w.debounce, func() {
		w.requestSync(ctx, state, logf)
	})
}

func (w *dbWorker) stopDebounce() {
	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
	}
}

// requestSync delegerer til coalescer: hvis en sync allerede kører, markeres
// pending; ellers startes syncLoop'et.
func (w *dbWorker) requestSync(ctx context.Context, state *daemonState, logf func(string, ...interface{})) {
	w.sync.request(func() { w.doSync(ctx, state, logf) })
}

// doSync kører én pull+push cycle under state.syncMu. Mutex'en serialiserer
// på tværs af workers, hvilket undgår race conditions på cfg-pointeren og
// dens nested EntryStates-maps under toml-encoding ved config.Save.
func (w *dbWorker) doSync(ctx context.Context, state *daemonState, logf func(string, ...interface{})) {
	if ctx.Err() != nil {
		return
	}
	state.syncMu.Lock()
	defer state.syncMu.Unlock()
	if ctx.Err() != nil {
		return
	}

	env := newRunEnvBorrowed(ctx, w.cfg, w.db, w.cli, w.password, w.masterKey, w.entryKey, w.syncTimeout)
	env.quiet = true
	defer env.cleanup()

	newSeq, merged, deletions, err := env.pullChanges()
	if err != nil {
		logf("pull error: %v", err)
		return
	}

	pushed, deleted, maxPushSeq, err := env.pushChanges(false)
	if err != nil {
		logf("push error: %v", err)
		if maxPushSeq > newSeq {
			newSeq = maxPushSeq
		}
		w.db.LastSeq = newSeq
		if serr := config.Save(state.cfg); serr != nil {
			logf("save error: %v", serr)
		}
		return
	}

	if maxPushSeq > newSeq {
		newSeq = maxPushSeq
	}
	w.db.LastSeq = newSeq
	if err := config.Save(state.cfg); err != nil {
		logf("save error: %v", err)
		return
	}

	if merged > 0 || deletions > 0 || pushed > 0 || deleted > 0 {
		logf("sync: pulled %d (+ %d tombstones), pushed %d (+ %d tombstones). last_seq=%d",
			merged, deletions, pushed, deleted, newSeq)
	}
}
