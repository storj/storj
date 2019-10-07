// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/overlay"
)

var mon = monkit.Package()

// Chore populates the graceful exit transfer queue.
//
// architecture: Chore
type Chore struct {
	log          *zap.Logger
	Loop         sync2.Cycle
	db           DB
	config       Config
	overlay      overlay.DB
	metainfoLoop *metainfo.Loop
}

// Config for the chore
type Config struct {
	ChoreBatchSize int           `help:"size of the buffer used to batch inserts into the transfer queue." default:"500"`
	ChoreInterval  time.Duration `help:"how often to run the transfer queue chore." releaseDefault:"30s" devDefault:"10s"`
}

// NewChore instantiates Chore.
func NewChore(log *zap.Logger, db DB, overlay overlay.DB, config Config, metaLoop *metainfo.Loop) *Chore {
	return &Chore{
		log:          log,
		Loop:         *sync2.NewCycle(config.ChoreInterval),
		db:           db,
		config:       config,
		overlay:      overlay,
		metainfoLoop: metaLoop,
	}
}

// Run starts the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		chore.log.Info("running graceful exit chore.")

		exitingNodes, err := chore.overlay.GetExitingNodesLoopIncomplete(ctx)
		if err != nil {
			chore.log.Error("error retrieving nodes that have not completed the metainfo loop.", zap.Error(err))
			return nil
		}

		nodeCount := len(exitingNodes)
		chore.log.Debug("graceful exit.", zap.Int("exitingNodes", nodeCount))
		if nodeCount == 0 {
			return nil
		}

		pathCollector := NewPathCollector(chore.db, exitingNodes, chore.log, chore.config.ChoreBatchSize)
		err = chore.metainfoLoop.Join(ctx, pathCollector)
		if err != nil {
			chore.log.Error("error joining metainfo loop.", zap.Error(err))
			return nil
		}

		err = pathCollector.Flush(ctx)
		if err != nil {
			chore.log.Error("error flushing collector buffer.", zap.Error(err))
			return nil
		}

		now := time.Now().UTC()
		for _, nodeID := range exitingNodes {
			exitStatus := overlay.ExitStatusRequest{
				NodeID:              nodeID,
				ExitLoopCompletedAt: now,
			}
			_, err = chore.overlay.UpdateExitStatus(ctx, &exitStatus)
			if err != nil {
				chore.log.Error("error updating exit status.", zap.Error(err))
			}
		}
		return nil
	})
}

// Close closes chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
