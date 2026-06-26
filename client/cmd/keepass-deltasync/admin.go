// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"gitlab.com/Star95/keepass-deltasync/client/internal/api"
	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

// adminTokenEnvVar er env-var-navnet hvor admin-tokenet kan lagres så det
// ikke skal angives som flag for hvert kald. Værdien overrides altid af
// et eksplicit --admin-token flag.
const adminTokenEnvVar = "KEEPASS_DELTASYNC_ADMIN_TOKEN"

// adminUsage er help-teksten for `admin`-subkommandoen.
const adminUsage = `keepass-deltasync admin — administrative kommandoer.

Usage:
  keepass-deltasync admin <subcommand> [arguments]

Subcommands:
  token-sql                              Print SQL til DBeaver for at oprette en frisk admin-token
  user-create <username> [opts]          Opret bruger + returnér enrollment-token
  user-list                              Tabular liste over alle brugere
  user-enrollment <username>             Ny enrollment-token til eksisterende bruger
  user-disable <username>                Sæt disabled=true (auth blokeres, data bevares)
  user-enable <username>                 Sæt disabled=false
  user-delete <username>                 Slet bruger permanent (CASCADE)

Admin-token sourcing (alle subkommandoer på nær token-sql):
  --admin-token <token>          Overrider env-var
  $KEEPASS_DELTASYNC_ADMIN_TOKEN Default-kilde

Server-URL (user-create / user-enrollment):
  --server <url>                 Påkrævet på en maskine der ikke er enrolled
                                 endnu; ellers læses URL'en fra config.toml.

Hvis du mangler en admin-token: kør 'keepass-deltasync admin token-sql' og
paste den SQL i DBeaver.
`

func runAdmin(args []string) error {
	if len(args) == 0 {
		fmt.Print(adminUsage)
		return nil
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "token-sql":
		return runAdminTokenSQL(rest)
	case "user-create":
		return runAdminUserCreate(rest)
	case "user-list":
		return runAdminUserList(rest)
	case "user-enrollment":
		return runAdminUserEnrollment(rest)
	case "user-disable":
		return runAdminSetDisabled(rest, true)
	case "user-enable":
		return runAdminSetDisabled(rest, false)
	case "user-delete":
		return runAdminUserDelete(rest)
	case "-h", "--help", "help":
		fmt.Print(adminUsage)
		return nil
	default:
		fmt.Fprintf(os.Stderr, "unknown admin subcommand: %s\n\n%s", sub, adminUsage)
		os.Exit(2)
	}
	return nil
}

