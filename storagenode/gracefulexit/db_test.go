// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

// TestDB tests the graceful exit database calls.
func TestDB(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		for i := 0; i <= 3; i++ {
			nodeID := testrand.NodeID()
			start := time.Now()

			_, err := db.Satellites().GetSatellite(ctx, nodeID)
			require.NoError(t, err)

			require.NoError(t, db.Satellites().InitiateGracefulExit(ctx, nodeID, start, 5000))
			exits, err := db.Satellites().ListGracefulExits(ctx)
			require.NoError(t, err)
			require.Equal(t, len(exits), i+1)
			require.Equal(t, exits[i].BytesDeleted, int64(0))
			require.Equal(t, exits[i].CompletionReceipt, []byte(nil))
			require.True(t, exits[i].InitiatedAt.Equal(start))
			require.Nil(t, exits[i].FinishedAt)
			require.Equal(t, exits[i].SatelliteID, nodeID)
			require.Equal(t, exits[i].StartingDiskUsage, int64(5000))

			require.NoError(t, db.Satellites().UpdateGracefulExit(ctx, nodeID, 1000))
			require.NoError(t, db.Satellites().UpdateGracefulExit(ctx, nodeID, 1000))
			require.NoError(t, db.Satellites().UpdateGracefulExit(ctx, nodeID, 1000))

			exits, err = db.Satellites().ListGracefulExits(ctx)
			require.NoError(t, err)
			require.Equal(t, len(exits), i+1)
			require.Equal(t, exits[i].BytesDeleted, int64(3000))
			require.Equal(t, exits[i].CompletionReceipt, []byte(nil))
			require.True(t, exits[i].InitiatedAt.Equal(start))
			require.Nil(t, exits[i].FinishedAt)
			require.Equal(t, exits[i].SatelliteID, nodeID)
			require.Equal(t, exits[i].StartingDiskUsage, int64(5000))

			stop := time.Now()
			require.NoError(t, db.Satellites().CompleteGracefulExit(ctx, nodeID, stop, satellites.ExitSucceeded, []byte{0, 0, 0}))
			exits, err = db.Satellites().ListGracefulExits(ctx)
			require.NoError(t, err)
			require.Equal(t, len(exits), i+1)
			require.Equal(t, exits[i].BytesDeleted, int64(3000))
			require.Equal(t, exits[i].CompletionReceipt, []byte{0, 0, 0})
			require.True(t, exits[i].InitiatedAt.Equal(start))
			require.True(t, exits[i].FinishedAt.Equal(stop))
			require.Equal(t, exits[i].SatelliteID, nodeID)
			require.Equal(t, exits[i].StartingDiskUsage, int64(5000))

			satellite, err := db.Satellites().GetSatellite(ctx, nodeID)
			require.NoError(t, err)
			require.Equal(t, nodeID, satellite.SatelliteID)
			require.False(t, satellite.AddedAt.IsZero())
			require.EqualValues(t, 3, satellite.Status)
		}
	})
}
