// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventkit

import (
	"context"
	"sync"

	"storj.io/eventkit"
)

// MockEventkitDestination is a mock implementation of eventkit.Destination for testing.
// Implementation copied from storj.io/eventkit/destination/batch_test.go
type MockEventkitDestination struct {
	mu     sync.Mutex
	events []*eventkit.Event
}

// Submit records the submitted events.
func (m *MockEventkitDestination) Submit(events ...*eventkit.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, events...)
}

// Run is a no-op for the mock destination.
func (m *MockEventkitDestination) Run(_ context.Context) {}

// GetEvents returns a copy of the recorded events.
func (m *MockEventkitDestination) GetEvents() []*eventkit.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to avoid race conditions
	result := make([]*eventkit.Event, len(m.events))
	copy(result, m.events)
	return result
}

var _ eventkit.Destination = &MockEventkitDestination{}
