// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package rolluparchive

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/accounting"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("rolluparchive")
	mon   = monkit.Package()
)

// Config contains configurable values for rollup archiver.
type Config struct {
	Enabled    bool          `help:"whether or not the rollup archive is enabled." default:"true"`
	Interval   time.Duration `help:"how frequently rollup archiver should run" releaseDefault:"24h" devDefault:"120s" testDefault:"$TESTINTERVAL"`
	ArchiveAge time.Duration `help:"age at which a rollup is archived" default:"2160h" testDefault:"24h"`
	BatchSize  int           `help:"number of records to delete per delete execution." default:"100" testDefault:"1000"`
}

// Chore archives bucket and storagenode rollups at a given interval.
//
// architecture: Chore
type Chore struct {
	log               *zap.Logger
	Loop              *sync2.Cycle
	config            Config
	nodeAccounting    accounting.StoragenodeAccounting
	projectAccounting accounting.ProjectAccounting
}

// New creates a new rollup archiver chore.
func New(log *zap.Logger, sdb accounting.StoragenodeAccounting, pdb accounting.ProjectAccounting, config Config) *Chore {
	return &Chore{
		log:               log,
		Loop:              sync2.NewCycle(config.Interval),
		config:            config,
		nodeAccounting:    sdb,
		projectAccounting: pdb,
	}
}

// Run starts the archiver chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if chore.config.ArchiveAge < 0 {
		return Error.New("archive age can't be less than 0")
	}
	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		cutoff := time.Now().UTC().Add(-chore.config.ArchiveAge)
		err := chore.ArchiveRollups(ctx, cutoff, chore.config.BatchSize)
		if err != nil {
			chore.log.Error("error archiving SN and bucket bandwidth rollups", zap.Error(err))
		}
		return nil
	})
}

// Close stops the service and releases any resources.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}

// ArchiveRollups will remove old rollups from active rollup tables.
func (chore *Chore) ArchiveRollups(ctx context.Context, cutoff time.Time, batchSize int) (err error) {
	defer mon.Task()(&ctx)(&err)
	nodeRollupsArchived, err := chore.nodeAccounting.ArchiveRollupsBefore(ctx, cutoff, batchSize)
	if err != nil {
		chore.log.Error("archiving node bandwidth rollups", zap.Int("rollups-archived", nodeRollupsArchived), zap.Error(err))
		return Error.Wrap(err)
	}
	bucketRollupsArchived, err := chore.projectAccounting.ArchiveRollupsBefore(ctx, cutoff, batchSize)
	if err != nil {
		chore.log.Error("archiving bucket bandwidth rollups", zap.Int("rollups-archived", bucketRollupsArchived), zap.Error(err))
		return Error.Wrap(err)
	}
	return nil
}
