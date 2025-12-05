// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package dbcleanup

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/console"
)

var mon = monkit.Package()

// Config contains the configuration for the console DB cleanup chore.
type Config struct {
	Enabled  bool          `help:"whether to run this chore" default:"false"`
	Interval time.Duration `help:"interval between chore cycles" default:"24h"`

	AsOfSystemTimeInterval time.Duration `help:"interval for 'AS OF SYSTEM TIME' clause (CockroachDB specific) to read from the DB at a specific time in the past" default:"-5m" testDefault:"0"`
	PageSize               int           `help:"maximum number of database records to scan at once" default:"1000"`

	MaxUnverifiedUserAge time.Duration `help:"maximum lifetime of unverified user account records" default:"168h"`
}

// Chore periodically removes unwanted records from the satellite console database.
type Chore struct {
	log           *zap.Logger
	db            console.DB
	Loop          *sync2.Cycle
	config        Config
	consoleConfig console.Config
}

// NewChore creates a new console DB cleanup chore.
func NewChore(log *zap.Logger, db console.DB, config Config, consoleConfig console.Config) *Chore {
	return &Chore{
		log:           log,
		db:            db,
		config:        config,
		consoleConfig: consoleConfig,
		Loop:          sync2.NewCycle(config.Interval),
	}
}

// Run runs the console DB cleanup chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		before := time.Now().Add(-chore.config.MaxUnverifiedUserAge)
		err := chore.db.Users().DeleteUnverifiedBefore(ctx, before, chore.config.AsOfSystemTimeInterval, chore.config.PageSize)
		if err != nil {
			chore.log.Error("Error deleting unverified users", zap.Error(err))
		}

		err = chore.db.WebappSessions().DeleteExpired(ctx, time.Now(), chore.config.AsOfSystemTimeInterval, chore.config.PageSize)
		if err != nil {
			chore.log.Error("Error deleting expired webapp sessions", zap.Error(err))
		}

		err = chore.db.APIKeys().DeleteExpiredByNamePrefix(ctx, chore.consoleConfig.ObjectBrowserKeyLifetime, chore.consoleConfig.ObjectBrowserKeyNamePrefix, chore.config.AsOfSystemTimeInterval, chore.config.PageSize)
		if err != nil {
			chore.log.Error("Error deleting expired API keys", zap.Error(err))
		}

		return nil
	})
}

// Close stops the console DB cleanup chore.
func (chore *Chore) Close() error {
	chore.Loop.Stop()
	return nil
}
