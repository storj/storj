// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package flightrecorder

import (
	"sort"
	"sync/atomic"
)

// CircularBuffer implements a lock-free ring buffer for storing events.
// It uses a single atomic "write" counter to determine which slot to use (modulo capacity).
// The entire fixed buffer is read in Dump, discarding any zero (unused) events.
type CircularBuffer struct {
	buffer   []Event
	slots    []atomic.Bool
	capacity int
	write    atomic.Uint64
}

// NewCircularBuffer creates a new CircularBuffer with the given capacity and maximum stack frames per event.
func NewCircularBuffer(capacity int) *CircularBuffer {
	return &CircularBuffer{
		buffer:   make([]Event, capacity),
		slots:    make([]atomic.Bool, capacity),
		capacity: capacity,
	}
}

// Enqueue adds a new event into the circular buffer in a lock-free manner.
// It claims the next slot using the atomic write pointer. If the slot is already
// acquired by another writer, the event is discarded and the function returns false.
func (cb *CircularBuffer) Enqueue(event Event) {
	// Atomically claim the next slot.
	// atomic.Uint64.Add returns the new value; subtract 1 to get the previous value.
	newW := cb.write.Add(1) - 1
	slot := int(newW % uint64(cb.capacity))

	// Try to acquire the slot guard immediately.
	// If we cannot acquire the slot, discard the event.
	if !cb.slots[slot].CompareAndSwap(false, true) {
		return
	}

	cb.buffer[slot] = event
	cb.slots[slot].Store(false)
}

// Dump returns a slice of events in the buffer in chronological order (oldest first).
func (cb *CircularBuffer) Dump() []Event {
	return cb.DumpTo(make([]Event, 0, cb.capacity))
}

// DumpTo appends events to the provided 'dst' slice in chronological order (oldest first).
// It iterates over the entire fixed buffer, acquiring each slot to safely read the event,
// and discards events that are zero (unused).
func (cb *CircularBuffer) DumpTo(dst []Event) []Event {
	for i := range cb.buffer {
		for !cb.slots[i].CompareAndSwap(false, true) {
			// spin until acquired.
		}

		ev := cb.buffer[i]
		cb.slots[i].Store(false)

		if !ev.IsZero() {
			dst = append(dst, ev)
		}
	}

	sort.Slice(dst, func(i, j int) bool {
		return dst[i].Timestamp < dst[j].Timestamp
	})

	return dst
}
