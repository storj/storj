// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/mud"
)

// RangedLoop is a subcommand to start the ranged loop with configurable observers.
//
// Example:
//
//	satellite-mud ranged-loop --components='$bloomfilter.SyncObserverV2,$piecetracker.Observer'
type RangedLoop struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (r *RangedLoop) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		Observability(ball),
		mud.Select[*rangedloop.Service](ball))
}

// RangedLoopOnce is a subcommand to run the ranged loop once and stop.
type RangedLoopOnce struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (r *RangedLoopOnce) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		Observability(ball),
		mud.Select[*rangedloop.RunOnce](ball))
}
