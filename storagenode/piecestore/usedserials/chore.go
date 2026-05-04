// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package usedserials

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/sync2"
)

// Config holds configuration for the used-serials cleanup Chore.
type Config struct {
	Interval              time.Duration `help:"how frequently expired serial numbers are removed from the in-memory store" default:"1h0m0s"`
	ExpirationGracePeriod time.Duration `help:"how long after expiration before a serial number is eligible for removal. Must be at least 30 minutes." default:"1h0m0s"`
}

// Chore periodically removes expired serial numbers from the in-memory table.
//
// architecture: Chore
type Chore struct {
	log         *zap.Logger
	table       *Table
	Loop        *sync2.Cycle
	gracePeriod time.Duration
}

// NewChore creates a Chore that cleans up the table according to cfg.
func NewChore(log *zap.Logger, table *Table, cfg Config) *Chore {
	if cfg.ExpirationGracePeriod < 30*time.Minute {
		log.Warn("ExpirationGracePeriod cannot be less than 30 minutes. Using default.")
		cfg.ExpirationGracePeriod = time.Hour
	}
	return &Chore{
		log:         log,
		table:       table,
		Loop:        sync2.NewCycle(cfg.Interval),
		gracePeriod: cfg.ExpirationGracePeriod,
	}
}

// Run starts the periodic cleanup loop.
func (c *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return c.Loop.Run(ctx, func(ctx context.Context) error {
		c.table.DeleteExpired(ctx, time.Now().Add(-c.gracePeriod))
		return nil
	})
}

// Close stops the chore.
func (c *Chore) Close() error {
	c.Loop.Close()
	return nil
}
