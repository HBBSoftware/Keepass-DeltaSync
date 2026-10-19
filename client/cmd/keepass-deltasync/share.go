// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
	"gitlab.com/Star95/keepass-deltasync/client/internal/crypto"
	"gitlab.com/Star95/keepass-deltasync/client/internal/keyring"
	"gitlab.com/Star95/keepass-deltasync/client/internal/passwd"
)

// runShare implementerer "keepass-deltasync share <db-name> <username>".
//
// Alice deriverer master_key fra masterpassword (Argon2id), sluser Bob's
// device-public-key op, wrap'er master_key som sealed-box, og POST'er til
// /shares. Server lagrer den opaque sealed-box; den kan kun unwrappes af
// Bob's enheds private-key.
//
// Bob's public-key kommer fra serveren, og det er hele svagheden: en
// kompromitteret server kan levere sin egen og få master_key i klartekst.
// Derfor wrapper vi kun til en nøgle brugeren har godkendt — enten fordi den
// er den samme som sidst vi delte med Bob (cfg.KnownKeys), eller fordi
// brugeren har sammenlignet aftrykket med Bob ad en anden kanal:
//
//   - interaktivt: vi viser aftrykket og spørger.
//   - --fingerprint <fp>: aftrykket er bekræftet andetsteds (GUI'ens dialog
//     efter `share --check`). Afviger serverens nøgle nu, afbryder vi.
//   - --password-stdin uden --fingerprint: kun en kendt, uændret nøgle.
func runShare(args []string) error {
	fs := flag.NewFlagSet("share", flag.ContinueOnError)
	pwStdin := fs.Bool("password-stdin", false, "read masterpassword from stdin instead of interactive prompt")
	check := fs.Bool("check", false, "only look up the recipient's key and print its fingerprint and status; share nothing")
	expectFP := fs.String("fingerprint", "", "share only if the recipient's key has this fingerprint (as compared with them)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync share [--check | --fingerprint <fp>] [--password-stdin] <db-name> <username>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return errors.New("share takes exactly 2 arguments: <db-name> <username>")
	}
	dbName := fs.Arg(0)
	username := fs.Arg(1)

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Server.URL == "" || cfg.Server.DeviceToken == "" {
		return errors.New("not enrolled — run `keepass-deltasync enroll` first")
	}
	db := cfg.FindDatabase(dbName)
	if db == nil {
		return fmt.Errorf("database %q not found in local config — run `keepass-deltasync init` first", dbName)
	}
	if db.LocalOnly() {
		return errLocalOnly(dbName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := api.New(cfg.Server.URL)

	// Auto-upgrade keypair hvis legacy. Vi har ikke selv brug for vores
	// public_key her, men vi vil have det op på serveren før vi forsøger
	// nogen sharing-handlinger.
	if upErr := ensureDevicePublicKey(ctx, cfg, client); upErr != nil {
		fmt.Fprintf(os.Stderr, "warning: auto-upgrade of device keypair failed (%v) — continuing anyway\n", upErr)
	}

	// Verificér at vi er owner. Server håndhæver det også, men vi får en
	// pænere fejl her end "404 not found".
	role, err := fetchRole(ctx, client, cfg.Server.DeviceToken, db.RemoteID)
	if err != nil {
		return err
	}
	if role != "owner" {
		return fmt.Errorf("cannot share %q: you are %s, not owner", dbName, role)
	}

	// Slå Bob op + få hans target-device public-key.
	lookup, err := client.LookupUser(ctx, cfg.Server.DeviceToken, username)
	if err != nil {
		return fmt.Errorf("lookup user %q: %w", username, err)
	}
	targetPub, err := base64.StdEncoding.DecodeString(lookup.TargetDevice.PublicKey)
	if err != nil {
		return fmt.Errorf("decode target public key: %w", err)
	}
	if len(targetPub) != crypto.BoxPublicKeySize {
		return fmt.Errorf("target public key has unexpected size: %d", len(targetPub))
	}

	recipient := lookup.User.Username
	fingerprint := crypto.KeyFingerprint(targetPub)
	status := recipientKeyStatus(cfg.KnownKey(recipient), crypto.KeyID(targetPub))

	if *check {
		// Linjeformat som GUI'en parser (gui/cli.go shareCheck). Felterne
		// kan udvides, men eksisterende navne må ikke ændres.
		fmt.Printf("user: %s\n", recipient)
		fmt.Printf("device: %s\n", lookup.TargetDevice.Name)
		fmt.Printf("fingerprint: %s\n", fingerprint)
		fmt.Printf("status: %s\n", status)
		return nil
	}

	switch {
	case *expectFP != "":
		if !crypto.SameFingerprint(*expectFP, fingerprint) {
			return fmt.Errorf("%s's key is not the one you confirmed: the server now returns %s, you confirmed %s. "+
				"Nothing was shared. Compare the fingerprint with %s again before you retry", recipient, fingerprint, *expectFP, recipient)
		}
	case status == keyKnown:
		// Samme nøgle som sidst — intet at spørge om.
	case !*pwStdin && term.IsTerminal(int(os.Stdin.Fd())):
		printKeyNotice(recipient, lookup.TargetDevice.Name, fingerprint, status)
		if !confirmYes(fmt.Sprintf("Have you compared this fingerprint with %s, and does it match? [y/N] ", recipient)) {
			return errors.New("not shared")
		}
	default:
		return fmt.Errorf("%s's key needs your confirmation (%s, fingerprint %s). Compare it with %s, "+
			"who can see it in their DeltaSync app or with `keepass-deltasync fingerprint`, "+
			"then run again with --fingerprint %s", recipient, status, fingerprint, recipient, fingerprint)
	}

	// Deriver master_key fra masterpassword. Argon2id ~200ms.
	// Tjek OS-keyring først (samme convention som daemon-mode), fall back til prompt.
	password, kerr := keyring.Get(db.RemoteID)
	if kerr != nil {
		password, err = passwd.Read(fmt.Sprintf("Masterpassword for %s: ", dbName), *pwStdin)
		if err != nil {
			return err
		}
	} else {
		fmt.Fprintf(os.Stderr, "[%s] masterpassword loaded from OS keyring.\n", dbName)
	}
	defer passwd.Zero(password)

	fmt.Fprintln(os.Stderr, "Deriving master key (Argon2id, ~200ms)...")
	masterKey, err := crypto.DeriveMasterKey(password, db.RemoteID)
	if err != nil {
		return fmt.Errorf("derive master key: %w", err)
	}
	defer passwd.Zero(masterKey)

	wrapped, err := crypto.WrapKey(masterKey, targetPub)
	if err != nil {
		return fmt.Errorf("wrap master key: %w", err)
	}

	if err := client.ShareDatabase(ctx, cfg.Server.DeviceToken, db.RemoteID, lookup.User.ID, wrapped); err != nil {
		return fmt.Errorf("share database: %w", err)
	}

	// Først nu er nøglen godkendt og brugt; næste deling med samme nøgle
	// spørger ikke igen. En fejl her ændrer ikke at delingen lykkedes.
	cfg.RememberKey(recipient, crypto.KeyID(targetPub))
	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not remember %s's key (%v); you will be asked again next time\n", recipient, err)
	}

	fmt.Printf("Shared %q with %s (device: %s).\n", dbName, lookup.User.Username, lookup.TargetDevice.Name)
	fmt.Println("They can now run `databases` to see the shared database, then `init-shared` (coming in M4) to bootstrap a local copy.")
	return nil
}

// fetchRole returnerer caller'ens rolle ('owner' / 'member') for en given
// database ved at slå op i GET /databases. Hvis databasen ikke er listet,
// returneres en klar fejl. Bruges af share/unshare/shares-kommandoerne til
// klient-side rolle-tjek før det dyre Argon2id-arbejde sættes i gang.
func fetchRole(ctx context.Context, client *api.Client, deviceToken, databaseID string) (string, error) {
	dbs, err := client.ListDatabases(ctx, deviceToken)
	if err != nil {
		return "", fmt.Errorf("list databases: %w", err)
	}
	for _, d := range dbs {
		if d.ID == databaseID {
			if d.Role == "" {
				// Pre-v2 server returnerer ingen role. Antag owner.
				return "owner", nil
			}
			return d.Role, nil
		}
	}
	return "", fmt.Errorf("database %s not found on server (deleted, or you lost access)", databaseID)
}

// Status for en modtagers nøgle set fra denne maskine. Strengene er en del af
// `share --check`'s output og læses af GUI'en.
const (
	keyNew     = "new"     // aldrig delt med modtageren herfra
	keyKnown   = "known"   // samme nøgle som sidst
	keyChanged = "changed" // en anden nøgle end sidst
)

func recipientKeyStatus(known, current string) string {
	switch {
	case known == "":
		return keyNew
	case known == current:
		return keyKnown
	default:
		return keyChanged
	}
}

func printKeyNotice(recipient, device, fingerprint, status string) {
	fmt.Fprintln(os.Stderr)
	if status == keyChanged {
		fmt.Fprintf(os.Stderr, "WARNING: %s's key has changed since you last shared with them.\n", recipient)
		fmt.Fprintf(os.Stderr, "That is expected if %s has set up a new device. Otherwise the server may be\n", recipient)
		fmt.Fprintln(os.Stderr, "handing you a key that is not theirs, and sharing would give away the database.")
	} else {
		fmt.Fprintf(os.Stderr, "You have not shared with %s from this machine before.\n", recipient)
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  Device:       %s\n", device)
	fmt.Fprintf(os.Stderr, "  Fingerprint:  %s\n", fingerprint)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Ask %s to read out the fingerprint of that device (in their DeltaSync app, or\n", recipient)
	fmt.Fprintln(os.Stderr, "`keepass-deltasync fingerprint`), by phone or in person. Share only if it matches.")
	fmt.Fprintln(os.Stderr)
}

// confirmYes stiller et ja/nej-spørgsmål på terminalen. Alt andet end y/yes
// (også en tom linje eller EOF) er nej.
func confirmYes(prompt string) bool {
	fmt.Fprint(os.Stderr, prompt)
	var answer string
	if _, err := fmt.Fscanln(os.Stdin, &answer); err != nil {
		return false
	}
	a := strings.ToLower(strings.TrimSpace(answer))
	return a == "y" || a == "yes"
}
