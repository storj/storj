// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"fmt"
	"sync"

	"github.com/spacemonkeygo/monkit/v3"
)

func sendObserverDurations(observerDurations []ObserverDuration) {
	initCompletedObserverStats()
	completedObserverStatsInstance.setObserverDurations(observerDurations)
}

// completedObserverStatsInstance is initialized once
// so that hopefully there is never more than once object instance per satellite process
// and statistics of different object instances don't clobber each other.
var completedObserverStatsInstance *completedObserverStats

// Implements monkit.StatSource.
// Reports the duration per observer from the last completed run of the ranged segment loop.
type completedObserverStats struct {
	mu                sync.Mutex
	observerDurations []ObserverDuration
}

func initCompletedObserverStats() {
	if completedObserverStatsInstance != nil {
		return
	}

	completedObserverStatsInstance = &completedObserverStats{
		observerDurations: []ObserverDuration{},
	}

	// wire statistics up with monkit
	mon.Chain(completedObserverStatsInstance)
}

// Stats implements monkit.StatSource to send the observer durations every time monkit is polled externally.
func (o *completedObserverStats) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// if there are no completed observers yet, no statistics will be sent
	for _, observerDuration := range o.observerDurations {
		key := monkit.NewSeriesKey("completed-observer-duration")
		key = key.WithTag("observer", fmt.Sprintf("%T", observerDuration.Observer))

		cb(key, "duration", observerDuration.Duration.Seconds())
	}
}

// setObserverDurations sets the observer durations to report at ranged segment loop completion.
func (o *completedObserverStats) setObserverDurations(observerDurations []ObserverDuration) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.observerDurations = observerDurations
}