// rearrangeFlagsFirst flytter "-flag value" og "-flag=value" til front af args
// så Go's stdlib flag-pakke (der stopper ved første positional arg) kan parse
// dem uanset rækkefølge i brugerens kommandolinje. boolFlags angiver hvilke
// flag-navne IKKE tager en separat værdi-arg ("-yes" alene, ingen næste arg).
func rearrangeFlagsFirst(args []string, boolFlags map[string]bool) []string {
	flags := make([]string, 0, len(args))
	positional := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			// alt efter "--" er positional per POSIX-konvention
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || len(a) == 1 {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		if strings.Contains(name, "=") {
			continue
		}
		if boolFlags[strings.SplitN(name, "=", 2)[0]] {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

// resolveAdminToken finder admin-token fra (1) flag-værdi, (2) env-var.
// Returnerer fejl hvis ingen er sat — caller behøver ikke selv validere.
func resolveAdminToken(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if env := strings.TrimSpace(os.Getenv(adminTokenEnvVar)); env != "" {
		return env, nil
	}
	return "", fmt.Errorf("admin token required — pass --admin-token <token> or set $%s", adminTokenEnvVar)
}

// resolveServerURL henter server-URL fra config. Bruges af admin-subkommandoer
// så vi ikke kræver --server flag når brugeren allerede er enrolled.
func resolveServerURL() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	if cfg.Server.URL == "" {
		return "", errors.New("server URL ikke konfigureret — kør `enroll --server <url>` først, eller pass --server")
	}
	return cfg.Server.URL, nil
}

// resolveServerURLWith bruger en eksplicit --server flag-værdi hvis sat, ellers
// falder den tilbage til config (resolveServerURL). Det lader admin-kommandoer
// køre på en frisk maskine uden config.toml — fx GUI'ens avancerede tilmelding,
// der udsteder et enrollment-token og enroller PC'en i samme arbejdsgang.
func resolveServerURLWith(flagValue string) (string, error) {
	if strings.TrimSpace(flagValue) != "" {
		return flagValue, nil
	}
	return resolveServerURL()
}

// runAdminTokenSQL printer SQL der genererer en frisk admin-token via
// Postgres' pgcrypto. Klienten har ikke direkte DB-adgang, så vi kan ikke
// selv køre SQL'en — brugeren paste-kører i DBeaver.
func runAdminTokenSQL(args []string) error {
	fs := flag.NewFlagSet("admin token-sql", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync admin token-sql")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	fmt.Println(`-- Paste det følgende i DBeaver's SQL-editor og kør (F5 / Ctrl+Enter).
-- Klartekst-tokenet vises som SELECT-resultat ÉN gang — gem det med det
-- samme. Kun SHA-256-hashen lagres i DB.
--
-- Kræver pgcrypto-extension (aktiveret i migration 001).

WITH new_token AS (
    SELECT rtrim(
               translate(
                   encode(gen_random_bytes(32), 'base64'),
                   '+/',
                   '-_'
               ),
               '='
           ) AS token
),
inserted AS (
    INSERT INTO admin_tokens (token_hash)
    SELECT encode(digest(token, 'sha256'), 'hex')
      FROM new_token
    RETURNING token_hash, created_at
)
SELECT
    nt.token        AS admin_token,
    i.token_hash    AS stored_hash,
    i.created_at    AS created_at
  FROM new_token nt,
       inserted  i;

-- Når du har tokenet:
--   export KEEPASS_DELTASYNC_ADMIN_TOKEN=<token>    # bash
--   $env:KEEPASS_DELTASYNC_ADMIN_TOKEN = "<token>"  # PowerShell
-- Derefter kan du bruge admin-subkommandoerne uden --admin-token flag.`)
	return nil
}

func runAdminUserCreate(args []string) error {
	fs := flag.NewFlagSet("admin user-create", flag.ContinueOnError)
	adminToken := fs.String("admin-token", "", "admin-token (alternativt via $"+adminTokenEnvVar+")")
	displayName := fs.String("display-name", "", "valgfri display-name til brugeren")
	serverFlag := fs.String("server", "", "server URL (default: fra config.toml — påkrævet på en maskine der ikke er enrolled endnu)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync admin user-create <username> [--display-name NAME] [--server URL] [--admin-token TOKEN]")
		fs.PrintDefaults()
	}
	args = rearrangeFlagsFirst(args, nil)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("user-create takes exactly 1 argument: <username>")
	}
	username := fs.Arg(0)

	token, err := resolveAdminToken(*adminToken)
	if err != nil {
		return err
	}
	serverURL, err := resolveServerURLWith(*serverFlag)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := api.New(serverURL)
	resp, err := client.AdminCreateUser(ctx, token, username, *displayName)
	if err != nil {
		return err
	}

	fmt.Printf("User '%s' created.\n", resp.User.Username)
	fmt.Printf("  User ID:           %s\n", resp.User.ID)
	if resp.User.DisplayName != nil && *resp.User.DisplayName != "" {
		fmt.Printf("  Display name:      %s\n", *resp.User.DisplayName)
	}
	fmt.Printf("  Created at:        %s\n", resp.User.CreatedAt)
	fmt.Printf("\nEnrollment-token (expires %s):\n", resp.ExpiresAt)
	fmt.Printf("  %s\n", resp.EnrollmentToken)
	fmt.Printf("\nGive this to %s and let them run:\n", username)
	fmt.Printf("  keepass-deltasync enroll --server %s %s\n", serverURL, resp.EnrollmentToken)
	return nil
}

func runAdminUserList(args []string) error {
	fs := flag.NewFlagSet("admin user-list", flag.ContinueOnError)
	adminToken := fs.String("admin-token", "", "admin-token (alternativt via $"+adminTokenEnvVar+")")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync admin user-list [--admin-token TOKEN]")
		fs.PrintDefaults()
	}
	args = rearrangeFlagsFirst(args, nil)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	token, err := resolveAdminToken(*adminToken)
	if err != nil {
		return err
	}
	serverURL, err := resolveServerURL()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	users, err := api.New(serverURL).AdminListUsers(ctx, token)
	if err != nil {
		return err
	}

	if len(users) == 0 {
		fmt.Println("(no users)")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "USERNAME\tDISPLAY\tSTATUS\tDEVICES\tDATABASES\tCREATED")
	for _, u := range users {
		display := "-"
		if u.DisplayName != nil && *u.DisplayName != "" {
			display = *u.DisplayName
		}
		status := "active"
		if u.Disabled {
			status = "disabled"
		}
		created := u.CreatedAt
		if len(created) >= 10 {
			created = created[:10]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\n",
			u.Username, display, status, u.DeviceCount, u.DatabaseCount, created)
	}
	return w.Flush()
}

func runAdminUserEnrollment(args []string) error {
	fs := flag.NewFlagSet("admin user-enrollment", flag.ContinueOnError)
	adminToken := fs.String("admin-token", "", "admin-token (alternativt via $"+adminTokenEnvVar+")")
	serverFlag := fs.String("server", "", "server URL (default: fra config.toml — påkrævet på en maskine der ikke er enrolled endnu)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync admin user-enrollment <username> [--server URL] [--admin-token TOKEN]")
		fs.PrintDefaults()
	}
	args = rearrangeFlagsFirst(args, nil)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("user-enrollment takes exactly 1 argument: <username>")
	}
	username := fs.Arg(0)

	token, err := resolveAdminToken(*adminToken)
	if err != nil {
		return err
	}
	serverURL, err := resolveServerURLWith(*serverFlag)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := api.New(serverURL)

	userID, err := lookupUserIDByUsername(ctx, client, token, username)
	if err != nil {
		return err
	}

	enrollToken, expiresAt, err := client.AdminCreateEnrollmentToken(ctx, token, userID)
	if err != nil {
		return err
	}

	fmt.Printf("New enrollment-token for '%s' (expires %s):\n\n", username, expiresAt)
	fmt.Printf("  %s\n\n", enrollToken)
	fmt.Printf("Tell %s to run:\n  keepass-deltasync enroll --server %s %s\n",
		username, serverURL, enrollToken)
	return nil
}

func runAdminSetDisabled(args []string, disabled bool) error {
	verb := "disable"
	if !disabled {
		verb = "enable"
	}
	fs := flag.NewFlagSet("admin user-"+verb, flag.ContinueOnError)
	adminToken := fs.String("admin-token", "", "admin-token (alternativt via $"+adminTokenEnvVar+")")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: keepass-deltasync admin user-%s <username> [--admin-token TOKEN]\n", verb)
		fs.PrintDefaults()
	}
	args = rearrangeFlagsFirst(args, nil)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("user-%s takes exactly 1 argument: <username>", verb)
	}
	username := fs.Arg(0)

	token, err := resolveAdminToken(*adminToken)
	if err != nil {
		return err
	}
	serverURL, err := resolveServerURL()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := api.New(serverURL)

	userID, err := lookupUserIDByUsername(ctx, client, token, username)
	if err != nil {
		return err
	}

	if err := client.AdminSetUserDisabled(ctx, token, userID, disabled); err != nil {
		return err
	}

	past := "disabled"
	if !disabled {
		past = "enabled"
	}
	fmt.Printf("User '%s' %s.\n", username, past)
	return nil
}

func runAdminUserDelete(args []string) error {
	fs := flag.NewFlagSet("admin user-delete", flag.ContinueOnError)
	adminToken := fs.String("admin-token", "", "admin-token (alternativt via $"+adminTokenEnvVar+")")
	yes := fs.Bool("yes", false, "skip confirmation prompt")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: keepass-deltasync admin user-delete <username> [--yes] [--admin-token TOKEN]")
		fs.PrintDefaults()
	}
	args = rearrangeFlagsFirst(args, map[string]bool{"yes": true})
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("user-delete takes exactly 1 argument: <username>")
	}
	username := fs.Arg(0)

	token, err := resolveAdminToken(*adminToken)
	if err != nil {
		return err
	}
	serverURL, err := resolveServerURL()
	if err != nil {
		return err
	}

	if !*yes {
		fmt.Fprintf(os.Stderr, "About to PERMANENTLY delete user '%s' (CASCADE: all devices, databases, entries). Type 'yes' to confirm: ", username)
		var input string
		fmt.Scanln(&input)
		if strings.TrimSpace(input) != "yes" {
			return errors.New("aborted")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := api.New(serverURL)

	userID, err := lookupUserIDByUsername(ctx, client, token, username)
	if err != nil {
		return err
	}

	if err := client.AdminDeleteUser(ctx, token, userID); err != nil {
		return err
	}

	fmt.Printf("User '%s' deleted. CASCADE removed devices, databases and entries.\n", username)
	return nil
}

// lookupUserIDByUsername er en helper: admin-API'en accepterer kun user_id
// (UUID) som path-parameter for PATCH/DELETE/enrollment, men UX'en bruger
// username. Vi slår op via AdminListUsers og finder den matchende række.
func lookupUserIDByUsername(ctx context.Context, client *api.Client, adminToken, username string) (string, error) {
	users, err := client.AdminListUsers(ctx, adminToken)
	if err != nil {
		return "", fmt.Errorf("list users: %w", err)
	}
	for _, u := range users {
		if u.Username == username {
			return u.ID, nil
		}
	}
	return "", fmt.Errorf("user %q not found", username)
}
