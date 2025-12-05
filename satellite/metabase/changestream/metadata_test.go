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

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/recordeddb"
	"storj.io/storj/shared/dbutil/spannerutil"
)

// setupMetadataTest creates a metadata table for testing and returns the client and cleanup function.
func setupMetadataTest(ctx *testcontext.Context, t *testing.T, db *metabase.DB, feedName string) (
	client *recordeddb.SpannerClient,
	cleanup func(),
) {
	if db.Implementation() != dbutil.Spanner {
		t.Skip("test requires Spanner adapter")
	}

	streamId := metabasetest.RandObjectStream()
	adapter := db.ChooseAdapter(streamId.ProjectID)
	spannerAdapter, ok := adapter.(*metabase.SpannerAdapter)
	require.True(t, ok, "adapter should be SpannerAdapter")

	client = spannerAdapter.UnderlyingDB()

	// Clean up any existing table first to ensure fresh schema
	_ = spannerAdapter.TestDeleteChangeStreamMetadata(ctx, feedName)

	err := spannerAdapter.TestCreateChangeStreamMetadata(ctx, feedName)
	require.NoError(t, err)

	cleanup = func() {
		err := spannerAdapter.TestDeleteChangeStreamMetadata(ctx, feedName)
		require.NoError(t, err)
	}

	return client, cleanup
}

func TestNoPartitionMetadata(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		feedName := "test_no_partition_metadata"
		client, cleanup := setupMetadataTest(ctx, t, db, feedName)
		defer cleanup()

		t.Run("empty table returns true", func(t *testing.T) {
			empty, err := changestream.NoPartitionMetadata(ctx, client, feedName)
			require.NoError(t, err)
			require.True(t, empty, "metadata table should be empty")
		})

		t.Run("non-empty table returns false", func(t *testing.T) {
			// Add a partition
			err := changestream.AddChildPartition(ctx, client, feedName, "", nil, time.Now())
			require.NoError(t, err)

			empty, err := changestream.NoPartitionMetadata(ctx, client, feedName)
			require.NoError(t, err)
			require.False(t, empty, "metadata table should not be empty")
		})
	})
}

func TestAddChildPartition(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		feedName := "test_add_child_partition"
		client, cleanup := setupMetadataTest(ctx, t, db, feedName)
		defer cleanup()

		t.Run("add initial partition with empty token", func(t *testing.T) {
			startTime := time.Now()
			err := changestream.AddChildPartition(ctx, client, feedName, "", nil, startTime)
			require.NoError(t, err)

			partition, err := getPartition(ctx, client, feedName, "")
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

			err := changestream.AddChildPartition(ctx, client, feedName, token, parentTokens, startTime)
			require.NoError(t, err)

			partition, err := getPartition(ctx, client, feedName, token)
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
			err := changestream.AddChildPartition(ctx, client, feedName, token, nil, startTime)
			require.NoError(t, err)

			// Add second time - should not error
			err = changestream.AddChildPartition(ctx, client, feedName, token, nil, startTime)
			require.NoError(t, err, "duplicate insertion should be idempotent")
		})
	})
}

func TestGetPartitionsByState(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		feedName := "test_get_partitions_by_state"
		client, cleanup := setupMetadataTest(ctx, t, db, feedName)
		defer cleanup()

		// Add partitions in different states
		startTime := time.Now()

		// Add some partitions in Created state
		err := changestream.AddChildPartition(ctx, client, feedName, "created-1", nil, startTime)
		require.NoError(t, err)
		err = changestream.AddChildPartition(ctx, client, feedName, "created-2", nil, startTime)
		require.NoError(t, err)

		// Add partition and update to Scheduled state
		err = changestream.AddChildPartition(ctx, client, feedName, "scheduled-1", nil, startTime)
		require.NoError(t, err)
		err = changestream.UpdatePartitionState(ctx, client, feedName, "scheduled-1", changestream.StateScheduled)
		require.NoError(t, err)

		t.Run("query Created state", func(t *testing.T) {
			partitions, err := changestream.GetPartitionsByState(ctx, client, feedName, changestream.StateCreated)
			require.NoError(t, err)
			require.Len(t, partitions, 2)
			require.Contains(t, partitions, "created-1")
			require.Contains(t, partitions, "created-2")
		})

		t.Run("query Scheduled state", func(t *testing.T) {
			partitions, err := changestream.GetPartitionsByState(ctx, client, feedName, changestream.StateScheduled)
			require.NoError(t, err)
			require.Len(t, partitions, 1)
			require.Contains(t, partitions, "scheduled-1")
		})

		t.Run("query state with no matches", func(t *testing.T) {
			partitions, err := changestream.GetPartitionsByState(ctx, client, feedName, changestream.StateRunning)
			require.NoError(t, err)
			require.Empty(t, partitions)
		})

		t.Run("verify watermark values", func(t *testing.T) {
			partitions, err := changestream.GetPartitionsByState(ctx, client, feedName, changestream.StateCreated)
			require.NoError(t, err)
			for _, watermark := range partitions {
				require.WithinDuration(t, startTime, watermark, time.Second)
			}
		})
	})
}

