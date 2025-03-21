// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package flightrecorder_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"storj.io/storj/shared/flightrecorder"
)

func TestFlightRecorder(t *testing.T) {
	baseConfig := flightrecorder.Config{
		DBStackFrameCapacity: 5,
	}

	mergeOptions := []bool{true, false}

	t.Run("Enqueue less than capacity", func(t *testing.T) {
		for _, merge := range mergeOptions {
			t.Run(fmt.Sprintf("merge=%v", merge), func(t *testing.T) {
				cfg := baseConfig
				core, observedLogs := observer.New(zap.DebugLevel)
				logger := zap.New(core)
				recorder := flightrecorder.NewBox(logger, cfg)

				numEvents := 3
				for i := 0; i < numEvents; i++ {
					simulateDBQuery(recorder)
				}
				recorder.DumpAndReset(merge)

				debugEntries := filterEntriesByLevel(observedLogs.All(), zap.DebugLevel)
				require.Equal(t, numEvents, len(debugEntries))

				for _, entry := range debugEntries {
					ctxMap := entry.ContextMap()
					stackStr, ok := ctxMap["stack"].(string)
					require.True(t, ok)
					require.NotEmpty(t, stackStr)
					require.Equal(t, flightrecorder.EventTypeDB.String(), ctxMap["type"])
				}

				// When merging, check that the events are sorted by timestamp.
				if merge {
					var prev uint64
					for _, entry := range debugEntries {
						timestamp, ok := entry.ContextMap()["timestamp"].(uint64)
						require.True(t, ok)
						if prev != 0 {
							require.LessOrEqual(t, prev, timestamp)
						}
						prev = timestamp
					}
				}
			})
		}
	})

	t.Run("Enqueue wrap around", func(t *testing.T) {
		for _, merge := range mergeOptions {
			t.Run(fmt.Sprintf("merge=%v", merge), func(t *testing.T) {
				cfg := baseConfig
				core, observedLogs := observer.New(zap.DebugLevel)
				logger := zap.New(core)
				recorder := flightrecorder.NewBox(logger, cfg)

				totalEvents := 7 // more than capacity (5)
				for i := 1; i <= totalEvents; i++ {
					simulateDBQuery(recorder)
				}
				recorder.DumpAndReset(merge)

				debugEntries := filterEntriesByLevel(observedLogs.All(), zap.DebugLevel)
				require.Equal(t, cfg.DBStackFrameCapacity, len(debugEntries))
			})
		}
	})

	t.Run("FormattedStack does not exceed fixed frame count", func(t *testing.T) {
		for _, merge := range mergeOptions {
			t.Run(fmt.Sprintf("merge=%v", merge), func(t *testing.T) {
				cfg := baseConfig
				core, observedLogs := observer.New(zap.DebugLevel)
				logger := zap.New(core)
				recorder := flightrecorder.NewBox(logger, cfg)

				simulateDBQuery(recorder)
				recorder.DumpAndReset(merge)

				debugEntries := filterEntriesByLevel(observedLogs.All(), zap.DebugLevel)
				require.Equal(t, 1, len(debugEntries))

				ctxMap := debugEntries[0].ContextMap()
				stackStr, ok := ctxMap["stack"].(string)
				require.True(t, ok)

				// Each valid frame prints two lines; since Event.Stack is [8]uintptr, we expect at most 8 frames.
				numLines := len(strings.Split(strings.TrimSpace(stackStr), "\n"))
				numFrames := numLines / 2
				require.LessOrEqual(t, numFrames, 8)
			})
		}
	})

	t.Run("Concurrent enqueues", func(t *testing.T) {
		for _, merge := range mergeOptions {
			t.Run(fmt.Sprintf("merge=%v", merge), func(t *testing.T) {
				cfg := baseConfig
				cfg.DBStackFrameCapacity = 10
				core, observedLogs := observer.New(zap.DebugLevel)
				logger := zap.New(core)
				recorder := flightrecorder.NewBox(logger, cfg)

				totalGoroutines := 20
				enqueuesPerGoroutine := 100
				var wg sync.WaitGroup
				wg.Add(totalGoroutines)
				for i := 0; i < totalGoroutines; i++ {
					go func() {
						defer wg.Done()
						for j := 0; j < enqueuesPerGoroutine; j++ {
							simulateDBQuery(recorder)
						}
					}()
				}
				wg.Wait()
				recorder.DumpAndReset(merge)

				debugEntries := filterEntriesByLevel(observedLogs.All(), zap.DebugLevel)
				require.Equal(t, cfg.DBStackFrameCapacity, len(debugEntries))
			})
		}
	})

	t.Run("Unknown event type logs warning", func(t *testing.T) {
		for _, merge := range mergeOptions {
			t.Run(fmt.Sprintf("merge=%v", merge), func(t *testing.T) {
				cfg := baseConfig
				core, observedLogs := observer.New(zap.DebugLevel)
				logger := zap.New(core)
				recorder := flightrecorder.NewBox(logger, cfg)

				simulateDBQuery(recorder)
				recorder.Enqueue(flightrecorder.EventType(999), 0)
				recorder.DumpAndReset(merge)

				allEntries := observedLogs.All()
				debugEntries := filterEntriesByLevel(allEntries, zap.DebugLevel)
				require.Equal(t, 1, len(debugEntries))

				ctxMap := debugEntries[0].ContextMap()
				require.Equal(t, flightrecorder.EventTypeDB.String(), ctxMap["type"])

				// There should be at least one warning about the missing buffer.
				warnEntries := filterEntriesByLevel(allEntries, zap.WarnLevel)
				require.Greater(t, len(warnEntries), 0)

				found := false
				for _, entry := range warnEntries {
					ctx := entry.ContextMap()
					if ctx["eventType"] == "Unknown" {
						found = true
						break
					}
				}
				require.True(t, found)
			})
		}
	})
}

// simulateDBQuery is a helper to simulate a DB query that triggers an event.
func simulateDBQuery(recorder *flightrecorder.Box) {
	recorder.Enqueue(flightrecorder.EventTypeDB, 0)
}

// filterEntriesByLevel filters logged entries by zap level.
func filterEntriesByLevel(entries []observer.LoggedEntry, level zapcore.Level) []observer.LoggedEntry {
	var filtered []observer.LoggedEntry
	for _, entry := range entries {
		if entry.Level == level {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
