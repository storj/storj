// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
	"storj.io/storj/satellite/metabase/metabasetest"
)

// countingAdapter is a minimal Adapter stub that counts UpdateChangeStreamPartitions calls.
type countingAdapter struct {
	changestream.Adapter
	updateCalls int
}

func (a *countingAdapter) UpdateChangeStreamPartitions(_ context.Context, _ string, _ changestream.PartitionUpdates) error {
	a.updateCalls++
	return nil
}

func TestMetadataBatcherNoOp(t *testing.T) {
	t.Run("empty flush does not call adapter", func(t *testing.T) {
		adapter := &countingAdapter{}
		b := changestream.NewMetadataBatcher(zap.NewNop(), adapter, "feed")
		require.NoError(t, b.Flush(context.Background()))
		require.Zero(t, adapter.updateCalls)
	})

	t.Run("buffers are cleared after flush", func(t *testing.T) {
		adapter := &countingAdapter{}
		b := changestream.NewMetadataBatcher(zap.NewNop(), adapter, "feed")
		b.UpdatePartitionState(changestream.StateFinished, "token-1")
		require.NoError(t, b.Flush(context.Background()))
		require.Equal(t, 1, adapter.updateCalls)

		// Second flush should not call the adapter — buffers are cleared
		require.NoError(t, b.Flush(context.Background()))
		require.Equal(t, 1, adapter.updateCalls)
	})
}

func TestMetadataBatcher(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		feedName := "test_metadata_batcher"
		adapter, cleanup := setupMetadataTest(ctx, t, db, feedName)
		defer cleanup()

		startTime := time.Now()

		t.Run("batch multiple child partitions in single flush", func(t *testing.T) {
			b := changestream.NewMetadataBatcher(zap.NewNop(), adapter, feedName)
			b.AddChildPartition("batch-token-1", nil, startTime)
			b.AddChildPartition("batch-token-2", []string{"batch-token-1"}, startTime)
			require.NoError(t, b.Flush(ctx))

			p1, err := getPartition(ctx, adapter.UnderlyingDB(), feedName, "batch-token-1")
			require.NoError(t, err)
			assert.Equal(t, changestream.StateCreated, p1.State)
			assert.WithinDuration(t, startTime, p1.StartTimestamp, time.Second)

			p2, err := getPartition(ctx, adapter.UnderlyingDB(), feedName, "batch-token-2")
			require.NoError(t, err)
			assert.Equal(t, changestream.StateCreated, p2.State)
			assert.ElementsMatch(t, []string{"batch-token-1"}, p2.ParentTokens)
		})

		t.Run("batch state and watermark updates in single flush", func(t *testing.T) {
			b := changestream.NewMetadataBatcher(zap.NewNop(), adapter, feedName)
			b.UpdatePartitionState(changestream.StateRunning, "batch-token-1")
			newWatermark := startTime.Add(time.Hour)
			b.UpdatePartitionWatermark("batch-token-2", newWatermark)
			require.NoError(t, b.Flush(ctx))

			p1, err := getPartition(ctx, adapter.UnderlyingDB(), feedName, "batch-token-1")
			require.NoError(t, err)
			assert.Equal(t, changestream.StateRunning, p1.State)
			require.NotNil(t, p1.RunningAt)

			p2, err := getPartition(ctx, adapter.UnderlyingDB(), feedName, "batch-token-2")
			require.NoError(t, err)
			assert.WithinDuration(t, newWatermark, p2.Watermark, time.Second)
		})

		t.Run("watermark last-write-wins", func(t *testing.T) {
			first := startTime.Add(time.Minute)
			last := startTime.Add(2 * time.Minute)

			b := changestream.NewMetadataBatcher(zap.NewNop(), adapter, feedName)
			b.UpdatePartitionWatermark("batch-token-1", first)
			b.UpdatePartitionWatermark("batch-token-1", last)
			require.NoError(t, b.Flush(ctx))

			p, err := getPartition(ctx, adapter.UnderlyingDB(), feedName, "batch-token-1")
			require.NoError(t, err)
			assert.WithinDuration(t, last, p.Watermark, time.Second)
		})
	})
}
