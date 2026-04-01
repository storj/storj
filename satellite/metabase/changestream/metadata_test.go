// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream_test

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/recordeddb"
	"storj.io/storj/shared/dbutil/spannerutil"
)

// setupMetadataTest creates a metadata table for testing and returns the adapter and cleanup function.
func setupMetadataTest(ctx *testcontext.Context, t *testing.T, db *metabase.DB, feedName string) (
	adapter *metabase.SpannerAdapter,
	cleanup func(),
) {
	if db.Implementation() != dbutil.Spanner {
		t.Skip("test requires Spanner adapter")
	}

	streamId := metabasetest.RandObjectStream()
	dbAdapter := db.ChooseAdapter(streamId.ProjectID)
	spannerAdapter, ok := dbAdapter.(*metabase.SpannerAdapter)
	require.True(t, ok, "adapter should be SpannerAdapter")

	// Clean up any existing table first to ensure fresh schema
	_ = spannerAdapter.TestDeleteChangeStreamMetadata(ctx, feedName)

	err := spannerAdapter.TestCreateChangeStreamMetadata(ctx, feedName)
	require.NoError(t, err)

	cleanup = func() {
		err := spannerAdapter.TestDeleteChangeStreamMetadata(ctx, feedName)
		require.NoError(t, err)
	}

	return spannerAdapter, cleanup
}

// flushState is a helper to buffer a state update and immediately flush it to Spanner.
func flushState(ctx context.Context, t *testing.T, adapter changestream.Adapter, feedName, token string, state changestream.PartitionState) {
	t.Helper()
	b := changestream.NewMetadataBatcher(zap.NewNop(), adapter, feedName)
	b.UpdatePartitionState(state, token)
	require.NoError(t, b.Flush(ctx))
}

// flushWatermark is a helper to buffer a watermark update and immediately flush it to Spanner.
func flushWatermark(ctx context.Context, t *testing.T, adapter changestream.Adapter, feedName, token string, watermark time.Time) {
	t.Helper()
	b := changestream.NewMetadataBatcher(zap.NewNop(), adapter, feedName)
	b.UpdatePartitionWatermark(token, watermark)
	require.NoError(t, b.Flush(ctx))
}

// flushPartition is a helper to buffer a child partition insert and immediately flush it to Spanner.
func flushPartition(ctx context.Context, t *testing.T, adapter changestream.Adapter, feedName, token string, parentTokens []string, start time.Time) {
	t.Helper()
	b := changestream.NewMetadataBatcher(zap.NewNop(), adapter, feedName)
	b.AddChildPartition(token, parentTokens, start)
	require.NoError(t, b.Flush(ctx))
}

func TestNoPartitionMetadata(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		feedName := "test_no_partition_metadata"
		adapter, cleanup := setupMetadataTest(ctx, t, db, feedName)
		defer cleanup()

		t.Run("empty table returns true", func(t *testing.T) {
			empty, err := adapter.ChangeStreamNoPartitionMetadata(ctx, feedName)
			require.NoError(t, err)
			require.True(t, empty, "metadata table should be empty")
		})

		t.Run("non-empty table returns false", func(t *testing.T) {
			// Add a partition
			flushPartition(ctx, t, adapter, feedName, "", nil, time.Now())

			empty, err := adapter.ChangeStreamNoPartitionMetadata(ctx, feedName)
			require.NoError(t, err)
			require.False(t, empty, "metadata table should not be empty")
		})
	})
}

func TestAddChildPartition(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		feedName := "test_add_child_partition"
		adapter, cleanup := setupMetadataTest(ctx, t, db, feedName)
		defer cleanup()

		t.Run("add initial partition with empty token", func(t *testing.T) {
			startTime := time.Now()
			flushPartition(ctx, t, adapter, feedName, "", nil, startTime)

			partition, err := getPartition(ctx, adapter.UnderlyingDB(), feedName, "")
			require.NoError(t, err)
			require.NotNil(t, partition)
			assert.Empty(t, partition.PartitionToken)
			assert.Empty(t, partition.ParentTokens)
			assert.WithinDuration(t, startTime, partition.StartTimestamp, time.Second)
			assert.Equal(t, changestream.StateCreated, partition.State)
			assert.WithinDuration(t, startTime, partition.Watermark, time.Second)
			assert.WithinDuration(t, startTime, partition.CreatedAt, time.Second)
			assert.Nil(t, partition.ScheduledAt)
			assert.Nil(t, partition.RunningAt)
			assert.Nil(t, partition.FinishedAt)
		})

		t.Run("add partition with token and parents", func(t *testing.T) {
			startTime := time.Now()
			token := "test-token-123"
			parentTokens := []string{"parent-1", "parent-2"}

			flushPartition(ctx, t, adapter, feedName, token, parentTokens, startTime)

			partition, err := getPartition(ctx, adapter.UnderlyingDB(), feedName, token)
			require.NoError(t, err)
			require.NotNil(t, partition)
			assert.Equal(t, token, partition.PartitionToken)
			assert.ElementsMatch(t, parentTokens, partition.ParentTokens)
			assert.WithinDuration(t, startTime, partition.StartTimestamp, time.Second)
			assert.Equal(t, changestream.StateCreated, partition.State)
			assert.WithinDuration(t, startTime, partition.Watermark, time.Second)
			assert.WithinDuration(t, startTime, partition.CreatedAt, time.Second)
			assert.Nil(t, partition.ScheduledAt)
			assert.Nil(t, partition.RunningAt)
			assert.Nil(t, partition.FinishedAt)
		})

		t.Run("duplicate partition is idempotent", func(t *testing.T) {
			startTime := time.Now()
			token := "duplicate-token"

			// Add first time
			flushPartition(ctx, t, adapter, feedName, token, nil, startTime)

			// Add second time - should not error (idempotent via InsertOrUpdate)
			flushPartition(ctx, t, adapter, feedName, token, nil, startTime)
		})
	})
}

