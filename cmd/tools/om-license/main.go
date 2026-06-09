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
		Short: "Grant every active user free OM seats up to a configurable target count",
		Long: "Iterates over all active users and ensures each has an active free OM license\n" +
			"(Type=OM, ProductID=0) granting at least --count seats. If an active free OM row already\n" +
			"grants >= target seats, the user is skipped. If it grants fewer, its Count is bumped\n" +
			"to the target. If no active free OM row exists, a new one is appended with ProductID=0\n" +
			"and Count=target; any expired or revoked OM rows are left untouched.",
		RunE: run,
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
	Count        int
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
	f.IntVar(&c.Count, "count", 1, "target free OM seat count per user; users already at or above this target are skipped")
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
	if c.Count < 1 {
		errlist.Add(errors.New("flag '--count' must be >= 1"))
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

	confirmPrompt := fmt.Sprintf("Grant up to %d free OM seat(s) per active user (bumping existing active free rows if below target)?", config.Count)
	if config.EmailPattern != "" {
		confirmPrompt = fmt.Sprintf("Grant up to %d free OM seat(s) per active user whose email matches %q?", config.Count, config.EmailPattern)
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

// GrantOMLicenseToAllActiveUsers iterates over all active users and ensures each
// has an active free OM license (Type=OM, ProductID=0) with Count >= cfg.Count.
// If an active free OM row exists with Count >= target, the user is skipped. If
// it exists with Count < target, that row's Count is bumped to target. If no
// active free OM row exists, a new row is appended with the configured
// expiration; pre-existing expired or revoked OM rows are left untouched.
// PublicID and bucket on new rows remain unset.
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
	target := cfg.Count
	var added, updated, skipped int

	for _, user := range snapshot {
		existing, err := licenses.Get(ctx, user.ID)
		if err != nil {
			errList.Add(errs.New("error getting licenses for user %s: %+v", user.ID, err))
			continue
		}

		log := log.With(zap.Stringer("user_id", user.ID), zap.String("email", user.Email))

		indexes, found := findActiveFreeOMLicense(existing, now)

		switch {
		case found && len(indexes) > 1:
			skipped++
			if cfg.Verbose {
				log.Info("User already has more than an active free OM license independently if they above or below the target count, skipping",
					zap.Int("current_count", existing.Licenses[indexes[0]].Count),
					zap.Int("target_count", target),
					zap.Stringer("user_id", user.ID),
				)
			}

		case found && existing.Licenses[indexes[0]].Count >= target:
			skipped++
			if cfg.Verbose {
				log.Info("User already has active free OM license at or above target count, skipping",
					zap.Int("current_count", existing.Licenses[indexes[0]].Count),
					zap.Int("target_count", target),
					zap.Stringer("user_id", user.ID),
				)
			}

		case found:
			currentCount := existing.Licenses[indexes[0]].Count
			if cfg.DryRun {
				updated++
				log.Info("Would update Count on existing active free OM license (dry run)",
					zap.Int("current_count", currentCount),
					zap.Int("target_count", target),
					zap.Stringer("user_id", user.ID),
				)
				continue
			}
			existing.Licenses[indexes[0]].Count = target
			if err := licenses.Set(ctx, user.ID, existing); err != nil {
				errList.Add(errs.New("error setting licenses for user %s: %+v", user.ID, err))
				continue
			}
			updated++
			if cfg.Verbose {
				log.Info("Updated Count on existing active free OM license",
					zap.Int("previous_count", currentCount),
					zap.Int("new_count", target),
					zap.Stringer("user_id", user.ID),
				)
			}

		default:
			if cfg.DryRun {
				added++
				log.Info("Would add new free OM license (dry run)",
					zap.Int("target_count", target),
					zap.Time("expires_at", cfg.parsedExpiresAt),
					zap.Stringer("user_id", user.ID),
				)
				continue
			}
			existing.Licenses = append(existing.Licenses, entitlements.AccountLicense{
				Type:      omLicenseType,
				ProductID: 0,
				Count:     target,
				ExpiresAt: cfg.parsedExpiresAt,
			})
			if err := licenses.Set(ctx, user.ID, existing); err != nil {
				errList.Add(errs.New("error setting licenses for user %s: %+v", user.ID, err))
				continue
			}
			added++
			if cfg.Verbose {
				log.Info("Added free OM license",
					zap.Int("count", target),
					zap.Time("expires_at", cfg.parsedExpiresAt),
					zap.Stringer("user_id", user.ID),
				)
			}
		}
	}

	log.Info("Grant OM license complete",
		zap.Bool("dry_run", cfg.DryRun),
		zap.Int("target_count", target),
		zap.Int("added", added),
		zap.Int("updated", updated),
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

// findActiveFreeOMLicense returns the indexes of the active free OM licenses (Type == OM,
// ProductID == 0, not expired, not revoked) at now, and reports whether any was found.
func findActiveFreeOMLicense(licenses entitlements.AccountLicenses, now time.Time) ([]int, bool) {
	var indexes []int
	for i, l := range licenses.Licenses {
		if l.Type != omLicenseType {
			continue
		}
		if l.ProductID != 0 {
			continue
		}
		if !l.ExpiresAt.IsZero() && !l.ExpiresAt.After(now) {
			continue
		}
		if !l.RevokedAt.IsZero() && !l.RevokedAt.After(now) {
			continue
		}

		indexes = append(indexes, i)
	}

	return indexes, len(indexes) > 0
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

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading your input. %s", err)
		os.Exit(1)
	}

	return false
}
