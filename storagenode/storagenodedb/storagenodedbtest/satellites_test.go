// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedbtest_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

// TestGracefulExitDB tests the graceful exit database calls
func TestGracefulExitDB(t *testing.T) { //satelliteID storj.NodeID, finishedAt time.Time, exitStatus satelliteStatus, completionReceipt []byte) (err error) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		start := time.Now()
		require.NoError(t, db.Satellites().InitiateGracefulExit(ctx, storj.NodeID{}, start, 5000))
		exits, err := db.Satellites().ListGracefulExits(ctx)
		require.NoError(t, err)
		require.Equal(t, len(exits), 1)
		require.Equal(t, exits[0].BytesDeleted, int64(0))
		require.Equal(t, exits[0].CompletionReceipt, []byte(nil))
		require.Equal(t, *exits[0].InitiatedAt, start.UTC())
		require.Nil(t, exits[0].FinishedAt)
		require.Equal(t, exits[0].SatelliteID, storj.NodeID{})
		require.Equal(t, exits[0].StartingDiskUsage, int64(5000))

		require.NoError(t, db.Satellites().UpdateGracefulExit(ctx, storj.NodeID{}, 1000))
		require.NoError(t, db.Satellites().UpdateGracefulExit(ctx, storj.NodeID{}, 1000))
		require.NoError(t, db.Satellites().UpdateGracefulExit(ctx, storj.NodeID{}, 1000))

		exits, err = db.Satellites().ListGracefulExits(ctx)
		require.NoError(t, err)
		require.Equal(t, len(exits), 1)
		require.Equal(t, exits[0].BytesDeleted, int64(3000))
		require.Equal(t, exits[0].CompletionReceipt, []byte(nil))
		require.Equal(t, *exits[0].InitiatedAt, start.UTC())
		require.Nil(t, exits[0].FinishedAt)
		require.Equal(t, exits[0].SatelliteID, storj.NodeID{})
		require.Equal(t, exits[0].StartingDiskUsage, int64(5000))

		stop := time.Now()
		require.NoError(t, db.Satellites().CompleteGracefulExit(ctx, storj.NodeID{}, stop, satellites.ExitSucceeded, []byte{0, 0, 0}))
		exits, err = db.Satellites().ListGracefulExits(ctx)
		require.NoError(t, err)
		require.Equal(t, len(exits), 1)
		require.Equal(t, exits[0].BytesDeleted, int64(2000))
		require.Equal(t, exits[0].CompletionReceipt, []byte{0, 0, 0})
		require.Equal(t, *exits[0].InitiatedAt, start.UTC())
		require.Equal(t, *exits[0].FinishedAt, stop.UTC())
		require.Equal(t, exits[0].SatelliteID, storj.NodeID{})
		require.Equal(t, exits[0].StartingDiskUsage, int64(5000))

	})
}
