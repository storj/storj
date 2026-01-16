// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// Package eventkitspy adds an eventkit.Destination to DefaultRegistry,
// that can be used for testing purposes.
//
// The test that uses eventkitspy should be non-parallel.
package eventkitspy

import (
	"context"
	"sync"

	"storj.io/eventkit"
)

var defaultDestination = NewDestination(100)

// racecheck ensures that the default destination is not used concurrently. When
// there's a concurrent write, then the race detector will detect it and report
// an error.
//
// When you see this in a test, it means that the test was not marked as
// non-parallel or the Clear/GetEvents were called in a goroutine that didn't
// have sufficient synchronization.
var racecheck int

func init() {
	_ = racecheck // ignore staticcheck warning
	eventkit.DefaultRegistry.AddDestination(defaultDestination)
}

// Clear clears the recorded events.
func Clear() {
	racecheck++
	defaultDestination.Clear()
}

// GetEvents returns a copy of the recorded events.
func GetEvents() []*eventkit.Event {
	racecheck++
	return defaultDestination.GetEvents()
}

var _ eventkit.Destination = &Destination{}

// Destination is a eventkit.Destination that collects all the submitted events for testing.
type Destination struct {
	limit int

	mu     sync.Mutex
	read   int
	write  int
	events []*eventkit.Event
}

// NewDestination creates a new Destination with the given limit.
func NewDestination(limit int) *Destination {
	limit++ // to account for the read/write head position
	return &Destination{
		limit:  limit,
		events: make([]*eventkit.Event, limit),
	}
}

// Clear clears the recorded events.
func (m *Destination) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.read = 0
	m.write = 0
}

// Submit records the submitted events.
func (m *Destination) Submit(events ...*eventkit.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ev := range events {
		m.add(ev)
	}
}

// add adds an event to the destination.
func (m *Destination) add(event *eventkit.Event) {
	m.events[m.write] = event
	m.write = (m.write + 1) % m.limit
	if m.write == m.read {
		m.read = (m.read + 1) % m.limit
	}
}

// Run is a no-op for the mock destination.
func (m *Destination) Run(_ context.Context) {}

// GetEvents returns a copy of the recorded events.
func (m *Destination) GetEvents() []*eventkit.Event {
	m.mu.Lock()
	defer m.mu.Unlock()

	var count int
	if m.write >= m.read {
		count = m.write - m.read
	} else {
		count = m.limit - m.read + m.write
	}

	result := make([]*eventkit.Event, count)
	for i := 0; i < count; i++ {
		idx := (m.read + i) % m.limit
		result[i] = m.events[idx]
	}
	return result
}
