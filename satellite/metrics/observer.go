// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metrics

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase/rangedloop"
)

var (
	// Error defines the metrics chore errors class.
	Error = errs.Class("metrics")
	mon   = monkit.Package()
)

// Observer implements the ranged segment loop observer interface for data
// science metrics collection.
type Observer struct {
	metrics Metrics
}

var _ rangedloop.Observer = (*Observer)(nil)

// NewObserver instantiates a new rangedloop observer which aggregates
// object statistics from observed segments.
func NewObserver() *Observer {
	return &Observer{}
}

// Start implements the Observer method of the same name by resetting the
// aggregated metrics.
func (obs *Observer) Start(ctx context.Context, startTime time.Time) error {
	obs.metrics.Reset()
	return nil
}

// Fork implements the Observer method of the same name by returning a partial
// implementation that aggregates metrics about the observed segments/streams.
// These metrics will be aggregated into the observed totals during Join.
func (obs *Observer) Fork(ctx context.Context) (rangedloop.Partial, error) {
	return &observerFork{}, nil
}

// Join aggregates the partial metrics.
func (obs *Observer) Join(ctx context.Context, partial rangedloop.Partial) error {
	fork, ok := partial.(*observerFork)
	if !ok {
		return Error.New("expected %T but got %T", fork, partial)
	}

	// Flushing to count the stats for the last observed stream.
	fork.Flush()
	obs.metrics.Aggregate(fork.totals)
	return nil
}

// Finish emits the aggregated metrics.
func (obs *Observer) Finish(ctx context.Context) error {
	mon.IntVal("remote_dependent_object_count").Observe(obs.metrics.RemoteObjects)
	mon.IntVal("inline_object_count").Observe(obs.metrics.InlineObjects)

	mon.IntVal("total_inline_bytes").Observe(obs.metrics.TotalInlineBytes) //mon:locked
	mon.IntVal("total_remote_bytes").Observe(obs.metrics.TotalRemoteBytes) //mon:locked

	mon.IntVal("total_inline_segments").Observe(obs.metrics.TotalInlineSegments) //mon:locked
	mon.IntVal("total_remote_segments").Observe(obs.metrics.TotalRemoteSegments) //mon:locked
	return nil
}

// TestingMetrics returns the accumulated metrics. It is intended to be called
// from tests.
func (obs *Observer) TestingMetrics() Metrics {
	return obs.metrics
}

type observerFork struct {
	totals   Metrics
	stream   streamMetrics
	streamID uuid.UUID
}

// Process aggregates metrics about a range of metrics provided by the
// segment ranged loop.
func (fork *observerFork) Process(ctx context.Context, segments []rangedloop.Segment) error {
	for _, segment := range segments {
		if fork.streamID != segment.StreamID {
			// Stream ID has changed. Flush what we have so far.
			fork.Flush()
			fork.streamID = segment.StreamID
		}
		if segment.Inline() {
			fork.stream.inlineSegments++
			fork.stream.inlineBytes += int64(segment.EncryptedSize)
		} else {
			fork.stream.remoteSegments++
			fork.stream.remoteBytes += int64(segment.EncryptedSize)
		}
	}
	return nil
}

// Flush is called whenever a new stream is observed and when the fork is
// joined to aggregate the accumulated stream stats into the totals.
func (fork *observerFork) Flush() {
	fork.totals.TotalInlineSegments += fork.stream.inlineSegments
	fork.totals.TotalRemoteSegments += fork.stream.remoteSegments
	fork.totals.TotalInlineBytes += fork.stream.inlineBytes
	fork.totals.TotalRemoteBytes += fork.stream.remoteBytes
	if fork.stream.remoteSegments > 0 {
		// At least one remote segment was found for this stream so classify
		// as a remote object.
		fork.totals.RemoteObjects++
	} else if fork.stream.inlineSegments > 0 {
		// Only count an inline object if there is at least one inline segment
		// and no remote segments.
		fork.totals.InlineObjects++
	}
	fork.stream = streamMetrics{}
}

// streamMetrics tracks the metrics for an individual stream.
type streamMetrics struct {
	remoteSegments int64
	remoteBytes    int64
	inlineSegments int64
	inlineBytes    int64
}
