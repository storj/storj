// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package flightrecorder_test

import (
	"fmt"
	"sync"
	"sync/atomic"
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

func BenchmarkCircularBuffer(b *testing.B) {
	sizes := []int{200, 1000, 2000}

	b.Run("enqueue sequential", func(b *testing.B) {
		for _, size := range sizes {
			b.Run(fmt.Sprintf("capacity=%d", size), func(b *testing.B) {
				cb := flightrecorder.NewCircularBuffer(size)
				evt := flightrecorder.NewEvent(flightrecorder.EventTypeDB, 0)

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					cb.Enqueue(evt)
				}
			})
		}
	})

	b.Run("enqueue parallel", func(b *testing.B) {
		for _, size := range sizes {
			b.Run(fmt.Sprintf("capacity=%d", size), func(b *testing.B) {
				cb := flightrecorder.NewCircularBuffer(size)
				evt := flightrecorder.NewEvent(flightrecorder.EventTypeDB, 0)

				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						cb.Enqueue(evt)
					}
				})
			})
		}
	})

	b.Run("dump empty", func(b *testing.B) {
		for _, size := range sizes {
			b.Run(fmt.Sprintf("capacity=%d", size), func(b *testing.B) {
				cb := flightrecorder.NewCircularBuffer(size)

				dst := make([]flightrecorder.Event, 0, size)

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					dst = cb.DumpTo(dst[:0])
				}
			})
		}
	})

	b.Run("dump full", func(b *testing.B) {
		for _, size := range sizes {
			b.Run(fmt.Sprintf("capacity=%d", size), func(b *testing.B) {
				cb := flightrecorder.NewCircularBuffer(size)
				evt := flightrecorder.NewEvent(flightrecorder.EventTypeDB, 0)

				for i := 0; i < size; i++ {
					cb.Enqueue(evt)
				}

				dst := make([]flightrecorder.Event, 0, size)

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					dst = cb.DumpTo(dst[:0])
				}
			})
		}
	})

	b.Run("dump half full", func(b *testing.B) {
		for _, size := range sizes {
			b.Run(fmt.Sprintf("capacity=%d", size), func(b *testing.B) {
				cb := flightrecorder.NewCircularBuffer(size)
				evt := flightrecorder.NewEvent(flightrecorder.EventTypeDB, 0)

				for i := 0; i < size/2; i++ {
					cb.Enqueue(evt)
				}

				dst := make([]flightrecorder.Event, 0, size)

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					dst = cb.DumpTo(dst[:0])
				}
			})
		}
	})

	b.Run("enqueue/dump concurrent", func(b *testing.B) {
		for _, size := range sizes {
			b.Run(fmt.Sprintf("capacity=%d", size), func(b *testing.B) {
				cb := flightrecorder.NewCircularBuffer(size)
				evt := flightrecorder.NewEvent(flightrecorder.EventTypeDB, 0)
				var counter uint32

				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					// Half of goroutines enqueue, half dump.
					isEnqueuer := atomic.AddUint32(&counter, 1)%2 == 0

					dst := make([]flightrecorder.Event, 0, size)

					for pb.Next() {
						if isEnqueuer {
							cb.Enqueue(evt)
						} else {
							cb.DumpTo(dst[:0])
						}
					}
				})
			})
		}
	})

	b.Run("high contention enqueue/dump", func(b *testing.B) {
		capacity := 10
		cb := flightrecorder.NewCircularBuffer(capacity)
		evt := flightrecorder.NewEvent(flightrecorder.EventTypeDB, 0)

		numWorkers := 8
		opsPerWorker := 1000

		dst := make([]flightrecorder.Event, 0, capacity)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup
			wg.Add(numWorkers)

			for k := 0; k < numWorkers; k++ {
				go func() {
					defer wg.Done()

					for n := 0; n < opsPerWorker; n++ {
						cb.Enqueue(evt)
						cb.DumpTo(dst[:0])
					}
				}()
			}
			wg.Wait()
		}
	})
}
