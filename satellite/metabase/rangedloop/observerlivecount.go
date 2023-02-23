// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/storj/satellite/metabase/segmentloop"
)

var _ monkit.StatSource = (*LiveCountObserver)(nil)
var _ Observer = (*LiveCountObserver)(nil)
var _ Partial = (*LiveCountObserver)(nil)

// LiveCountObserver reports a count of segments during loop execution.
// This can be used to report the rate and progress of the loop.
type LiveCountObserver struct {
	numSegments int64
}

// NewLiveCountObserver .
// To avoid pollution, make sure to only use one instance of this observer.
// Also make sure to only add it to instances of the loop which are actually doing something.
func NewLiveCountObserver() *LiveCountObserver {
	liveCount := &LiveCountObserver{}
	mon.Chain(liveCount)
	return liveCount
}

// Start resets the count at start of the ranged segment loop.
func (o *LiveCountObserver) Start(context.Context, time.Time) error {
	atomic.StoreInt64(&o.numSegments, 0)
	return nil
}

// Fork returns a shared instance so we have a view of all loop ranges.
func (o *LiveCountObserver) Fork(ctx context.Context) (Partial, error) {
	return o, nil
}

// Join does nothing because the instance is shared across ranges.
func (o *LiveCountObserver) Join(ctx context.Context, partial Partial) error {
	return nil
}

// Finish does nothing at loop end.
func (o *LiveCountObserver) Finish(ctx context.Context) error {
	return nil
}

// Process increments the counter.
func (o *LiveCountObserver) Process(ctx context.Context, segments []segmentloop.Segment) error {
	processed := atomic.AddInt64(&o.numSegments, int64(len(segments)))

	mon.IntVal("segmentsProcessed").Observe(processed)
	return nil
}

// Stats implements monkit.StatSource to report the number of segments.
func (o *LiveCountObserver) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	cb(monkit.NewSeriesKey("rangedloop_live"), "num_segments", float64(atomic.LoadInt64(&o.numSegments)))
}
