// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/storj/satellite/metabase"
)

var _ monkit.StatSource = (*LiveCountObserver)(nil)
var _ Observer = (*LiveCountObserver)(nil)
var _ Partial = (*LiveCountObserver)(nil)

// LiveCountObserver reports a count of segments during loop execution.
// This can be used to report the rate and progress of the loop.
// TODO we may need better name for this type.
type LiveCountObserver struct {
	metabase                 *metabase.DB
	suspiciousProcessedRatio float64
	asOfSystemInterval       time.Duration

	segmentsProcessed int64
	segmentsBefore    int64
}

// NewLiveCountObserver .
// To avoid pollution, make sure to only use one instance of this observer.
// Also make sure to only add it to instances of the loop which are actually doing something.
func NewLiveCountObserver(metabase *metabase.DB, suspiciousProcessedRatio float64, asOfSystemInterval time.Duration) *LiveCountObserver {
	liveCount := &LiveCountObserver{
		metabase:                 metabase,
		suspiciousProcessedRatio: suspiciousProcessedRatio,
		asOfSystemInterval:       asOfSystemInterval,
	}
	mon.Chain(liveCount)
	return liveCount
}

// Start resets the count at start of the ranged segment loop and gets
// statistis about segments table.
func (o *LiveCountObserver) Start(ctx context.Context, startTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	atomic.StoreInt64(&o.segmentsProcessed, 0)

	stats, err := o.metabase.GetTableStats(ctx, metabase.GetTableStats{
		AsOfSystemInterval: o.asOfSystemInterval,
	})
	if err != nil {
		return err
	}

	o.segmentsBefore = stats.SegmentCount
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

// Process increments the counter.
func (o *LiveCountObserver) Process(ctx context.Context, segments []Segment) error {
	processed := atomic.AddInt64(&o.segmentsProcessed, int64(len(segments)))

	mon.IntVal("segmentsProcessed").Observe(processed)
	return nil
}

// Finish gets segments count after range execution and verifies them against
// processed segments and segments in table before loop execution.
func (o *LiveCountObserver) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	stats, err := o.metabase.GetTableStats(ctx, metabase.GetTableStats{
		AsOfSystemInterval: o.asOfSystemInterval,
	})
	if err != nil {
		return err
	}

	segmentsProcessed := atomic.LoadInt64(&o.segmentsProcessed)
	return o.verifyCount(o.segmentsBefore, stats.SegmentCount, segmentsProcessed)
}

// Stats implements monkit.StatSource to report the number of segments.
func (o *LiveCountObserver) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	cb(monkit.NewSeriesKey("rangedloop_live"), "num_segments", float64(atomic.LoadInt64(&o.segmentsProcessed)))
}

func (o *LiveCountObserver) verifyCount(before, after, processed int64) error {
	low, high := before, after
	if low > high {
		low, high = high, low
	}

	var deltaFromBounds int64
	var ratio float64
	if processed < low {
		deltaFromBounds = low - processed
		// +1 to avoid division by zero
		ratio = float64(deltaFromBounds) / float64(low+1)
	} else if processed > high {
		deltaFromBounds = processed - high
		// +1 to avoid division by zero
		ratio = float64(deltaFromBounds) / float64(high+1)
	}

	mon.IntVal("segmentloop_verify_before").Observe(before)
	mon.IntVal("segmentloop_verify_after").Observe(after)
	mon.IntVal("segmentloop_verify_processed").Observe(processed)
	mon.IntVal("segmentloop_verify_outside").Observe(deltaFromBounds)
	mon.FloatVal("segmentloop_verify_outside_ratio").Observe(ratio)

	// If we have very few items from the bounds, then it's expected and the ratio does not capture it well.
	const minimumDeltaThreshold = 100
	if deltaFromBounds < minimumDeltaThreshold {
		return nil
	}

	if ratio > o.suspiciousProcessedRatio {
		mon.Event("ranged_loop_suspicious_segments_count")

		return Error.New("processed count looks suspicious: before:%v after:%v processed:%v ratio:%v threshold:%v", before, after, processed, ratio, o.suspiciousProcessedRatio)
	}

	return nil
}
