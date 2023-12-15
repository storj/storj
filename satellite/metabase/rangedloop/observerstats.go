// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"fmt"
	"sync"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/eventkit"
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