func TestUpdatePartitionWatermark(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		feedName := "test_update_partition_watermark"
		client, cleanup := setupMetadataTest(ctx, t, db, feedName)
		defer cleanup()

		t.Run("successfully update watermark", func(t *testing.T) {
			token := "test-token"
			startTime := time.Now()
			err := changestream.AddChildPartition(ctx, client, feedName, token, nil, startTime)
			require.NoError(t, err)

			// Update watermark
			newWatermark := startTime.Add(time.Hour)
			err = changestream.UpdatePartitionWatermark(ctx, client, feedName, token, newWatermark)
			require.NoError(t, err)

			// Verify watermark was updated
			partitions, err := changestream.GetPartitionsByState(ctx, client, feedName, changestream.StateCreated)
			require.NoError(t, err)
			require.Contains(t, partitions, token)
			require.WithinDuration(t, newWatermark, partitions[token], time.Second)
		})

		t.Run("error on non-existent partition", func(t *testing.T) {
			err := changestream.UpdatePartitionWatermark(ctx, client, feedName, "non-existent", time.Now())
			require.Error(t, err)
			require.Contains(t, err.Error(), "affected 0 rows")
		})
	})
}

func TestUpdatePartitionState(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		feedName := "test_update_partition_state"
		client, cleanup := setupMetadataTest(ctx, t, db, feedName)
		defer cleanup()

		t.Run("update to StateScheduled", func(t *testing.T) {
			token := "token-scheduled"
			err := changestream.AddChildPartition(ctx, client, feedName, token, nil, time.Now())
			require.NoError(t, err)

			err = changestream.UpdatePartitionState(ctx, client, feedName, token, changestream.StateScheduled)
			require.NoError(t, err)

			// Verify state was updated and scheduled_at is set
			metadata, err := getPartition(ctx, client, feedName, token)
			require.NoError(t, err)
			require.Equal(t, changestream.StateScheduled, metadata.State)
			require.NotNil(t, metadata.ScheduledAt, "scheduled_at should be set")
			require.Nil(t, metadata.RunningAt, "running_at should not be set yet")
			require.Nil(t, metadata.FinishedAt, "finished_at should not be set yet")
		})

		t.Run("update to StateRunning", func(t *testing.T) {
			token := "token-running"
			err := changestream.AddChildPartition(ctx, client, feedName, token, nil, time.Now())
			require.NoError(t, err)

			err = changestream.UpdatePartitionState(ctx, client, feedName, token, changestream.StateRunning)
			require.NoError(t, err)

			// Verify state was updated and running_at is set
			metadata, err := getPartition(ctx, client, feedName, token)
			require.NoError(t, err)
			require.Equal(t, changestream.StateRunning, metadata.State)
			require.NotNil(t, metadata.RunningAt, "running_at should be set")
			require.Nil(t, metadata.FinishedAt, "finished_at should not be set yet")
		})

		t.Run("update to StateFinished", func(t *testing.T) {
			token := "token-finished"
			err := changestream.AddChildPartition(ctx, client, feedName, token, nil, time.Now())
			require.NoError(t, err)

			err = changestream.UpdatePartitionState(ctx, client, feedName, token, changestream.StateFinished)
			require.NoError(t, err)

			// Verify state was updated and finished_at is set
			metadata, err := getPartition(ctx, client, feedName, token)
			require.NoError(t, err)
			require.Equal(t, changestream.StateFinished, metadata.State)
			require.NotNil(t, metadata.FinishedAt, "finished_at should be set")
		})

		t.Run("error on updating to StateCreated", func(t *testing.T) {
			token := "token-created-error"
			err := changestream.AddChildPartition(ctx, client, feedName, token, nil, time.Now())
			require.NoError(t, err)

			err = changestream.UpdatePartitionState(ctx, client, feedName, token, changestream.StateCreated)
			require.Error(t, err)
		})

		t.Run("error on invalid state value", func(t *testing.T) {
			token := "token-invalid"
			err := changestream.AddChildPartition(ctx, client, feedName, token, nil, time.Now())
			require.NoError(t, err)

			err = changestream.UpdatePartitionState(ctx, client, feedName, token, changestream.PartitionState(99))
			require.Error(t, err)
		})

		t.Run("error on non-existent partition", func(t *testing.T) {
			err := changestream.UpdatePartitionState(ctx, client, feedName, "non-existent", changestream.StateScheduled)
			require.Error(t, err)
			require.Contains(t, err.Error(), "affected 0 rows")
		})
	})
}

