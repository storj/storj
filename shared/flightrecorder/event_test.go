// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package flightrecorder_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/flightrecorder"
)

func TestEvent(t *testing.T) {
	t.Run("is zero", func(t *testing.T) {
		var ev flightrecorder.Event
		require.True(t, ev.IsZero())

		ev.Timestamp = uint64(time.Now().UnixNano())
		require.False(t, ev.IsZero())
	})

	t.Run("formatted stack", func(t *testing.T) {
		ev := flightrecorder.NewEvent(flightrecorder.EventTypeDB, -1)
		formatted := ev.FormattedStack()
		require.NotEmpty(t, formatted)
		require.Contains(t, formatted, "NewEvent")
	})

	t.Run("formatted stack empty", func(t *testing.T) {
		ev := flightrecorder.Event{
			Timestamp: uint64(time.Now().UnixNano()),
			Type:      flightrecorder.EventTypeDB,
		}

		formatted := ev.FormattedStack()
		require.Empty(t, strings.TrimSpace(formatted))
	})
}

// This variable needs to be at package level
// to ensure the compiler can't prove it's unused.
var sinkEvent flightrecorder.Event

func BenchmarkNewEvent(b *testing.B) {
	b.Run("NewEvent", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkEvent = flightrecorder.NewEvent(flightrecorder.EventTypeDB, 0)
		}
	})

	for _, skipFrames := range []int{1, 3, 5} {
		skipFrames := skipFrames
		name := fmt.Sprintf("NewEvent/skipCallers=%d", skipFrames)

		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				sinkEvent = flightrecorder.NewEvent(flightrecorder.EventTypeDB, skipFrames)
			}
		})
	}
}
