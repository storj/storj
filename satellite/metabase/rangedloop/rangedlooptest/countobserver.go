// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedlooptest

import (
	"context"
	"time"

	"storj.io/storj/satellite/metabase/rangedloop"
)

var _ rangedloop.Observer = (*CountObserver)(nil)
var _ rangedloop.Partial = (*CountObserver)(nil)

// CountObserver is a subscriber to the ranged segment  loop which counts the number of segments.
type CountObserver struct {
	NumSegments int
}

// Start is the callback for segment loop start.
func (c *CountObserver) Start(ctx context.Context, time time.Time) error {
	c.NumSegments = 0
	return nil
}

// Fork splits the observer to count ranges of segments.
func (c *CountObserver) Fork(ctx context.Context) (rangedloop.Partial, error) {
	// return new instance for threadsafety
	return &CountObserver{}, nil
}

// Join adds the count of all the ranges together.
func (c *CountObserver) Join(ctx context.Context, partial rangedloop.Partial) error {
	countPartial := partial.(*CountObserver)
	c.NumSegments += countPartial.NumSegments
	// Range done
	return nil
}

// Finish is the callback for ranged segment loop end.
func (c *CountObserver) Finish(ctx context.Context) error {
	return nil
}

// Process counts the size of a batch of segments.
func (c *CountObserver) Process(ctx context.Context, segments []rangedloop.Segment) error {
	c.NumSegments += len(segments)
	return nil
}
