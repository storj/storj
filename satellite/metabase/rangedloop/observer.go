// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"time"

	"storj.io/storj/satellite/metabase/segmentloop"
)

// Observer subscribes to the parallel segment loop.
// It is intended that a naïve implementation is threadsafe.
type Observer interface {
	// Start is called at the beginning of each segment loop.
	Start(context.Context, time.Time) error

	// Fork creates a Partial to process a chunk of all the segments. It is
	// called after Start. It is not called concurrently.
	Fork(context.Context) (Partial, error)

	// Join is called for each partial returned by Fork. This gives the
	// opportunity to merge the output like in a reduce step. It will be called
	// before Finish. It is not called concurrently.
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
