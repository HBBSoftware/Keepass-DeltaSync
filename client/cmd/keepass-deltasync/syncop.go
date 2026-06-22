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
	"gitlab.com/Star95/keepass-deltasync/client/internal/kdbx/canonical"
	"gitlab.com/Star95/keepass-deltasync/client/internal/passwd"
)

// lastModificationTimeRe fanger entry'ens egen <LastModificationTime>. Vi
// erstatter altid den første forekomst — der ligger inde i <Times>-blokken
// FØR <History>, så efterfølgende historiske forekomster bevares uændret.
var lastModificationTimeRe = regexp.MustCompile(`<LastModificationTime>[^<]*</LastModificationTime>`)

// runEnv samler den shared state alle synkroniserings-kommandoer (pull/push/
// sync) skal bruge: API-klient, kdbx-CLI, config, masterpassword og deriverede
// nøgler. Bygges via setupEnv (one-shot CLI-kommandoer) eller
// newRunEnvBorrowed (daemon-loop, hvor keys ejes af caller).
//
// ownsKeys styrer om cleanup() zeroer password/masterKey/entryKey. One-shot-
// commands ejer keys; daemon låner dem fra sit ydre setup og må ikke zeroe
// dem mellem sync-cycles.
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
	ownsKeys  bool
	// quiet undertrykker per-cycle progress-prints ("Fetching...",
	// "Exporting...", "Found N entries...") til stderr. Warnings og errors
	// printes uanset. Daemon-loopet sætter quiet=true så hvert poll-tick
	// ikke spammer logs på no-op-syncs.
	quiet bool
}

// progressf skriver per-cycle progress til stderr, men no-op'er hvis quiet
// er sat. Warnings og errors må ikke bruge denne — de skal printes uanset.
func (e *runEnv) progressf(format string, args ...any) {
	if e.quiet {
		return
	}
	fmt.Fprintf(os.Stderr, format, args...)
}

// setupEnv resolverer config, database-binding, kdbx-cli, password og deriverede
// nøgler. Argon2id-derivationen tager ~200ms — vi gør det op-front så
// pull/push/sync alle kan bruge entryKey uden lazy-logik.
func setupEnv(name string, pwStdin bool, cliPath string, timeout time.Duration, prompt string) (*runEnv, error) {
	cfg, db, cli, err := loadDBAndCLI(name, cliPath)
	if err != nil {
		return nil, err
	}
	client := api.New(cfg.Server.URL)

	// Auto-upgrade legacy enheder (enrolled før v2) med X25519 keypair før
	// vi prompter for password. På den måde får brugeren ikke skrevet sit
	// masterpassword forgæves hvis netværket er nede.
	upgradeCtx, upgradeCancel := context.WithTimeout(context.Background(), 30*time.Second)
	upErr := ensureDevicePublicKey(upgradeCtx, cfg, client)
	upgradeCancel()
	if upErr != nil {
		fmt.Fprintf(os.Stderr, "warning: auto-upgrade of device keypair failed (%v) — sharing features will not work until fixed\n", upErr)
	}

	// Slå role + wrapped_master_key op for denne database. For members
	// bruges wrapped_master_key til at unwrappe master_key i stedet for at
	// derivere fra password. Vi gør det FØR password-prompt så netværksfejl
	// ikke spilder brugerens tastearbejde.
	roleCtx, roleCancel := context.WithTimeout(context.Background(), 30*time.Second)
	serverDB, err := findServerDatabase(roleCtx, client, cfg.Server.DeviceToken, db.RemoteID)
	roleCancel()
	if err != nil {
		return nil, fmt.Errorf("lookup db role: %w", err)
	}

	password, err := passwd.Read(prompt, pwStdin)
	if err != nil {
		return nil, err
	}

	masterKey, entryKey, err := resolveMasterEntryKeys(password, serverDB.Role, db.RemoteID, serverDB.WrappedMasterKey, cfg.Server.DevicePrivateKey)
	if err != nil {
		passwd.Zero(password)
		return nil, err
	}

	env := newRunEnvBorrowed(context.Background(), cfg, db, cli, password, masterKey, entryKey, timeout)
	env.ownsKeys = true
	return env, nil
}

// findServerDatabase finder en database i serverens database-liste på remote_id.
// Bruges af setupEnv (og daemon) til at hente role + wrapped_master_key for en
// kendt lokal binding. Hvis serveren ikke har den (slettet, eller share blev
// trukket tilbage), returneres en klar fejl.
func findServerDatabase(ctx context.Context, client *api.Client, deviceToken, remoteID string) (*api.Database, error) {
	dbs, err := client.ListDatabases(ctx, deviceToken)
	if err != nil {
		return nil, fmt.Errorf("list databases: %w", err)
	}
	for i := range dbs {
		if dbs[i].ID == remoteID {
			return &dbs[i], nil
		}
	}
	return nil, fmt.Errorf("database %s not found on server (deleted, or you lost access)", remoteID)
}

