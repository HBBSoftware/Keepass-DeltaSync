// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/crypto"
	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx"
	"gitlab.com/Star95/keepass-deltasync/client/internal/passwd"
)

// lastModificationTimeRe fanger entry'ens egen <LastModificationTime>. Vi
// erstatter altid den første forekomst — der ligger inde i <Times>-blokken
// FØR <History>, så efterfølgende historiske forekomster bevares uændret.
var lastModificationTimeRe = regexp.MustCompile(`<LastModificationTime>[^<]*</LastModificationTime>`)

// runEnv samler den shared state alle synkroniserings-kommandoer (pull/push/
// sync) skal bruge: API-klient, kdbx-CLI, config, masterpassword og deriverede
// nøgler. Bygges via setupEnv og frigøres via cleanup() (zeroer alle hemmelige
// bytes).
type runEnv struct {
	ctx       context.Context
	cancel    context.CancelFunc
	client    *api.Client
	cli       *kdbx.CLI
	cfg       *config.Config
	db        *config.Database
	password  []byte
	masterKey []byte
	entryKey  []byte
}

// setupEnv resolverer config, database-binding, kdbx-cli, password og deriverede
// nøgler. Argon2id-derivationen tager ~200ms — vi gør det op-front så
// pull/push/sync alle kan bruge entryKey uden lazy-logik.
func setupEnv(name string, pwStdin bool, cliPath string, timeout time.Duration, prompt string) (*runEnv, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.Server.URL == "" || cfg.Server.DeviceToken == "" {
		return nil, errors.New("not enrolled — run `keepass-deltasync enroll` first")
	}
	db := cfg.FindDatabase(name)
	if db == nil {
		return nil, fmt.Errorf("database %q not found in local config — run `keepass-deltasync init` first", name)
	}
	if _, err := os.Stat(db.LocalPath); err != nil {
		return nil, fmt.Errorf("local kdbx %s: %w", db.LocalPath, err)
	}

	cli, err := kdbx.NewCLI(cliPath)
	if err != nil {
		return nil, err
	}

	password, err := passwd.Read(prompt, pwStdin)
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(os.Stderr, "Deriving master key (Argon2id, ~200ms)...")
	masterKey, err := crypto.DeriveMasterKey(password, db.RemoteID)
	if err != nil {
		passwd.Zero(password)
		return nil, fmt.Errorf("derive master key: %w", err)
	}
	entryKey, err := crypto.DeriveEntryKey(masterKey, db.RemoteID)
	if err != nil {
		passwd.Zero(password)
		passwd.Zero(masterKey)
		return nil, fmt.Errorf("derive entry key: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return &runEnv{
		ctx:       ctx,
		cancel:    cancel,
		client:    api.New(cfg.Server.URL),
		cli:       cli,
		cfg:       cfg,
		db:        db,
		password:  password,
		masterKey: masterKey,
		entryKey:  entryKey,
	}, nil
}

// cleanup zeroer hemmeligheder og frigør context-cancel. Skal kaldes via defer
// efter setupEnv lykkes.
func (e *runEnv) cleanup() {
	if e.cancel != nil {
		e.cancel()
	}
	passwd.Zero(e.password)
	passwd.Zero(e.masterKey)
	passwd.Zero(e.entryKey)
}

// pullChanges henter alle entries fra serveren siden db.LastSeq, dekrypterer
// dem, og merger ind i den lokale .kdbx via en staging-kdbx. Returnerer det
// nye current_seq fra serveren samt antal entries og tombstones merget.
// Caller'en gemmer config selv.
func (e *runEnv) pullChanges() (newSeq int64, merged, deletionCount int, err error) {
	fmt.Fprintf(os.Stderr, "Fetching changes since seq=%d...\n", e.db.LastSeq)
	changes, err := e.client.GetChanges(e.ctx, e.cfg.Server.DeviceToken, e.db.RemoteID, e.db.LastSeq)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("GET /changes: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Server has current_seq=%d, %d new entries.\n", changes.CurrentSeq, len(changes.Entries))

	if len(changes.Entries) == 0 {
		return changes.CurrentSeq, 0, 0, nil
	}

	var entries []kdbx.StagingEntry
	var deletions []kdbx.StagingDeletion
	for _, c := range changes.Entries {
		blob, derr := base64.StdEncoding.DecodeString(c.Blob)
		if derr != nil {
			return 0, 0, 0, fmt.Errorf("entry %s: server blob not valid base64: %w", c.UUID, derr)
		}
		modAt, terr := time.Parse(time.RFC3339, c.ModifiedAt)
		if terr != nil {
			return 0, 0, 0, fmt.Errorf("entry %s: parse modified_at: %w", c.UUID, terr)
		}
		// Optag server's modified_at som "vi har set denne version" så
		// push-delta næste sync ikke re-pusher den pullede entry.
		e.db.RecordEntryState(c.UUID, modAt.UTC().Format("2006-01-02T15:04:05Z"))

		if c.Deleted {
			deletions = append(deletions, kdbx.StagingDeletion{UUID: c.UUID, DeletedAt: modAt})
			continue
		}
		fragment, derr := crypto.DecryptBlob(e.entryKey, blob)
		if derr != nil {
			return 0, 0, 0, fmt.Errorf("entry %s: decrypt failed — wrong masterpassword? %w", c.UUID, derr)
		}
		// Server's metadata modified_at er sandheden — den interne
		// <LastModificationTime> i blob'en kan være ældre (sker ved
		// restore: server kopierer en gammel blob men sætter ny mtime
		// for at vinde merge). Synk altid feltet til server's værdi så
		// KeePassXC's merge picker den rigtige version.
		fragment = rewriteLastModificationTime(fragment, modAt)
		entries = append(entries, kdbx.StagingEntry{
			UUID:       c.UUID,
			Fragment:   fragment,
			ModifiedAt: modAt,
		})
	}
	fmt.Fprintf(os.Stderr, "Decrypted %d entries (+ %d tombstones).\n", len(entries), len(deletions))

	// Find lokal Root-gruppes UUID, så staging-merge lander nye entries
	// direkte i Root i stedet for at oprette en "deltasync"-undergruppe.
	// Kræver en ekstra kdbx-export (~200ms), men kun når der faktisk er
	// noget at merge.
	localXML, err := e.cli.Export(e.ctx, e.db.LocalPath, e.password)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("export local for root uuid: %w", err)
	}
	rootUUID, err := kdbx.RootGroupUUID(localXML)
	if err != nil {
		// Defensiv: hvis vi ikke kan finde Root UUID, falder vi tilbage
		// til en random staging-gruppe (= gammel deltasync-undergruppe-
		// adfærd). Bedre noget der virker end en hård fejl.
		fmt.Fprintf(os.Stderr, "warning: could not extract root UUID (%v); falling back to deltasync subgroup\n", err)
		rootUUID = ""
	}

	stagingXML, err := kdbx.BuildStagingXML(entries, deletions, rootUUID)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("build staging xml: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "kdsync-pull-*")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("mkdir temp: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpXML := filepath.Join(tmpDir, "staging.xml")
	tmpKDBX := filepath.Join(tmpDir, "staging.kdbx")
	if err := os.WriteFile(tmpXML, stagingXML, 0o600); err != nil {
		return 0, 0, 0, fmt.Errorf("write staging xml: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Building staging kdbx...")
	if err := e.cli.Import(e.ctx, tmpXML, tmpKDBX, e.password); err != nil {
		return 0, 0, 0, err
	}

	backupPath := e.db.LocalPath + ".bak"
	if err := copyFile(e.db.LocalPath, backupPath); err != nil {
		return 0, 0, 0, fmt.Errorf("backup local kdbx: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Merging staging into local kdbx...")
	if err := e.cli.Merge(e.ctx, e.db.LocalPath, tmpKDBX, e.password); err != nil {
		if restoreErr := copyFile(backupPath, e.db.LocalPath); restoreErr != nil {
			return 0, 0, 0, fmt.Errorf("merge failed AND restore failed (backup at %s): merge=%v, restore=%w", backupPath, err, restoreErr)
		}
		return 0, 0, 0, fmt.Errorf("merge failed (local restored from backup): %w", err)
	}

	if err := os.Remove(backupPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not delete backup %s: %v\n", backupPath, err)
	}

	return changes.CurrentSeq, len(entries), len(deletions), nil
}

// pushChanges eksporterer den lokale .kdbx via keepassxc-cli, parser entries
// og deletions, og uploader dem til serveren. Filteret er pr.-entry: en entry
// pushes hvis dens mtime er nyere end den seneste i db.EntryStates (eller
// hvis den slet ikke er i map'en). Med force=true ignoreres map'en og alt
// pushes (initial-sync / recovery use case).
//
// Returnerer også maxSeq — den højeste server_seq returneret af nogen PUT
// eller DELETE i denne kørsel (0 hvis intet blev pushet). Caller'en kan
// avancere db.LastSeq forbi denne værdi, så vores egne pushes ikke pulles
// tilbage ved næste sync.
func (e *runEnv) pushChanges(force bool) (pushed, deleted int, maxSeq int64, err error) {
	fmt.Fprintln(os.Stderr, "Exporting kdbx via keepassxc-cli...")
	xmlBytes, err := e.cli.Export(e.ctx, e.db.LocalPath, e.password)
	if err != nil {
		return 0, 0, 0, err
	}

	entries, deletions, err := kdbx.ParseExport(xmlBytes)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse export: %w", err)
	}

	if force {
		fmt.Fprintf(os.Stderr, "Pushing all %d entries + %d tombstones (force).\n", len(entries), len(deletions))
	} else {
		fmt.Fprintf(os.Stderr, "Found %d entries + %d tombstones; checking per-entry tracking...\n", len(entries), len(deletions))
	}

	for _, en := range entries {
		if !force && !shouldPush(e.db.EntryStates, en.UUID, en.ModifiedAt) {
			continue
		}
		blob, eerr := crypto.EncryptBlob(e.entryKey, en.Fragment)
		if eerr != nil {
			return pushed, deleted, maxSeq, fmt.Errorf("encrypt entry %s: %w", en.UUID, eerr)
		}
		resp, perr := e.client.PutEntry(e.ctx, e.cfg.Server.DeviceToken, e.db.RemoteID, en.UUID, blob, en.ModifiedAt)
		if perr != nil {
			return pushed, deleted, maxSeq, fmt.Errorf("PUT entry %s: %w", en.UUID, perr)
		}
		if resp.Seq > maxSeq {
			maxSeq = resp.Seq
		}
		e.db.RecordEntryState(en.UUID, en.ModifiedAt.UTC().Format("2006-01-02T15:04:05Z"))
		pushed++
	}

	for _, d := range deletions {
		if !force && !shouldPush(e.db.EntryStates, d.UUID, d.DeletedAt) {
			continue
		}
		resp, derr := e.client.DeleteEntry(e.ctx, e.cfg.Server.DeviceToken, e.db.RemoteID, d.UUID, nil, d.DeletedAt)
		if derr != nil {
			return pushed, deleted, maxSeq, fmt.Errorf("DELETE entry %s: %w", d.UUID, derr)
		}
		if resp.Seq > maxSeq {
			maxSeq = resp.Seq
		}
		e.db.RecordEntryState(d.UUID, d.DeletedAt.UTC().Format("2006-01-02T15:04:05Z"))
		deleted++
	}

	return pushed, deleted, maxSeq, nil
}

// shouldPush returnerer true hvis entry'en skal pushes til serveren — dvs.
// vi har ingen recorded state, eller vores recorded mtime er strengt før
// entry'ens nuværende mtime.
func shouldPush(states map[string]string, uuid string, currentMtime time.Time) bool {
	recorded, ok := states[uuid]
	if !ok {
		return true
	}
	recordedT, err := time.Parse(time.RFC3339, recorded)
	if err != nil {
		// Korrupt state-værdi: behandl som "aldrig pushet" så vi recovers
		// ved at re-pushe entry'en og overskrive den dårlige værdi.
		return true
	}
	return recordedT.Before(currentMtime)
}

// rewriteLastModificationTime erstatter den første forekomst af
// <LastModificationTime>...</LastModificationTime> i fragmentet med en frisk
// ISO-tidsstempel. Den første forekomst er entry'ens egen Times — historiske
// versioner ligger i <History> bagefter og forbliver uændret.
func rewriteLastModificationTime(fragment []byte, t time.Time) []byte {
	loc := lastModificationTimeRe.FindIndex(fragment)
	if loc == nil {
		return fragment
	}
	iso := t.UTC().Format("2006-01-02T15:04:05Z")
	replacement := []byte("<LastModificationTime>" + iso + "</LastModificationTime>")
	out := make([]byte, 0, len(fragment)+len(replacement)-(loc[1]-loc[0]))
	out = append(out, fragment[:loc[0]]...)
	out = append(out, replacement...)
	out = append(out, fragment[loc[1]:]...)
	return out
}

// copyFile er en minimal helper til backup/restore. Ikke atomisk — men vi
// bruger den kun lige før merge, hvor atomicitet ikke er påkrævet.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
