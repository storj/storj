// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"github.com/zeebo/errs"

	"storj.io/storj/satellite/gc/bloomfilter"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/mud"
)

// GcBf is a subcommand to start only a ranged loop with BF generation.
type GcBf struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *GcBf) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	mud.Provide[*GcBfCycleCheck](ball, NewGcBfCycleCheck)
	return mud.Or(
		Observability(ball),
		// the observer is optional in the ball, this subcommand is the one
		// that needs it
		mud.Select[*bloomfilter.Observer](ball),
		mud.Select[*GcBfCycleCheck](ball),
		mud.Select[*rangedloop.Service](ball))
}

// GcBfCycleCheck rejects configuration that only a one-shot run can honor.
type GcBfCycleCheck struct {
}

// NewGcBfCycleCheck fails when the configuration describes a single scan, which
// this continuous loop cannot honor.
func NewGcBfCycleCheck(config bloomfilter.Config) (*GcBfCycleCheck, error) {
	if config.ShardCount > 1 {
		// the shard cursor and the upload prefix only live in the process, so a
		// rotation interrupted by a restart would keep publishing generations
		// that cover shard 0 only
		return nil, errs.New("garbage-collection-bf.shard-count %d requires the gc-bf-once subcommand", config.ShardCount)
	}
	return &GcBfCycleCheck{}, nil
}
