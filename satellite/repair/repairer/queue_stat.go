// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/repair/queue"
)

// QueueStatConfig configures the queue checker chore.
// note: this is intentionally not part of the Config, as it is required by the chore, and it makes it possible to print out only required configs for chore (full repair config is not required).
type QueueStatConfig struct {
	Interval time.Duration `help:"how frequently core should check the size of the repair queue" releaseDefault:"1h" devDefault:"1m0s" testDefault:"$TESTINTERVAL"`
}

// QueueStat contains the information and variables to ensure the Software is up-to-date.
type QueueStat struct {
	db         queue.RepairQueue
	log        *zap.Logger
	mon        *monkit.Scope
	Loop       *sync2.Cycle
	mu         sync.Mutex
	stats      map[string]queue.Stat
	updated    time.Time
	placements []storj.PlacementConstraint
}

var _ monkit.StatSource = &QueueStat{}

// NewQueueStat creates a chore to stat repair queue statistics.
func NewQueueStat(log *zap.Logger, registry *monkit.Registry, placements []storj.PlacementConstraint, db queue.RepairQueue, checkInterval time.Duration) *QueueStat {

	chore := &QueueStat{
		db:         db,
		log:        log,
		mon:        registry.Package(),
		Loop:       sync2.NewCycle(checkInterval),
		placements: placements,
	}
	chore.mon.Chain(chore)
	return chore
}

// Run logs the current version information.
func (c *QueueStat) Run(ctx context.Context) (err error) {
	defer c.mon.Task()(&ctx)(&err)
	return c.Loop.Run(ctx, func(ctx context.Context) error {
		c.RunOnce(ctx)
		return nil
	})
}

// RunOnce refresh the queue statistics.
func (c *QueueStat) RunOnce(ctx context.Context) {
	stats, err := c.db.Stat(ctx)
	if err != nil {
		c.log.Error("couldn't get repair queue statistic", zap.Error(err))
	}
	c.mu.Lock()
	c.stats = map[string]queue.Stat{}
	for _, stat := range stats {
		c.stats[key(stat.Placement, stat.MinAttemptedAt != nil)] = stat
	}
	c.updated = time.Now()
	c.mu.Unlock()
}

// Stats implements stat source.
func (c *QueueStat) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.updated) > 24*time.Hour {
		// stat is one day old (or never retrieved), not so interesting...
		return
	}
	for _, placement := range c.placements {
		for _, attempted := range []bool{false, true} {
			keyWithDefaultTags := monkit.NewSeriesKey("repair_queue").
				WithTags(
					monkit.NewSeriesTag("attempted", strconv.FormatBool(attempted)),
					monkit.NewSeriesTag("placement", fmt.Sprintf("%d", placement)))

			k := key(placement, attempted)
			stat, found := c.stats[k]
			if !found {
				cb(keyWithDefaultTags, "count", 0)
				cb(keyWithDefaultTags, "age", time.Since(c.updated).Seconds())
				cb(keyWithDefaultTags, "since_oldest_inserted_sec", 0)
				cb(keyWithDefaultTags, "since_latest_inserted_sec", 0)
				cb(keyWithDefaultTags, "since_oldest_attempted_sec", 0)
				cb(keyWithDefaultTags, "since_latest_attempted_sec", 0)

				continue
			}

			cb(keyWithDefaultTags, "count", float64(stat.Count))
			cb(keyWithDefaultTags, "age", time.Since(c.updated).Seconds())
			cb(keyWithDefaultTags, "since_oldest_inserted_sec", time.Since(stat.MinInsertedAt).Seconds())
			cb(keyWithDefaultTags, "since_latest_inserted_sec", time.Since(stat.MaxInsertedAt).Seconds())
			if stat.MinAttemptedAt != nil {
				cb(keyWithDefaultTags, "since_oldest_attempted_sec", time.Since(*stat.MinAttemptedAt).Seconds())
			} else {
				cb(keyWithDefaultTags, "since_oldest_attempted_sec", 0)
			}
			if stat.MaxAttemptedAt != nil {
				cb(keyWithDefaultTags, "since_latest_attempted_sec", time.Since(*stat.MaxAttemptedAt).Seconds())
			} else {
				cb(keyWithDefaultTags, "since_latest_attempted_sec", 0)
			}
		}
	}

}

func key(placement storj.PlacementConstraint, attempted bool) string {
	return fmt.Sprintf("%d-%v", placement, attempted)
}