func TestGetPartitionsByState(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		feedName := "test_get_partitions_by_state"
		adapter, cleanup := setupMetadataTest(ctx, t, db, feedName)
		defer cleanup()

		// Add partitions in different states
		startTime := time.Now()

		// Add some partitions in Created state
		flushPartition(ctx, t, adapter, feedName, "created-1", nil, startTime)
		flushPartition(ctx, t, adapter, feedName, "created-2", nil, startTime)

		// Add partition and update to Scheduled state
		flushPartition(ctx, t, adapter, feedName, "scheduled-1", nil, startTime)
		flushState(ctx, t, adapter, feedName, "scheduled-1", changestream.StateScheduled)

		t.Run("query Created state", func(t *testing.T) {
			partitions, err := adapter.GetChangeStreamPartitionsByState(ctx, feedName, changestream.StateCreated)
			require.NoError(t, err)
			require.Len(t, partitions, 2)
			require.Contains(t, partitions, "created-1")
			require.Contains(t, partitions, "created-2")
		})

		t.Run("query Scheduled state", func(t *testing.T) {
			partitions, err := adapter.GetChangeStreamPartitionsByState(ctx, feedName, changestream.StateScheduled)
			require.NoError(t, err)
			require.Len(t, partitions, 1)
			require.Contains(t, partitions, "scheduled-1")
		})

		t.Run("query state with no matches", func(t *testing.T) {
			partitions, err := adapter.GetChangeStreamPartitionsByState(ctx, feedName, changestream.StateRunning)
			require.NoError(t, err)
			require.Empty(t, partitions)
		})

		t.Run("verify watermark values", func(t *testing.T) {
			partitions, err := adapter.GetChangeStreamPartitionsByState(ctx, feedName, changestream.StateCreated)
			require.NoError(t, err)
			for _, watermark := range partitions {
				require.WithinDuration(t, startTime, watermark, time.Second)
			}
		})
	})
}

