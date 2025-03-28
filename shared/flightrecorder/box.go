// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package flightrecorder

import (
	"sort"
	"sync/atomic"

	"go.uber.org/zap"
)

// Box holds multiple circular buffers for recording events, one per event type.
// It also provides a mechanism to swap out active buffers
// so that a DumpAndReset operation does not block new events.
type Box struct {
	buffers atomic.Pointer[[EventTypeCount]*CircularBuffer]
	log     *zap.Logger
	config  Config
}

// NewBox creates a new Box based on the provided configuration.
// It initializes buffers for each supported event type.
func NewBox(log *zap.Logger, config Config) *Box {
	box := &Box{
		log:    log,
		config: config,
	}

	box.buffers.Store(box.newBuffers())

	return box
}

// Enqueue captures the call stack and enqueues an event for the given event type.
// The skipCallers parameter allows the caller to skip a number of stack frames.
func (b *Box) Enqueue(eventType EventType, skipCallers int) {
	buffers := b.buffers.Load()

	if int(eventType) >= len(*buffers) {
		b.log.Warn("Invalid event type", zap.String("eventType", eventType.String()))
		return
	}

	cb := buffers[eventType]
	if cb == nil {
		b.log.Warn("No buffer for event type", zap.String("eventType", eventType.String()))
		return
	}

	cb.Enqueue(NewEvent(eventType, skipCallers+1)) // +1 to skip the Enqueue call itself.
}

// Reset replaces the active buffers with new ones and returns the old ones.
// This ensures DumpAndReset reads from a stable snapshot while new events are recorded.
func (b *Box) Reset() *[EventTypeCount]*CircularBuffer {
	next := b.newBuffers()
	old := b.buffers.Swap(next)

	return old
}

// DumpAndReset swaps out active buffers and logs all events from the old buffers.
// It sorts events by timestamp and uses zap.Logger to output the details.
func (b *Box) DumpAndReset(mergeEventDumps bool) {
	oldBuffers := b.Reset()

	if mergeEventDumps {
		var allEvents []Event
		for _, buf := range *oldBuffers {
			if buf != nil {
				allEvents = buf.DumpTo(allEvents)
			}
		}

		sort.Slice(allEvents, func(i, j int) bool {
			return allEvents[i].Timestamp < allEvents[j].Timestamp
		})

		b.logEvents(allEvents)

		return
	}

	for _, buf := range *oldBuffers {
		if buf != nil {
			b.logEvents(buf.Dump())
		}
	}
}

// newBuffers creates a new set of circular buffers for the Box.
func (b *Box) newBuffers() *[EventTypeCount]*CircularBuffer {
	buffers := &[EventTypeCount]*CircularBuffer{}
	buffers[EventTypeDB] = NewCircularBuffer(b.config.DBStackFrameCapacity)

	return buffers
}

// logEvents logs each event using the Box's logger.
func (b *Box) logEvents(events []Event) {
	for _, event := range events {
		b.log.Debug("flight recorder event",
			zap.Uint64("timestamp", event.Timestamp),
			zap.String("type", event.Type.String()),
			zap.String("stack", event.FormattedStack()))
	}
}
