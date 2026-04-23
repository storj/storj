// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/satellitedb"
)

const omLicenseType = "OM"

var mon = monkit.Package()

var (
	rootCmd = &cobra.Command{
		Use:   "om-license",
		Short: "Tool for managing OM account license entitlements",
	}

	grantCmd = &cobra.Command{
		Use:   "grant",
		Short: "Grant every active user an active OM account license",
		Long:  "Iterates over all active users and, for each user without an active OM license entitlement, adds one with the provided expiration. PublicID and bucket remain unset.",
		RunE:  run,
	}

	config Config
)

// Config holds the command-line configuration.
type Config struct {
	SatelliteDB  string
	ExpiresAt    string
	SkipConfirm  bool
	Verbose      bool
	BatchSize    int
	EmailPattern string
	DryRun       bool

	parsedExpiresAt   time.Time
	lowerEmailPattern string
}

// BindFlags binds command line flags.
func (c *Config) BindFlags(f *flag.FlagSet) {
	f.StringVar(&c.SatelliteDB, "satellitedb", "", "connection URL for satellite DB (required)")
	f.StringVar(&c.ExpiresAt, "expires-at", "", "expiration time for the OM license in RFC3339 format (required, e.g. 2027-01-01T00:00:00Z)")
	f.BoolVar(&c.SkipConfirm, "skip-confirmation", false, "skip confirmation prompt")
	f.BoolVar(&c.Verbose, "verbose", false, "log info about each processed user")
	f.IntVar(&c.BatchSize, "batch-size", 500, "number of users to fetch per page")
	f.StringVar(&c.EmailPattern, "email-pattern", "", "only process users whose email matches this shell-style wildcard pattern, e.g. '*@example.com' (case-insensitive); if unset, process all")
	f.BoolVar(&c.DryRun, "dry-run", false, "log which users would receive an OM license without writing any changes to the DB")
}

// Verify validates the configuration and populates parsed fields.
func (c *Config) Verify() error {
	var errlist errs.Group
	if c.SatelliteDB == "" {
		errlist.Add(errors.New("flag '--satellitedb' is not set"))
	}
	if c.ExpiresAt == "" {
		errlist.Add(errors.New("flag '--expires-at' is not set"))
	}
	if c.BatchSize <= 0 {
		errlist.Add(errors.New("flag '--batch-size' must be positive"))
	}
	if err := errlist.Err(); err != nil {
		return err
	}

	expiresAt, err := time.Parse(time.RFC3339, c.ExpiresAt)
	if err != nil {
		return errs.New("invalid --expires-at value (expected RFC3339): %+v", err)
	}
	if !expiresAt.After(time.Now()) {
		return errs.New("--expires-at must be in the future")
	}
	c.parsedExpiresAt = expiresAt

	if c.EmailPattern != "" {
		c.lowerEmailPattern = strings.ToLower(c.EmailPattern)
		// path.Match validates the pattern on the first call; use a probe string so malformed patterns are caught here.
		if _, err := path.Match(c.lowerEmailPattern, "probe"); err != nil {
			return errs.New("invalid --email-pattern: %+v", err)
		}
	}

	return nil
}

// matchesEmail reports whether the configured wildcard pattern matches email. Returns true when no pattern is set.
func (c *Config) matchesEmail(email string) bool {
	if c.lowerEmailPattern == "" {
		return true
	}
	ok, _ := path.Match(c.lowerEmailPattern, strings.ToLower(email))
	return ok
}

func init() {
	rootCmd.AddCommand(grantCmd)
	config.BindFlags(grantCmd.Flags())
}

func main() {
	logger, _, _ := process.NewLogger("om-license")
	zap.ReplaceGlobals(logger)

	process.Exec(rootCmd)
}

func run(cmd *cobra.Command, _ []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	if err := config.Verify(); err != nil {
		return err
	}

	confirmPrompt := "Grant OM license for ALL active users without an active OM license?"
	if config.EmailPattern != "" {
		confirmPrompt = fmt.Sprintf("Grant OM license for active users whose email matches %q and do not already have an active OM license?", config.EmailPattern)
	}
	if config.DryRun {
		log.Info("Dry run enabled: no DB changes will be written")
	} else if !config.SkipConfirm {
		if !askForConfirmation(confirmPrompt) {
			log.Info("Operation cancelled by user")
			return nil
		}
	}

	satDB, err := satellitedb.Open(ctx, log.Named("db"), config.SatelliteDB, satellitedb.Options{
		ApplicationName: "om-license",
	})
	if err != nil {
		return errs.New("error connecting to satellite database: %+v", err)
	}
	defer func() { err = errs.Combine(err, satDB.Close()) }()

	return GrantOMLicenseToAllActiveUsers(ctx, log, satDB, config)
}

