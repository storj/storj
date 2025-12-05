// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedlooptest

import (
	"context"
	"time"

	"storj.io/storj/satellite/metabase/rangedloop"
)

// SleepObserver is a subscriber to the segment loop which sleeps for every batch.
type SleepObserver struct {
	Duration time.Duration
}

// Start is the callback for segment loop start.
func (c *SleepObserver) Start(ctx context.Context, time time.Time) error {
	return nil
}

// Fork splits the observer to process a segment range.
func (c *SleepObserver) Fork(ctx context.Context) (rangedloop.Partial, error) {
	return c, nil
}

// Join is a noop.
func (c *SleepObserver) Join(ctx context.Context, partial rangedloop.Partial) error {
	// Range done
	return nil
}

// Finish is the callback for segment loop end.
func (c *SleepObserver) Finish(ctx context.Context) error {
	return nil
}

// Process sleeps for every batch of segments to simulate execution time.
func (c *SleepObserver) Process(ctx context.Context, segments []rangedloop.Segment) error {
	sleepTime := time.Duration(c.Duration.Nanoseconds() * int64(len(segments)))
	time.Sleep(sleepTime)
	return nil
}
