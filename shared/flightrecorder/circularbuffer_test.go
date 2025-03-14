// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package flightrecorder_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/flightrecorder"
)

func TestCircularBuffer(t *testing.T) {
	t.Run("enqueue and dump basic", func(t *testing.T) {
		capacity := 5
		cb := flightrecorder.NewCircularBuffer(capacity)

		for i := 1; i <= capacity; i++ {
			ev := flightrecorder.NewEvent(flightrecorder.EventTypeDB, 0)
			cb.Enqueue(ev)
		}

		events := cb.Dump()
		require.Equal(t, capacity, len(events))

		// Verify events are sorted by timestamp.
		for i := 1; i < len(events); i++ {
			require.LessOrEqual(t, events[i-1].Timestamp, events[i].Timestamp)
		}
	})

	t.Run("overwrite", func(t *testing.T) {
		capacity := 3
		cb := flightrecorder.NewCircularBuffer(capacity)

		// Enqueue more events than the capacity.
		totalEvents := 5
		for i := 1; i <= totalEvents; i++ {
			ev := flightrecorder.Event{
				Timestamp: uint64(1000 + i),
				Type:      flightrecorder.EventTypeDB,
			}
			for j := 0; j < len(ev.Stack); j++ {
				ev.Stack[j] = uintptr(i*10 + j)
			}

			cb.Enqueue(ev)
		}

		events := cb.Dump()
		require.Equal(t, capacity, len(events))

		// Expect only the most recent events (events 3, 4, 5).
		expectedTimestamps := []uint64{1003, 1004, 1005}
		for i, ev := range events {
			require.Equal(t, expectedTimestamps[i], ev.Timestamp)
		}
	})

	t.Run("concurrent enqueue", func(t *testing.T) {
		capacity := 10
		cb := flightrecorder.NewCircularBuffer(capacity)

		totalGoroutines := 20
		enqueuesPerGoroutine := 100

		var wg sync.WaitGroup
		wg.Add(totalGoroutines)

		for i := 0; i < totalGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < enqueuesPerGoroutine; j++ {
					ev := flightrecorder.NewEvent(flightrecorder.EventTypeDB, 0)
					cb.Enqueue(ev)
				}
			}(i)
		}
		wg.Wait()

		events := cb.Dump()

		// When overfilled, our design uses a single write pointer so the Dump iterates over the whole buffer.
		// In the common case, the buffer is full. So we expect exactly 'capacity' non-zero events.
		require.Equal(t, capacity, len(events))
		for _, ev := range events {
			require.False(t, ev.IsZero())
		}
	})

	t.Run("concurrent enqueue and dump", func(t *testing.T) {
		capacity := 10
		cb := flightrecorder.NewCircularBuffer(capacity)
		totalEnqueues := 500
		numEnqueuers := 10
		numDumpers := 5
		var wg sync.WaitGroup

		// Channel to collect intermediate dump results.
		dumpsCh := make(chan []flightrecorder.Event, numDumpers*(totalEnqueues/10))

		wg.Add(numEnqueuers)
		for i := 0; i < numEnqueuers; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < totalEnqueues; j++ {
					ev := flightrecorder.NewEvent(flightrecorder.EventTypeDB, 0)
					cb.Enqueue(ev)

					// Introduce a small sleep to force more interleaving.
					time.Sleep(time.Millisecond)
				}
			}(i)
		}

		wg.Add(numDumpers)
		for i := 0; i < numDumpers; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < totalEnqueues/10; j++ {
					events := cb.Dump()
					require.LessOrEqual(t, len(events), capacity)

					for k := 1; k < len(events); k++ {
						require.LessOrEqual(t, events[k-1].Timestamp, events[k].Timestamp)
					}
					for _, ev := range events {
						require.False(t, ev.IsZero())
					}

					dumpsCh <- events
					time.Sleep(2 * time.Millisecond)
				}
			}()
		}

		wg.Wait()
		close(dumpsCh)

		for events := range dumpsCh {
			require.LessOrEqual(t, len(events), capacity)

			for i := 1; i < len(events); i++ {
				require.LessOrEqual(t, events[i-1].Timestamp, events[i].Timestamp)
			}
		}

		finalEvents := cb.Dump()
		require.Equal(t, capacity, len(finalEvents))

		for i := 1; i < len(finalEvents); i++ {
			require.LessOrEqual(t, finalEvents[i-1].Timestamp, finalEvents[i].Timestamp)
		}
	})
}
