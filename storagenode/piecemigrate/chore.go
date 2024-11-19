// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecemigrate

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/piecestore"
)

var mon = monkit.Package()

// Config defines the configuration for the chore.
type Config struct {
	Interval time.Duration `help:"" default:"10m0s"`
}

// Chore migrates pieces.
//
// architecture: Chore
type Chore struct {
	log  *zap.Logger
	Loop *sync2.Cycle

	old, new piecestore.PieceBackend
	mu       sync.Mutex
	active   map[storj.NodeID]struct{}
}

// NewChore initializes and returns a new Chore instance.
func NewChore(log *zap.Logger, config Config, old, new piecestore.PieceBackend) *Chore {
	return &Chore{
		log:  log,
		Loop: sync2.NewCycle(config.Interval),

		old: old,
		new: new,

		active: make(map[storj.NodeID]struct{}),
	}
}

// TryMigrateOne enqueues a migration item for the given satellite and
// piece if the queue has capacity. Fails silently if the queue is full.
func (chore *Chore) TryMigrateOne(sat storj.NodeID, piece storj.PieceID) {
	// ugh
}

// SetMigrate enables or disables migration for the given satellite.
// Adds the satellite to the active set if migrate is true; otherwise,
// removes it.
func (chore *Chore) SetMigrate(sat storj.NodeID, migrate bool) {
	chore.mu.Lock()
	defer chore.mu.Unlock()

	if migrate {
		chore.active[sat] = struct{}{}
	} else {
		delete(chore.active, sat)
	}
}

// Run starts the chore loop to migrate pieces based on the
// configuration.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, chore.RunOnce)
}

// RunOnce executes a single iteration of the chore to migrate pieces
// based on the configuration.
func (chore *Chore) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	chore.mu.Lock()
	sats := maps.Keys(chore.active)
	chore.mu.Unlock()

	for _, sat := range sats {
		_ = chore.runSatellite(ctx, sat)
	}

	return nil
}

func (chore *Chore) runSatellite(ctx context.Context, sat storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	fmt.Println("LOL MIGRATE", sat.String())
	return nil
}

// Close shuts down the chore's loop and releases associated resources.
// Always returns nil.
func (chore *Chore) Close() (err error) {
	chore.Loop.Close()
	return nil
}
