// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package flightrecorder

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// EventType defines the type of event.
type EventType int

const (
	// EventTypeDB represents a database event.
	EventTypeDB EventType = iota

	// EventTypeCount represents the total number of event types.
	// Always keep this last in the const block.
	EventTypeCount
)

// String returns the string representation of the EventType.
func (et EventType) String() string {
	switch et {
	case EventTypeDB:
		return "DB"
	default:
		return "Unknown"
	}
}

// Event represents a single recorded event.
// Timestamp is stored as a uint64 (Unix nanosecond timestamp) to minimize size.
// Stack is stored as a fixed-size array of uintptr, ensuring constant memory usage.
type Event struct {
	Timestamp   uint64
	Stack       [8]uintptr
	Type        EventType
	SkipCallers int
}

// NewEvent creates a new event with the given type and stack depth.
func NewEvent(eventType EventType, skipCallers int) Event {
	ev := Event{
		Timestamp: uint64(time.Now().UnixNano()),
		Type:      eventType,
	}

	runtime.Callers(skipCallers+2, ev.Stack[:]) // +2 to skip the runtime.Callers call itself and NewEvent.

	return ev
}

// IsZero returns true if the event is not set.
func (e *Event) IsZero() bool {
	return e.Timestamp == 0
}

// FormattedStack converts the fixed-size stack array into a human-readable string.
func (e *Event) FormattedStack() string {
	frames := runtime.CallersFrames(e.Stack[:])

	var sb strings.Builder
	for {
		frame, more := frames.Next()

		// If frame.Function is empty, assume no more valid frames are present.
		// Unlikely to happen, but still.
		if frame.Function == "" {
			break
		}

		sb.WriteString(fmt.Sprintf("%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line))
		if !more {
			break
		}
	}

	return sb.String()
}