func TestSchedulePartitions(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		feedName := "test_schedule_partitions"
		client, cleanup := setupMetadataTest(ctx, t, db, feedName)
		defer cleanup()

		t.Run("schedule initial partition", func(t *testing.T) {
			// Add initial partition with empty token
			err := changestream.AddChildPartition(ctx, client, feedName, "", nil, time.Now())
			require.NoError(t, err)

			// Schedule partitions
			count, err := changestream.SchedulePartitions(ctx, client, feedName)
			require.NoError(t, err)
			require.Equal(t, int64(1), count, "should schedule initial partition")

			// Verify initial partition is scheduled
			partition, err := getPartition(ctx, client, feedName, "")
			require.NoError(t, err)
			require.NotNil(t, partition)
			assert.Equal(t, changestream.StateScheduled, partition.State)
			require.NotNil(t, partition.ScheduledAt)
			assert.WithinDuration(t, time.Now(), *partition.ScheduledAt, time.Second)
		})

		t.Run("schedule children of initial partition after initial finishes", func(t *testing.T) {
			// Add children with no parents
			err := changestream.AddChildPartition(ctx, client, feedName, "child-1", nil, time.Now())
			require.NoError(t, err)
			// Add another child, but with empty parent list instead of nil
			err = changestream.AddChildPartition(ctx, client, feedName, "child-2", []string{}, time.Now())
			require.NoError(t, err)

			// Try to schedule - should not schedule yet because initial is not finished
			count, err := changestream.SchedulePartitions(ctx, client, feedName)
			require.NoError(t, err)
			require.Equal(t, int64(0), count, "should not schedule children yet")

			// Mark initial partition as finished
			err = changestream.UpdatePartitionState(ctx, client, feedName, "", changestream.StateFinished)
			require.NoError(t, err)

			// Now schedule should work
			count, err = changestream.SchedulePartitions(ctx, client, feedName)
			require.NoError(t, err)
			require.Equal(t, int64(2), count, "should schedule both children")

			// Verify children are scheduled
			child1, err := getPartition(ctx, client, feedName, "child-1")
			require.NoError(t, err)
			require.NotNil(t, child1)
			assert.Equal(t, changestream.StateScheduled, child1.State)
			require.NotNil(t, child1.ScheduledAt)
			assert.WithinDuration(t, time.Now(), *child1.ScheduledAt, time.Second)

			child2, err := getPartition(ctx, client, feedName, "child-2")
			require.NoError(t, err)
			require.NotNil(t, child2)
			assert.Equal(t, changestream.StateScheduled, child2.State)
			require.NotNil(t, child2.ScheduledAt)
			assert.WithinDuration(t, time.Now(), *child2.ScheduledAt, time.Second)
		})

		t.Run("schedule partition with all parents finished", func(t *testing.T) {
			// Mark parents as finished
			err := changestream.UpdatePartitionState(ctx, client, feedName, "child-1", changestream.StateFinished)
			require.NoError(t, err)
			err = changestream.UpdatePartitionState(ctx, client, feedName, "child-2", changestream.StateFinished)
			require.NoError(t, err)

			// Add grandchild with both parents
			err = changestream.AddChildPartition(ctx, client, feedName, "grandchild", []string{"child-1", "child-2"}, time.Now())
			require.NoError(t, err)

			// Schedule
			count, err := changestream.SchedulePartitions(ctx, client, feedName)
			require.NoError(t, err)
			require.Equal(t, int64(1), count, "should schedule grandchild")

			// Verify grandchild is scheduled
			grandchild, err := getPartition(ctx, client, feedName, "grandchild")
			require.NoError(t, err)
			require.NotNil(t, grandchild)
			assert.Equal(t, changestream.StateScheduled, grandchild.State)
			require.NotNil(t, grandchild.ScheduledAt)
			assert.WithinDuration(t, time.Now(), *grandchild.ScheduledAt, time.Second)
		})

		t.Run("don't schedule partition with incomplete parents", func(t *testing.T) {
			// Add a parent that's not finished (with a fictional parent so it won't be scheduled)
			err := changestream.AddChildPartition(ctx, client, feedName, "incomplete-parent", []string{"grandparent"}, time.Now())
			require.NoError(t, err)

			// Add child with incomplete parent
			err = changestream.AddChildPartition(ctx, client, feedName, "blocked-child", []string{"incomplete-parent"}, time.Now())
			require.NoError(t, err)

			// Try to schedule
			count, err := changestream.SchedulePartitions(ctx, client, feedName)
			require.NoError(t, err)
			require.Equal(t, int64(0), count, "should not schedule blocked child")

			// Verify blocked child is still in Created state
			child, err := getPartition(ctx, client, feedName, "blocked-child")
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
		client, cleanup := setupMetadataTest(ctx, t, db, feedName)
		defer cleanup()

		// Add the initial partition and set it in finished state
		err := changestream.AddChildPartition(ctx, client, feedName, "", nil, time.Now())
		require.NoError(t, err)
		err = changestream.UpdatePartitionState(ctx, client, feedName, "", changestream.StateFinished)
		require.NoError(t, err)

		token := "lifecycle-test"
		startTime := time.Now()

		// Create
		err = changestream.AddChildPartition(ctx, client, feedName, token, nil, startTime)
		require.NoError(t, err)

		partition, err := getPartition(ctx, client, feedName, token)
		require.NoError(t, err)
		require.NotNil(t, partition)
		assert.WithinDuration(t, startTime, partition.StartTimestamp, time.Second)
		assert.Equal(t, changestream.StateCreated, partition.State)
		assert.WithinDuration(t, startTime, partition.Watermark, time.Second)
		assert.WithinDuration(t, startTime, partition.CreatedAt, time.Second)

		// Schedule
		count, err := changestream.SchedulePartitions(ctx, client, feedName)
		require.NoError(t, err)
		require.Equal(t, int64(1), count)

		partition, err = getPartition(ctx, client, feedName, token)
		require.NoError(t, err)
		require.NotNil(t, partition)
		assert.Equal(t, changestream.StateScheduled, partition.State)
		require.NotNil(t, partition.ScheduledAt)
		assert.WithinDuration(t, time.Now(), *partition.ScheduledAt, time.Second)

		// Running
		err = changestream.UpdatePartitionState(ctx, client, feedName, token, changestream.StateRunning)
		require.NoError(t, err)

		partition, err = getPartition(ctx, client, feedName, token)
		require.NoError(t, err)
		require.NotNil(t, partition)
		assert.Equal(t, changestream.StateRunning, partition.State)
		require.NotNil(t, partition.RunningAt)
		assert.WithinDuration(t, time.Now(), *partition.RunningAt, time.Second)

		// Update watermark while running
		newWatermark := startTime.Add(time.Hour)
		err = changestream.UpdatePartitionWatermark(ctx, client, feedName, token, newWatermark)
		require.NoError(t, err)

		partition, err = getPartition(ctx, client, feedName, token)
		require.NoError(t, err)
		require.NotNil(t, partition)
		assert.Equal(t, changestream.StateRunning, partition.State)
		assert.WithinDuration(t, newWatermark, partition.Watermark, time.Second)

		// Finished
		err = changestream.UpdatePartitionState(ctx, client, feedName, token, changestream.StateFinished)
		require.NoError(t, err)

		partition, err = getPartition(ctx, client, feedName, token)
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
