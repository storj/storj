// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/eventing"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil"
)

func TestTiDBEventSource(t *testing.T) {
	metabasetest.RunWithConfig(t, metabase.Config{}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		if db.Implementation() != dbutil.TiDB {
			t.Skip("test requires TiDB")
		}

		adapter := db.ChooseAdapter(testrand.UUID()).(*metabase.TiDBAdapter)

		projectID := testrand.UUID()
		streamID := testrand.UUID()

		insertRow := func(t *testing.T, bucketName, objectKey string, eventName string) {
			t.Helper()
			require.NoError(t, adapter.TestingInsertBucketEvent(ctx, metabase.BucketEvent{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  projectID,
					BucketName: metabase.BucketName(bucketName),
					ObjectKey:  metabase.ObjectKey(objectKey),
					Version:    1,
					StreamID:   streamID,
				},
				TotalPlainSize: 100,
				EventName:      eventName,
			}))
		}

		countRows := func(t *testing.T) int {
			t.Helper()
			count, err := adapter.TestingCountBucketEvents(ctx)
			require.NoError(t, err)
			return count
		}

		t.Run("publishes and deletes rows", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			insertRow(t, "bucket1", "key1", eventing.EventNameObjectCreatedPut)
			insertRow(t, "bucket1", "key2", eventing.EventNameObjectRemovedDelete)
			insertRow(t, "bucket2", "key3", eventing.EventNameObjectCreatedCopy)

			source := eventing.NewTiDBEventSource(zap.NewNop(), adapter, 10*time.Millisecond, 10)

			cancelCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			done := make(chan error, 1)
			ctx.Go(func() error {
				done <- source.Listen(cancelCtx, func(event eventing.ChangeEvent) (eventing.PendingResult, error) {
					return eventing.ImmediateResult(event.CommitTimestamp), nil
				})
				return nil
			})

			// All rows should be deleted after confirmation.
			require.Eventually(t, func() bool {
				return countRows(t) == 0
			}, 2*time.Second, 50*time.Millisecond)

			cancel()

			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for Listen to stop")
			}
		})

		t.Run("decodes fields correctly", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			insertRow(t, "my-bucket", "my/key", eventing.EventNameObjectCreatedPut)

			source := eventing.NewTiDBEventSource(zap.NewNop(), adapter, 10*time.Millisecond, 10)

			cancelCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			done := make(chan error, 1)
			var got eventing.ChangeEvent
			ctx.Go(func() error {
				done <- source.Listen(cancelCtx, func(event eventing.ChangeEvent) (eventing.PendingResult, error) {
					got = event
					return eventing.ImmediateResult(event.CommitTimestamp), nil
				})
				return nil
			})

			require.Eventually(t, func() bool {
				return countRows(t) == 0
			}, 2*time.Second, 50*time.Millisecond)

			cancel()

			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for Listen to stop")
			}

			require.Equal(t, projectID, got.ProjectID)
			require.Equal(t, metabase.BucketName("my-bucket"), got.BucketName)
			require.Equal(t, metabase.ObjectKey("my/key"), got.ObjectKey)
			require.Equal(t, streamID, got.StreamID)
			require.Equal(t, metabase.Version(1), got.Version)
			require.Equal(t, int64(100), got.TotalPlainSize)
			require.Equal(t, eventing.EventNameObjectCreatedPut, got.EventName)
		})
	})
}

func BenchmarkTiDBEventSource(b *testing.B) {
	metabasetest.Bench(b, func(ctx *testcontext.Context, b *testing.B, db *metabase.DB) {
		if db.Implementation() != dbutil.TiDB {
			b.Skip("benchmark requires TiDB")
		}

		adapter := db.ChooseAdapter(testrand.UUID()).(*metabase.TiDBAdapter)

		projectID := testrand.UUID()
		streamID := testrand.UUID()

		insertRows := func(b *testing.B, n int) {
			b.Helper()
			for i := range n {
				require.NoError(b, adapter.TestingInsertBucketEvent(ctx, metabase.BucketEvent{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  projectID,
						BucketName: "bench-bucket",
						ObjectKey:  metabase.ObjectKey(fmt.Appendf(nil, "key-%d", i)),
						Version:    1,
						StreamID:   streamID,
					},
					TotalPlainSize: 100,
					EventName:      eventing.EventNameObjectCreatedPut,
				}))
			}
		}

		countRows := func(b *testing.B) int {
			b.Helper()
			count, err := adapter.TestingCountBucketEvents(ctx)
			require.NoError(b, err)
			return count
		}

		for _, batchSize := range []int{10, 100, 1000} {
			b.Run(fmt.Sprintf("batch=%d", batchSize), func(b *testing.B) {
				totalEvents := batchSize * 10
				fn := func(event eventing.ChangeEvent) (eventing.PendingResult, error) {
					return eventing.ImmediateResult(event.CommitTimestamp), nil
				}

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					metabasetest.DeleteAll{}.Check(ctx, b, db)
					insertRows(b, totalEvents)
					b.StartTimer()

					source := eventing.NewTiDBEventSource(zap.NewNop(), adapter, time.Millisecond, batchSize)
					cancelCtx, cancel := context.WithCancel(ctx)
					done := make(chan error, 1)
					go func() { done <- source.Listen(cancelCtx, fn) }()

					require.Eventually(b, func() bool { return countRows(b) == 0 }, 30*time.Second, time.Millisecond)
					cancel()
					<-done
				}

				b.ReportMetric(float64(totalEvents), "events/op")
			})
		}
	})
}
