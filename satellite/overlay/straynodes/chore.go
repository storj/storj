// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package straynodes

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/overlay"
)

var mon = monkit.Package()

// Config contains configurable values for stray nodes chore.
type Config struct {
	EnableDQ                  bool          `help:"whether nodes will be disqualified if they have not been contacted in some time" releaseDefault:"true" devDefault:"true"`
	Interval                  time.Duration `help:"how often to check for and DQ stray nodes" releaseDefault:"168h" devDefault:"5m" testDefault:"1m"`
	MaxDurationWithoutContact time.Duration `help:"length of time a node can go without contacting satellite before being disqualified" releaseDefault:"720h" devDefault:"7200h" testDefault:"5m"`
	Limit                     int           `help:"Max number of nodes to return in a single query. Chore will iterate until rows returned is less than limit" releaseDefault:"1000" devDefault:"1000"`
}

// Chore disqualifies stray nodes.
type Chore struct {
	log                       *zap.Logger
	cache                     *overlay.Service
	maxDurationWithoutContact time.Duration
	limit                     int
	Loop                      *sync2.Cycle
}

// NewChore creates a new stray nodes Chore.
func NewChore(log *zap.Logger, cache *overlay.Service, config Config) *Chore {
	return &Chore{
		log:                       log,
		cache:                     cache,
		maxDurationWithoutContact: config.MaxDurationWithoutContact,
		limit:                     config.Limit,
		Loop:                      sync2.NewCycle(config.Interval),
	}
}

// Run runs the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		var total int
		for {
			n, err := chore.cache.DQNodesLastSeenBefore(ctx, time.Now().UTC().Add(-chore.maxDurationWithoutContact), chore.limit)
			if err != nil {
				chore.log.Error("error disqualifying stray nodes", zap.Error(err))
				mon.IntVal("stray_nodes_dq_count").Observe(int64(total))
				return nil
			}
			total += n
			if n < chore.limit {
				break
			}
		}
		mon.IntVal("stray_nodes_dq_count").Observe(int64(total))
		return nil
	})
}

// Close closes chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
