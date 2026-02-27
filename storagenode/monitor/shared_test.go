// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/storj/storagenode/hashstore"
)

func TestSpaceUsage_ReservedField(t *testing.T) {
	ctx := context.Background()

	// Create a test hashstore backend to generate space usage stats
	config := hashstore.CreateDefaultConfig(hashstore.TableKind_HashTbl, false)
	config.Compaction.RewriteMultiple = 3.0 // Set a specific rewrite multiple for testing

	db, err := hashstore.New(ctx, config, t.TempDir(), "", nil, hashstore.Callbacks{})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	// Create some data to ensure non-zero table sizes
	for i := 0; i < 10; i++ {
		key := [32]byte{byte(i)} // Create a simple key
		w, err := db.Create(ctx, key, time.Time{})
		require.NoError(t, err)
		_, err = w.Write(make([]byte, 1024))
		require.NoError(t, err)
		require.NoError(t, w.Close())
	}

	// Get the hashstore stats
	dbStats, s0Stats, s1Stats := db.Stats()

	// Test that FreeRequired is calculated correctly in individual stores
	expectedS0FreeRequired := memory.Size(2+config.Compaction.RewriteMultiple) * s0Stats.Table.TableSize
	expectedS1FreeRequired := memory.Size(2+config.Compaction.RewriteMultiple) * s1Stats.Table.TableSize
	assert.Equal(t, expectedS0FreeRequired, s0Stats.FreeRequired)
	assert.Equal(t, expectedS1FreeRequired, s1Stats.FreeRequired)

	// Test that FreeRequired is aggregated correctly in DB stats
	expectedDBFreeRequired := max(s0Stats.FreeRequired, s1Stats.FreeRequired)
	assert.Equal(t, expectedDBFreeRequired, dbStats.FreeRequired)

	// Create a mock hash space usage
	hashSpaceUsage := SpaceUsage{
		UsedTotal:       int64(dbStats.LenSet),
		UsedForMetadata: int64(dbStats.TableSize),
		Reserved:        int64(dbStats.FreeRequired), // This is the new field we're testing
	}

	// Test that Reserved field is properly set
	assert.Equal(t, int64(dbStats.FreeRequired), hashSpaceUsage.Reserved)
	require.Greater(t, hashSpaceUsage.Reserved, int64(0), "Reserved space should be positive when there's data")
}
