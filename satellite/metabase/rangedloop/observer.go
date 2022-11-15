// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"time"

	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/segmentloop"
)

// Observer subscribes to the parallel segment loop.
// It is intended that a naïve implementation is threadsafe.
type Observer interface {
	Start(context.Context, time.Time, metabase.NodeAliasMap) error

	// Fork creates a Partial to process a chunk of all the segments.
	Fork(context.Context) (Partial, error)
	// Join is called after the chunk for Partial is done.
	// This gives the opportunity to merge the output like in a reduce step.
	Join(context.Context, Partial) error

	// Finish is called after all segments are processed by all observers.
	Finish(context.Context) error
}

// Partial processes a part of the total range of segments.
type Partial interface {
	// Process is called repeatedly with batches of segments.
	// It is not called concurrently on the same instance.
	Process(context.Context, []segmentloop.Segment) error
}
