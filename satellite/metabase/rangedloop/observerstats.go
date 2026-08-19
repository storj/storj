// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/eventkit"
	"storj.io/storj/satellite/metabase"
)

var (
	completedObserverStatsInstance         completedObserverStats
	completedObserverStatsInstanceInitOnce sync.Once
)

func sendObserverDurations(observerDurations []ObserverDuration) {
	for _, od := range observerDurations {
		ev.Event("rangedloop",
			eventkit.String("observer", observerName(od.Observer)),
			eventkit.Duration("duration", od.Duration))
	}

	completedObserverStatsInstance.setObserverDurations(observerDurations)
	completedObserverStatsInstanceInitOnce.Do(func() {
		mon.Chain(&completedObserverStatsInstance)
	})
}

// Implements monkit.StatSource.
// Reports the duration per observer from the last completed run of the ranged segment loop.
type completedObserverStats struct {
	mu                sync.Mutex
	observerDurations []ObserverDuration
}

// Stats implements monkit.StatSource to send the observer durations every time monkit is polled externally.
func (o *completedObserverStats) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// if there are no completed observers yet, no statistics will be sent
	for _, observerDuration := range o.observerDurations {
		key := monkit.NewSeriesKey("completed-observer-duration")
		key = key.WithTag("observer", observerName(observerDuration.Observer))

		cb(key, "duration", observerDuration.Duration.Seconds())
	}
}

// setObserverDurations sets the observer durations to report at ranged segment loop completion.
func (o *completedObserverStats) setObserverDurations(observerDurations []ObserverDuration) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.observerDurations = observerDurations
}

type withClass interface {
	GetClass() string
}

func observerName(o Observer) string {
	name := fmt.Sprintf("%T", o)
	// durability observers are per class instances.
	if dr, ok := o.(withClass); ok {
		name += fmt.Sprintf("[%s]", dr.GetClass())
	}
	return name
}

var _ Observer = (*SegmentsCountValidation)(nil)
var _ Partial = (*segmentsCountValidationFork)(nil)

// SegmentsCountValidation is an observer that validates the segments count before and after the ranged loop.
type SegmentsCountValidation struct {
	log            *zap.Logger
	mb             *metabase.DB
	checkTimestamp time.Time
	staleInterval  time.Duration

	runTimestamp time.Time
	skipped      bool
	counted      bool
	initialStats metabase.SegmentsStats

	processedSegments map[string]int64
}

// NewSegmentsCountValidation creates a new observer that validates the segments count.
// A non-zero checkTimestamp pins both counts to that time. Otherwise a fresh
// timestamp of now()-staleInterval is derived at the start of every run, so a
// long-lived observer does not keep reading at a timestamp that the database
// has already garbage collected. Without a timestamp, or on a backend that
// cannot count at one, the validation is left out for that run: a live count
// of a table under write traffic differs from itself, let alone from what the
// scan saw.
//
// Start counts the whole segments table at that timestamp, and Finish compares
// the count with what the scan processed. Where the run holds a safepoint, as
// the gc-bf subcommand does on TiDB, the timestamp stays readable for the
// whole scan; otherwise the scan itself fails once the database has garbage
// collected it. On CockroachDB, a legacy backend with limited support, the
// count is a full table scan.
func NewSegmentsCountValidation(log *zap.Logger, mb *metabase.DB, checkTimestamp time.Time, staleInterval time.Duration) *SegmentsCountValidation {
	return &SegmentsCountValidation{
		log:            log,
		mb:             mb,
		checkTimestamp: checkTimestamp,
		staleInterval:  staleInterval,
	}
}

// Start fetches the initial segments count.
func (s *SegmentsCountValidation) Start(ctx context.Context, startTime time.Time) error {
	s.runTimestamp = s.checkTimestamp
	if s.runTimestamp.IsZero() && s.staleInterval > 0 {
		s.runTimestamp = time.Now().Add(-s.staleInterval)
	}
	s.processedSegments = make(map[string]int64)
	s.counted = false

	s.skipped = s.runTimestamp.IsZero() || !ServesFixedReadTimestamp(s.mb.Implementations())
	if s.skipped {
		s.log.Info("leaving out segments count validation, the counts would describe no snapshot",
			zap.Time("check_timestamp", s.runTimestamp))
		return nil
	}

	s.log.Info("starting segments count validation", zap.Time("check_timestamp", s.runTimestamp))

	stats, err := s.mb.CountSegments(ctx, s.runTimestamp)
	if err != nil {
		return Error.Wrap(err)
	}
	s.initialStats = stats
	s.counted = true
	return nil
}

// SegmentsCount is the number of segments the snapshot holds, and whether it
// was counted at all: Start leaves the count out where it would describe no
// snapshot, and does not reach it when the count itself fails.
func (s *SegmentsCountValidation) SegmentsCount() (count int64, counted bool) {
	return s.initialStats.SegmentCount, s.counted
}

// Fork creates a new partial observer for a fork of the ranged loop.
func (s *SegmentsCountValidation) Fork(ctx context.Context) (Partial, error) {
	return &segmentsCountValidationFork{
		count: make(map[string]int64),
	}, nil
}

// Join aggregates the results from a fork of the ranged loop.
func (s *SegmentsCountValidation) Join(ctx context.Context, partial Partial) error {
	countPartial := partial.(*segmentsCountValidationFork)

	for key, value := range countPartial.count {
		s.processedSegments[key] += value
	}
	return nil
}

// Finish compares the initial segments count with the processed segments and
// logs a mismatch, per source. It is advisory: it runs after the observers
// before it in the loop have already published what they produced, so a job
// that has to refuse a short scan gates on PinnedSegmentsCountGuard instead.
func (s *SegmentsCountValidation) Finish(ctx context.Context) error {
	if s.skipped {
		return nil
	}

	var totalProcessed int64
	for _, count := range s.processedSegments {
		totalProcessed += count
	}

	if s.initialStats.SegmentCount != totalProcessed {
		s.log.Warn("segments count validation failed",
			zap.Int64("processed", totalProcessed),
			zap.Any("processed_by_source", s.processedSegments),
			zap.String("initial_stats", fmt.Sprintf("%d %v", s.initialStats.SegmentCount, s.initialStats.PerAdapterSegmentCount)))
	}

	return nil
}

type segmentsCountValidationFork struct {
	count map[string]int64
}

func (s *segmentsCountValidationFork) Process(ctx context.Context, segments []Segment) error {
	// TODO this is not supper efficient but not sure if this code will stay here for long
	for _, segment := range segments {
		s.count[segment.Source]++
	}
	return nil
}
