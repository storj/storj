// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package zombiedeletion

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/metabase"
)

var (
	// Error defines the zombiedeletion chore errors class.
	Error = errs.Class("zombie deletion chore")
	mon   = monkit.Package()
)

// Config contains configurable values for zombie object cleanup.
type Config struct {
	Interval           time.Duration `help:"the time between each attempt to go through the db and clean up zombie objects" releaseDefault:"15h" devDefault:"10s"`
	Enabled            bool          `help:"set if zombie object cleanup is enabled or not" default:"true"`
	ListLimit          int           `help:"how many objects to query in a batch" default:"100"`
	InactiveFor        time.Duration `help:"after what time object will be deleted if there where no new upload activity" default:"24h"`
	AsOfSystemInterval time.Duration `help:"as of system interval" releaseDefault:"-5m" devDefault:"-1us" testDefault:"-1us"`
}

// Chore implements the zombie objects cleanup chore.
//
// architecture: Chore
type Chore struct {
	log      *zap.Logger
	config   Config
	metabase *metabase.DB

	nowFn func() time.Time
	Loop  *sync2.Cycle
}

// NewChore creates a new instance of the zombiedeletion chore.
func NewChore(log *zap.Logger, config Config, metabase *metabase.DB) *Chore {
	return &Chore{
		log:      log,
		config:   config,
		metabase: metabase,

		nowFn: time.Now,
		Loop:  sync2.NewCycle(config.Interval),
	}
}

// Run starts the zombiedeletion loop service.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !chore.config.Enabled {
		return nil
	}

	return chore.Loop.Run(ctx, chore.deleteZombieObjects)
}

// Close stops the zombiedeletion chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}

// TestingSetNow allows tests to have the server act as if the current time is whatever they want.
func (chore *Chore) TestingSetNow(nowFn func() time.Time) {
	chore.nowFn = nowFn
}

func (chore *Chore) deleteZombieObjects(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	chore.log.Debug("deleting zombie objects")

	opts := metabase.DeleteZombieObjects{
		DeadlineBefore:     chore.nowFn(),
		InactiveDeadline:   chore.nowFn().Add(-chore.config.InactiveFor),
		BatchSize:          chore.config.ListLimit,
		AsOfSystemInterval: chore.config.AsOfSystemInterval,
	}

	err = chore.metabase.DeleteZombieObjects(ctx, opts)
	if err != nil {
		return err
	}

	return nil
}