// resolveMasterEntryKeys returnerer entry-kryptering-keys for en database
// baseret pa rolle. For owners: Argon2id(password). For members: unwrap
// wrapped_master_key med device-keypair. Caller ejer det returnerede
// keymateriale og skal zero det.
//
// password er det LOKALE kdbx-password (til keepassxc-cli). For owners er
// det også masterpassword'et til Argon2id-derivation; for members er det
// uafhængigt — Bob har valgt sit eget password til sin lokale kopi.
func resolveMasterEntryKeys(password []byte, role, remoteID string, wrappedKeyB64 *string, devicePriv []byte) (masterKey, entryKey []byte, err error) {
	if role == "member" {
		if wrappedKeyB64 == nil {
			return nil, nil, errors.New("server reports member role but wrapped_master_key is missing — owner must re-share")
		}
		if len(devicePriv) == 0 {
			return nil, nil, errors.New("device private key not in config — re-enroll or run any other command first to auto-upgrade")
		}
		wrapped, err := base64.StdEncoding.DecodeString(*wrappedKeyB64)
		if err != nil {
			return nil, nil, fmt.Errorf("decode wrapped key: %w", err)
		}
		pub, err := crypto.PublicKeyFromPrivate(devicePriv)
		if err != nil {
			return nil, nil, fmt.Errorf("derive device public key: %w", err)
		}
		masterKey, err = crypto.UnwrapKey(wrapped, pub, devicePriv)
		if err != nil {
			return nil, nil, fmt.Errorf("unwrap master key: %w", err)
		}
		entryKey, err = crypto.DeriveEntryKey(masterKey, remoteID)
		if err != nil {
			passwd.Zero(masterKey)
			return nil, nil, fmt.Errorf("derive entry key: %w", err)
		}
		return masterKey, entryKey, nil
	}
	// owner (eller pre-v2 server der ikke har role).
	return deriveKeys(password, remoteID)
}

// loadDBAndCLI udfører den ikke-hemmelighedsbærende del af setup: config-load,
// db-lookup, path-check og kdbx-cli-detection. Genbruges af daemon, som har
// behov for at validere disse trin før den prompter masterpassword.
func loadDBAndCLI(name, cliPath string) (*config.Config, *config.Database, *kdbx.CLI, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, err
	}
	if cfg.Server.URL == "" || cfg.Server.DeviceToken == "" {
		return nil, nil, nil, errors.New("not enrolled — run `keepass-deltasync enroll` first")
	}
	db := cfg.FindDatabase(name)
	if db == nil {
		return nil, nil, nil, fmt.Errorf("database %q not found in local config — run `keepass-deltasync init` first", name)
	}
	if _, err := os.Stat(db.LocalPath); err != nil {
		return nil, nil, nil, fmt.Errorf("local kdbx %s: %w", db.LocalPath, err)
	}
	cli, err := kdbx.NewCLI(cliPath)
	if err != nil {
		return nil, nil, nil, err
	}
	return cfg, db, cli, nil
}

// deriveKeys kører Argon2id master-key derivation + HKDF entry-key derivation.
// Caller ejer både password og de returnerede keys og skal zeroe dem.
func deriveKeys(password []byte, remoteID string) (masterKey, entryKey []byte, err error) {
	fmt.Fprintln(os.Stderr, "Deriving master key (Argon2id, ~200ms)...")
	masterKey, err = crypto.DeriveMasterKey(password, remoteID)
	if err != nil {
		return nil, nil, fmt.Errorf("derive master key: %w", err)
	}
	entryKey, err = crypto.DeriveEntryKey(masterKey, remoteID)
	if err != nil {
		passwd.Zero(masterKey)
		return nil, nil, fmt.Errorf("derive entry key: %w", err)
	}
	return masterKey, entryKey, nil
}

// newRunEnvBorrowed bygger en runEnv med eksterne keys (caller ejer). Bruges
// af daemon-loopet, der prompter password én gang og genbruger keys på tværs
// af mange sync-cycles. cleanup() vil ikke zero keys så længe ownsKeys er
// false (default). Parent-context lader daemon afbryde igangværende
// network/cli-calls ved shutdown.
func newRunEnvBorrowed(parent context.Context, cfg *config.Config, db *config.Database, cli *kdbx.CLI, password, masterKey, entryKey []byte, timeout time.Duration) *runEnv {
	ctx, cancel := context.WithTimeout(parent, timeout)
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
	}
}

