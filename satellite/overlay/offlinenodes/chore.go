// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package offlinenodes

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/overlay"
)

var mon = monkit.Package()

// Config contains configurable values for offline nodes chore.
type Config struct {
	Interval  time.Duration `help:"how often to check for offline nodes and send them emails" default:"1h"`
	Cooldown  time.Duration `help:"how long to wait between sending Node Offline emails" default:"24h"`
	MaxEmails int           `help:"max number of offline emails to send a node operator until the node comes back online" default:"3"`
	Limit     int           `help:"Max number of nodes to return in a single query. Chore will iterate until rows returned is less than limit" releaseDefault:"1000" devDefault:"1000"`
}

// Chore sends emails to offline nodes.
type Chore struct {
	log    *zap.Logger
	mail   *mailservice.Service
	cache  *overlay.Service
	config Config
	Loop   *sync2.Cycle
}

// NewChore creates a new offline nodes Chore.
func NewChore(log *zap.Logger, mail *mailservice.Service, cache *overlay.Service, config Config) *Chore {
	return &Chore{
		log:    log,
		mail:   mail,
		cache:  cache,
		config: config,
		Loop:   sync2.NewCycle(config.Interval),
	}
}

// Run runs the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	// multiply max emails by email cooldown to get cutoff for emails
	// e.g. cooldown = 24h, maxEmails = 3
	// after 72h the node should get 3 emails and no more.
	cutoff := time.Duration(chore.config.Cooldown.Nanoseconds() * int64(chore.config.MaxEmails))
	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		for {
			count, err := chore.cache.InsertOfflineNodeEvents(ctx, chore.config.Cooldown, cutoff, chore.config.Limit)
			if err != nil {
				chore.log.Error("error inserting offline node events", zap.Error(err))
				return nil
			}
			if count < chore.config.Limit {
				break
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