func TestSchedulePartitions(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		feedName := "test_schedule_partitions"
		adapter, cleanup := setupMetadataTest(ctx, t, db, feedName)
		defer cleanup()

		t.Run("schedule initial partition", func(t *testing.T) {
			// Add initial partition with empty token
			flushPartition(ctx, t, adapter, feedName, "", nil, time.Now())

			// Schedule partitions
			count, err := adapter.ScheduleChangeStreamPartitions(ctx, feedName)
			require.NoError(t, err)
			require.Equal(t, int64(1), count, "should schedule initial partition")

			// Verify initial partition is scheduled
			partition, err := getPartition(ctx, adapter.UnderlyingDB(), feedName, "")
			require.NoError(t, err)
			require.NotNil(t, partition)
			assert.Equal(t, changestream.StateScheduled, partition.State)
			require.NotNil(t, partition.ScheduledAt)
			assert.WithinDuration(t, time.Now(), *partition.ScheduledAt, time.Second)
		})

		t.Run("schedule children of initial partition after initial finishes", func(t *testing.T) {
			// Add children with no parents
			flushPartition(ctx, t, adapter, feedName, "child-1", nil, time.Now())
			// Add another child, but with empty parent list instead of nil
			flushPartition(ctx, t, adapter, feedName, "child-2", []string{}, time.Now())

			// Try to schedule - should not schedule yet because initial is not finished
			count, err := adapter.ScheduleChangeStreamPartitions(ctx, feedName)
			require.NoError(t, err)
			require.Equal(t, int64(0), count, "should not schedule children yet")

			// Mark initial partition as finished
			flushState(ctx, t, adapter, feedName, "", changestream.StateFinished)

			// Now schedule should work
			count, err = adapter.ScheduleChangeStreamPartitions(ctx, feedName)
			require.NoError(t, err)
			require.Equal(t, int64(2), count, "should schedule both children")

			// Verify children are scheduled
			child1, err := getPartition(ctx, adapter.UnderlyingDB(), feedName, "child-1")
			require.NoError(t, err)
			require.NotNil(t, child1)
			assert.Equal(t, changestream.StateScheduled, child1.State)
			require.NotNil(t, child1.ScheduledAt)
			assert.WithinDuration(t, time.Now(), *child1.ScheduledAt, time.Second)

			child2, err := getPartition(ctx, adapter.UnderlyingDB(), feedName, "child-2")
			require.NoError(t, err)
			require.NotNil(t, child2)
			assert.Equal(t, changestream.StateScheduled, child2.State)
			require.NotNil(t, child2.ScheduledAt)
			assert.WithinDuration(t, time.Now(), *child2.ScheduledAt, time.Second)
		})

		t.Run("schedule partition with all parents finished", func(t *testing.T) {
			// Mark parents as finished
			flushState(ctx, t, adapter, feedName, "child-1", changestream.StateFinished)
			flushState(ctx, t, adapter, feedName, "child-2", changestream.StateFinished)

			// Add grandchild with both parents
			flushPartition(ctx, t, adapter, feedName, "grandchild", []string{"child-1", "child-2"}, time.Now())

			// Schedule
			count, err := adapter.ScheduleChangeStreamPartitions(ctx, feedName)
			require.NoError(t, err)
			require.Equal(t, int64(1), count, "should schedule grandchild")

			// Verify grandchild is scheduled
			grandchild, err := getPartition(ctx, adapter.UnderlyingDB(), feedName, "grandchild")
			require.NoError(t, err)
			require.NotNil(t, grandchild)
			assert.Equal(t, changestream.StateScheduled, grandchild.State)
			require.NotNil(t, grandchild.ScheduledAt)
			assert.WithinDuration(t, time.Now(), *grandchild.ScheduledAt, time.Second)
		})

		t.Run("don't schedule partition with incomplete parents", func(t *testing.T) {
			// Add a parent that's not finished (with a fictional parent so it won't be scheduled)
			flushPartition(ctx, t, adapter, feedName, "incomplete-parent", []string{"grandparent"}, time.Now())

			// Add child with incomplete parent
			flushPartition(ctx, t, adapter, feedName, "blocked-child", []string{"incomplete-parent"}, time.Now())

			// Try to schedule
			count, err := adapter.ScheduleChangeStreamPartitions(ctx, feedName)
			require.NoError(t, err)
			require.Equal(t, int64(0), count, "should not schedule blocked child")

			// Verify blocked child is still in Created state
			child, err := getPartition(ctx, adapter.UnderlyingDB(), feedName, "blocked-child")
			require.NoError(t, err)
			require.NotNil(t, child)
			assert.Equal(t, changestream.StateCreated, child.State)
			assert.Nil(t, child.ScheduledAt)
		})
	})
}

func TestPartitionLifecycle(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		feedName := "test_partition_lifecycle"
		adapter, cleanup := setupMetadataTest(ctx, t, db, feedName)
		defer cleanup()

		// Add the initial partition and set it in finished state
		flushPartition(ctx, t, adapter, feedName, "", nil, time.Now())
		flushState(ctx, t, adapter, feedName, "", changestream.StateFinished)

		token := "lifecycle-test"
		startTime := time.Now()

		// Create
		flushPartition(ctx, t, adapter, feedName, token, nil, startTime)

		partition, err := getPartition(ctx, adapter.UnderlyingDB(), feedName, token)
		require.NoError(t, err)
		require.NotNil(t, partition)
		assert.WithinDuration(t, startTime, partition.StartTimestamp, time.Second)
		assert.Equal(t, changestream.StateCreated, partition.State)
		assert.WithinDuration(t, startTime, partition.Watermark, time.Second)
		assert.WithinDuration(t, startTime, partition.CreatedAt, time.Second)

		// Schedule
		count, err := adapter.ScheduleChangeStreamPartitions(ctx, feedName)
		require.NoError(t, err)
		require.Equal(t, int64(1), count)

		partition, err = getPartition(ctx, adapter.UnderlyingDB(), feedName, token)
		require.NoError(t, err)
		require.NotNil(t, partition)
		assert.Equal(t, changestream.StateScheduled, partition.State)
		require.NotNil(t, partition.ScheduledAt)
		assert.WithinDuration(t, time.Now(), *partition.ScheduledAt, time.Second)

		// Running
		flushState(ctx, t, adapter, feedName, token, changestream.StateRunning)

		partition, err = getPartition(ctx, adapter.UnderlyingDB(), feedName, token)
		require.NoError(t, err)
		require.NotNil(t, partition)
		assert.Equal(t, changestream.StateRunning, partition.State)
		require.NotNil(t, partition.RunningAt)
		assert.WithinDuration(t, time.Now(), *partition.RunningAt, time.Second)

		// Update watermark while running
		newWatermark := startTime.Add(time.Hour)
		flushWatermark(ctx, t, adapter, feedName, token, newWatermark)

		partition, err = getPartition(ctx, adapter.UnderlyingDB(), feedName, token)
		require.NoError(t, err)
		require.NotNil(t, partition)
		assert.Equal(t, changestream.StateRunning, partition.State)
		assert.WithinDuration(t, newWatermark, partition.Watermark, time.Second)

		// Finished
		flushState(ctx, t, adapter, feedName, token, changestream.StateFinished)

		partition, err = getPartition(ctx, adapter.UnderlyingDB(), feedName, token)
		require.NoError(t, err)
		require.NotNil(t, partition)
		assert.Equal(t, changestream.StateFinished, partition.State)
		require.NotNil(t, partition.FinishedAt)
		assert.WithinDuration(t, time.Now(), *partition.FinishedAt, time.Second)
	})
}