// GrantOMLicenseToAllActiveUsers iterates over all active users and, for each user without an active OM license entitlement, appends a new OM license with the configured expiration. PublicID and bucket remain unset.
func GrantOMLicenseToAllActiveUsers(ctx context.Context, log *zap.Logger, satelliteDB satellite.DB, cfg Config) (err error) {
	licenses := entitlements.NewService(log.Named("entitlements"), satelliteDB.Console().Entitlements()).Licenses()
	return grantOMLicenseToAllActiveUsers(ctx, log, satelliteDB.Console().Users(), licenses, cfg)
}

func grantOMLicenseToAllActiveUsers(ctx context.Context, log *zap.Logger, users console.Users, licenses *entitlements.Licenses, cfg Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Take an up-front snapshot of the active users so the long grant phase
	// below is immune to concurrent status changes (which would otherwise shift
	// OFFSET-based pagination and cause skips or duplicates), and so a mid-run
	// "page is out of range" error from GetByStatus (returned when TotalCount
	// shrinks between queries) does not abort the whole run.
	snapshot, err := collectActiveUsers(ctx, log, users, cfg)
	if err != nil {
		return err
	}
	log.Info("Collected active users", zap.Int("count", len(snapshot)))

	var errList errs.Group
	now := time.Now()
	var added, skipped int

	for _, user := range snapshot {
		existing, err := licenses.Get(ctx, user.ID)
		if err != nil {
			errList.Add(errs.New("error getting licenses for user %s: %+v", user.ID, err))
			continue
		}

		log := log.With(zap.Stringer("user_id", user.ID), zap.String("email", user.Email))
		if hasActiveOMLicense(existing, now) {
			skipped++
			if cfg.Verbose {
				log.Info("User already has active OM license, skipping")
			}
			continue
		}

		if cfg.DryRun {
			added++
			log.Info("Would add OM license (dry run)", zap.Time("expires_at", cfg.parsedExpiresAt))
			continue
		}

		existing.Licenses = append(existing.Licenses, entitlements.AccountLicense{
			Type:      omLicenseType,
			ExpiresAt: cfg.parsedExpiresAt,
		})

		if err := licenses.Set(ctx, user.ID, existing); err != nil {
			errList.Add(errs.New("error setting licenses for user %s: %+v", user.ID, err))
			continue
		}
		added++
		if cfg.Verbose {
			log.Info("Added OM license", zap.Time("expires_at", cfg.parsedExpiresAt))
		}
	}

	log.Info("Grant OM license complete",
		zap.Bool("dry_run", cfg.DryRun),
		zap.Int("added", added),
		zap.Int("skipped", skipped),
	)

	if err := errList.Err(); err != nil {
		return errs.New("errors occurred while ensuring OM licenses: %+v", err)
	}
	return nil
}

// collectActiveUsers paginates through every active user and returns a snapshot
// of {ID, Email}. "page is out of range" from the underlying GetByStatus is
// treated as end-of-iteration rather than a fatal error — it can surface when
// the active-user count shrinks between the count and fetch queries.
func collectActiveUsers(ctx context.Context, log *zap.Logger, users console.Users, cfg Config) (_ []console.User, err error) {
	defer mon.Task()(&ctx)(&err)

	cursor := console.UserCursor{Limit: uint(cfg.BatchSize), Page: 1}
	var snapshot []console.User

	for {
		page, err := users.GetByStatus(ctx, console.Active, cursor)
		if err != nil {
			if strings.Contains(err.Error(), "page is out of range") {
				break
			}
			return nil, errs.New("error fetching active users: %+v", err)
		}
		if len(page.Users) == 0 {
			break
		}

		var matched int
		for _, u := range page.Users {
			if !cfg.matchesEmail(u.Email) {
				continue
			}
			snapshot = append(snapshot, console.User{ID: u.ID, Email: u.Email})
			matched++
		}
		log.Info("Collecting active users batch", zap.Int("fetched", len(page.Users)), zap.Int("matched", matched), zap.Uint("page", cursor.Page))

		if cursor.Page >= page.PageCount {
			break
		}
		cursor.Page++
	}

	return snapshot, nil
}

// hasActiveOMLicense returns true if any OM license in the set is active at now.
func hasActiveOMLicense(licenses entitlements.AccountLicenses, now time.Time) bool {
	for _, l := range licenses.Licenses {
		if l.Type != omLicenseType {
			continue
		}
		if !l.ExpiresAt.IsZero() && !l.ExpiresAt.After(now) {
			continue
		}
		if !l.RevokedAt.IsZero() && !l.RevokedAt.After(now) {
			continue
		}
		return true
	}
	return false
}

func askForConfirmation(prompt string) bool {
	fmt.Printf("%s (y/n): ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
		fmt.Print("Please enter 'y' for yes or 'n' for no: ")
	}
	return false
}