// cleanup frigør context-cancel og — hvis runEnv'en ejer sine keys — zeroer
// password/masterKey/entryKey. Daemon-cycles sætter ownsKeys=false så cleanup
// kun annullerer context.
func (e *runEnv) cleanup() {
	if e.cancel != nil {
		e.cancel()
	}
	if e.ownsKeys {
		passwd.Zero(e.password)
		passwd.Zero(e.masterKey)
		passwd.Zero(e.entryKey)
	}
}

// pullChanges henter alle entries fra serveren siden db.LastSeq, dekrypterer
// dem, og merger ind i den lokale .kdbx via en staging-kdbx. Returnerer det
// nye current_seq fra serveren samt antal entries og tombstones merget.
// Caller'en gemmer config selv.
func (e *runEnv) pullChanges() (newSeq int64, merged, deletionCount int, err error) {
	e.progressf("Fetching changes since seq=%d...\n", e.db.LastSeq)
	changes, err := e.client.GetChanges(e.ctx, e.cfg.Server.DeviceToken, e.db.RemoteID, e.db.LastSeq, false)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("GET /changes: %w", err)
	}
	e.progressf("Server has current_seq=%d, %d new entries.\n", changes.CurrentSeq, len(changes.Entries))

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
		fragment, derr := decryptToFragment(e.entryKey, blob, c.UUID, modAt)
		if derr != nil {
			return 0, 0, 0, derr
		}
		entries = append(entries, kdbx.StagingEntry{
			UUID:       c.UUID,
			Fragment:   fragment,
			ModifiedAt: modAt,
		})
	}
	e.progressf("Decrypted %d entries (+ %d tombstones).\n", len(entries), len(deletions))

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

	e.progressf("Building staging kdbx...\n")
	if err := e.cli.Import(e.ctx, tmpXML, tmpKDBX, e.password); err != nil {
		return 0, 0, 0, err
	}

	backupPath := e.db.LocalPath + ".bak"
	if err := copyFile(e.db.LocalPath, backupPath); err != nil {
		return 0, 0, 0, fmt.Errorf("backup local kdbx: %w", err)
	}

	e.progressf("Merging staging into local kdbx...\n")
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
	e.progressf("Exporting kdbx via keepassxc-cli...\n")
	xmlBytes, err := e.cli.Export(e.ctx, e.db.LocalPath, e.password)
	if err != nil {
		return 0, 0, 0, err
	}

	entries, groups, deletions, err := kdbx.ParseExport(xmlBytes)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse export: %w", err)
	}

	if force {
		e.progressf("Pushing all %d groups + %d entries + %d tombstones (force).\n", len(groups), len(entries), len(deletions))
	} else {
		e.progressf("Found %d groups + %d entries + %d tombstones; checking per-object tracking...\n", len(groups), len(entries), len(deletions))
	}

	// Grupper først, så parent-grupper findes på serveren før entries der
	// peger på dem (selv om desktop-pull rebuilder hele træet på én gang).
	for _, g := range groups {
		if !force && !shouldPush(e.db.EntryStates, g.UUID, g.ModifiedAt) {
			continue
		}
		blob, gerr := encodeGroupToBlob(e.entryKey, g)
		if gerr != nil {
			return pushed, deleted, maxSeq, gerr
		}
		resp, perr := e.client.PutGroup(e.ctx, e.cfg.Server.DeviceToken, e.db.RemoteID, g.UUID, blob, g.ModifiedAt)
		if perr != nil {
			return pushed, deleted, maxSeq, fmt.Errorf("PUT group %s: %w", g.UUID, perr)
		}
		if resp.Seq > maxSeq {
			maxSeq = resp.Seq
		}
		e.db.RecordEntryState(g.UUID, g.ModifiedAt.UTC().Format("2006-01-02T15:04:05Z"))
		pushed++
	}

	for _, en := range entries {
		// Flyt-detektion: en flytning mellem grupper bumper LocationChanged,
		// ikke nødvendigvis ModifiedAt — brug den seneste som push-trigger.
		trigger := laterTime(en.ModifiedAt, en.LocationChanged)
		if !force && !shouldPush(e.db.EntryStates, en.UUID, trigger) {
			continue
		}
		blob, eerr := encodeFragmentToBlob(e.entryKey, en.UUID, en.ParentGroupUUID, en.Fragment)
		if eerr != nil {
			return pushed, deleted, maxSeq, eerr
		}
		resp, perr := e.client.PutEntry(e.ctx, e.cfg.Server.DeviceToken, e.db.RemoteID, en.UUID, blob, en.ModifiedAt)
		if perr != nil {
			return pushed, deleted, maxSeq, fmt.Errorf("PUT entry %s: %w", en.UUID, perr)
		}
		if resp.Seq > maxSeq {
			maxSeq = resp.Seq
		}
		e.db.RecordEntryState(en.UUID, trigger.UTC().Format("2006-01-02T15:04:05Z"))
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

// encodeFragmentToBlob konverterer et keepassxc-cli InnerXML-fragment til en
// krypteret canonical-blob klar til upload (v3 wire-format). Parser fragmentet
// til canonical.Entry, marshaller til JSON med format-byte-prefix, og krypterer
// resultatet. Fejlmeddelelser inkluderer entry-UUID for at gøre push-fejl
// debugbare.
func encodeFragmentToBlob(entryKey []byte, uuid, parentGroup string, fragment []byte) ([]byte, error) {
	ce, err := canonical.FromInnerXML(fragment)
	if err != nil {
		return nil, fmt.Errorf("entry %s: parse fragment: %w", uuid, err)
	}
	// v4: bær entry'ens gruppe-placering med i blob'en (tom = Root-sentinel).
	ce.ParentGroup = parentGroup
	plaintext, err := canonical.EncodeCanonical(ce)
	if err != nil {
		return nil, fmt.Errorf("entry %s: encode canonical: %w", uuid, err)
	}
	blob, err := crypto.EncryptBlob(entryKey, plaintext)
	if err != nil {
		return nil, fmt.Errorf("entry %s: encrypt: %w", uuid, err)
	}
	return blob, nil
}

// encodeGroupToBlob konverterer en kdbx.Group til en krypteret canonical
// group-blob (envelope 0x02) klar til PUT /groups (v4 group-sync).
func encodeGroupToBlob(entryKey []byte, g kdbx.Group) ([]byte, error) {
	cg := &canonical.Group{
		UUID:        g.UUID,
		Name:        g.Name,
		Notes:       g.Notes,
		ParentGroup: g.ParentUUID,
		IconID:      g.IconID,
		Times: canonical.Times{
			Created:         g.CreatedAt,
			Modified:        g.ModifiedAt,
			LocationChanged: g.LocationChanged,
		},
	}
	plaintext, err := canonical.EncodeGroup(cg)
	if err != nil {
		return nil, fmt.Errorf("group %s: encode canonical: %w", g.UUID, err)
	}
	blob, err := crypto.EncryptBlob(entryKey, plaintext)
	if err != nil {
		return nil, fmt.Errorf("group %s: encrypt: %w", g.UUID, err)
	}
	return blob, nil
}

// laterTime returnerer det seneste af to tidsstempler. Bruges til push-delta:
// en entry-flytning bumper LocationChanged, ikke ModifiedAt, så vi skal pushe
// hvis nogen af dem er nyere end sidst sete.
func laterTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// decryptToFragment dekrypterer en server-blob og returnerer InnerXML-fragmentet
// klar til staging. Håndterer dual-read mellem legacy XML (v1, byte 0 == '<') og
// canonical (v3, byte 0 == 0x01) under migrations-perioden.
//
// modAt er server's mtime for denne entry-version — vi forcerer altid entry'ens
// interne LastModificationTime til denne værdi så keepassxc-cli's merge picker
// den rigtige version (server's mtime stiger ved restore selv om indholdet er
// gammelt; uden override ville merge afvise det).
func decryptToFragment(entryKey, blob []byte, uuid string, modAt time.Time) ([]byte, error) {
	plaintext, err := crypto.DecryptBlob(entryKey, blob)
	if err != nil {
		return nil, fmt.Errorf("entry %s: decrypt failed — wrong masterpassword? %w", uuid, err)
	}

	switch canonical.DetectFormat(plaintext) {
	case canonical.FormatCanonical:
		ce, err := canonical.DecodeCanonical(plaintext)
		if err != nil {
			return nil, fmt.Errorf("entry %s: decode canonical: %w", uuid, err)
		}
		ce.Times.Modified = modAt
		fragment, err := canonical.ToInnerXML(ce)
		if err != nil {
			return nil, fmt.Errorf("entry %s: emit innerxml: %w", uuid, err)
		}
		return fragment, nil

	case canonical.FormatLegacyXML:
		return rewriteLastModificationTime(plaintext, modAt), nil

	default:
		if len(plaintext) == 0 {
			return nil, fmt.Errorf("entry %s: empty plaintext after decrypt", uuid)
		}
		return nil, fmt.Errorf("entry %s: unrecognized blob format byte 0x%02x", uuid, plaintext[0])
	}
}

// rewriteLastModificationTime erstatter den første forekomst af
// <LastModificationTime>...</LastModificationTime> i fragmentet med en frisk
// ISO-tidsstempel. Den første forekomst er entry'ens egen Times — historiske
// versioner ligger i <History> bagefter og forbliver uændret.
//
// Bruges kun på legacy-XML-stien i decryptToFragment; canonical-pull-stien
// sætter Modified direkte på Entry.Times før ToInnerXML. Funktionen kan
// fjernes når legacy-blobs er udfaset (E1 i v3-canonical-entry-format.md).
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