func TestPartitionState(t *testing.T) {
	t.Run("Valid returns true for valid states", func(t *testing.T) {
		require.True(t, changestream.StateCreated.Valid())
		require.True(t, changestream.StateScheduled.Valid())
		require.True(t, changestream.StateRunning.Valid())
		require.True(t, changestream.StateFinished.Valid())
	})

	t.Run("Valid returns false for invalid states", func(t *testing.T) {
		require.False(t, changestream.PartitionState(-1).Valid())
		require.False(t, changestream.PartitionState(99).Valid())
	})

	t.Run("EncodeSpanner encodes valid states", func(t *testing.T) {
		for expected, state := range []changestream.PartitionState{
			changestream.StateCreated,
			changestream.StateScheduled,
			changestream.StateRunning,
			changestream.StateFinished,
		} {
			value, err := state.EncodeSpanner()
			require.NoError(t, err)
			require.Equal(t, int64(expected), value)
		}
	})

	t.Run("EncodeSpanner errors on invalid state", func(t *testing.T) {
		_, err := changestream.PartitionState(99).EncodeSpanner()
		require.Error(t, err)
	})

	t.Run("DecodeSpanner decodes valid states", func(t *testing.T) {
		for value, expected := range []changestream.PartitionState{
			changestream.StateCreated,
			changestream.StateScheduled,
			changestream.StateRunning,
			changestream.StateFinished,
		} {
			var state changestream.PartitionState
			err := state.DecodeSpanner(int64(value))
			require.NoError(t, err)
			require.Equal(t, expected, state)
		}
	})

	t.Run("DecodeSpanner errors on invalid type", func(t *testing.T) {
		var state changestream.PartitionState
		err := state.DecodeSpanner("invalid")
		require.Error(t, err)
	})

	t.Run("DecodeSpanner errors on invalid value", func(t *testing.T) {
		var state changestream.PartitionState
		err := state.DecodeSpanner(int64(99))
		require.Error(t, err)
	})
}

type partitionMetadata struct {
	PartitionToken string
	ParentTokens   []string
	StartTimestamp time.Time
	State          changestream.PartitionState
	Watermark      time.Time
	CreatedAt      time.Time
	ScheduledAt    *time.Time
	RunningAt      *time.Time
	FinishedAt     *time.Time
}

func getPartition(ctx context.Context, client *recordeddb.SpannerClient, feedName, partitionToken string) (metadata *partitionMetadata, err error) {
	metadataTable := spannerutil.QuoteIdentifier(feedName + "_metadata")

	stmt := spanner.Statement{
		SQL: `
			SELECT partition_token, parent_tokens, start_timestamp, state, watermark,
			       created_at, scheduled_at, running_at, finished_at
			FROM ` + metadataTable + `
			WHERE partition_token = @partition_token
		`,
		Params: map[string]interface{}{
			"partition_token": partitionToken,
		},
	}

	var result *partitionMetadata
	err = client.Single().QueryWithOptions(ctx, stmt,
		spanner.QueryOptions{RequestTag: "change-stream-get-partition"},
	).Do(func(row *spanner.Row) error {
		result = &partitionMetadata{}
		return row.Columns(
			&result.PartitionToken,
			&result.ParentTokens,
			&result.StartTimestamp,
			&result.State,
			&result.Watermark,
			&result.CreatedAt,
			&result.ScheduledAt,
			&result.RunningAt,
			&result.FinishedAt,
		)
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	if result == nil {
		return nil, errs.New("partition not found: partition_token=%q", partitionToken)
	}

	return result, nil
}
