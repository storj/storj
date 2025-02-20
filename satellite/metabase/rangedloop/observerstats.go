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

var _ Observer = (*segmentsCountValidation)(nil)
var _ Partial = (*segmentsCountValidationFork)(nil)

type segmentsCountValidation struct {
	log            *zap.Logger
	mb             *metabase.DB
	checkTimestamp time.Time

	initialStats metabase.SegmentsStats

	processedSegments map[string]int64
}

// NewSegmentsCountValidation creates a new observer that validates the segments count.
func NewSegmentsCountValidation(log *zap.Logger, mb *metabase.DB, checkTimestamp time.Time) Observer {
	return &segmentsCountValidation{
		log:               log,
		mb:                mb,
		checkTimestamp:    checkTimestamp,
		processedSegments: make(map[string]int64),
	}
}

func (s *segmentsCountValidation) Start(ctx context.Context, startTime time.Time) error {
	s.log.Info("starting segments count validation", zap.Time("check timestamp", s.checkTimestamp))

	stats, err := s.mb.CountSegments(ctx, s.checkTimestamp)
	if err != nil {
		return Error.Wrap(err)
	}
	s.initialStats = stats
	return nil
}

func (s *segmentsCountValidation) Fork(ctx context.Context) (Partial, error) {
	return &segmentsCountValidationFork{
		count: make(map[string]int64),
	}, nil
}

func (s *segmentsCountValidation) Join(ctx context.Context, partial Partial) error {
	countPartial := partial.(*segmentsCountValidationFork)

	for key, value := range countPartial.count {
		s.processedSegments[key] += value
	}
	return nil
}

func (s *segmentsCountValidation) Finish(ctx context.Context) error {
	finalStats, err := s.mb.CountSegments(ctx, s.checkTimestamp)
	if err != nil {
		return Error.Wrap(err)
	}

	var totalProcessed int64
	for _, count := range s.processedSegments {
		totalProcessed += count
	}

	if s.initialStats.SegmentCount != finalStats.SegmentCount || s.initialStats.SegmentCount != totalProcessed {
		s.log.Warn("segments count validation failed",
			zap.Int64("processed", totalProcessed),
			zap.Any("processed by source", s.processedSegments),
			zap.String("initial stats", fmt.Sprintf("%d %v", s.initialStats.SegmentCount, s.initialStats.PerAdapterSegmentCount)),
			zap.String("final stats", fmt.Sprintf("%d %v", finalStats.SegmentCount, finalStats.PerAdapterSegmentCount)))
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
