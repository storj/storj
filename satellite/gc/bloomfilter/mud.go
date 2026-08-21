// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"go.uber.org/zap"

	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module provides dependency injection configuration for garbage collection bloom filter components.
func Module(ball *mud.Ball) {

	config.RegisterConfig[Config](ball, "garbage-collection-bf")

	// the check hangs off the observer rather than off a subcommand, because any
	// --components selection that generates filters needs it; the splitter knows
	// whether its scan is a snapshot, so sources that are one by construction
	// (like the Avro export) pass without a stale interval
	mud.Provide[*Observer](ball, func(log *zap.Logger, config Config, overlay Overlay, splitter rangedloop.RangeSplitter) (*Observer, error) {
		if splitter == nil || !splitter.ReadsSnapshot() {
			return nil, rangedloop.ErrNoSnapshot
		}
		return NewObserver(log, config, overlay), nil
	})
	mud.Implementation[[]rangedloop.Observer, *Observer](ball)
	mud.Tag[*Observer, mud.Optional](ball, mud.Optional{})
}
