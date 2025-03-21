// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package flightrecorder_test

import (
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
