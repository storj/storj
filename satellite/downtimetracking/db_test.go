// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package downtimetracking_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestDowntime(t *testing.T) {
	// test basic downtime functionality
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		downtimeDB := db.DowntimeTracking()

		now := time.Now()
		oneYearAgo := time.Now().Add(-time.Hour * 24 * 365)

		// node1 was offline for one hour recently, and offline for two hours a year ago
		nodeID1 := testrand.NodeID()
		err := downtimeDB.Add(ctx, nodeID1, now, time.Hour)
		require.NoError(t, err)
		err = downtimeDB.Add(ctx, nodeID1, oneYearAgo, 2*time.Hour)
		require.NoError(t, err)

		// node2 was offline for two hours a year ago
		nodeID2 := testrand.NodeID()
		err = downtimeDB.Add(ctx, nodeID2, oneYearAgo, 2*time.Hour)
		require.NoError(t, err)

		// if we only check recent history, node1 offline time should be 1 hour
		duration, err := downtimeDB.GetOfflineTime(ctx, nodeID1, now.Add(-time.Hour), now.Add(time.Hour))
		require.NoError(t, err)
		require.Equal(t, duration, time.Hour)

		// if we only check recent history, node2 should not be offline at all
		duration, err = downtimeDB.GetOfflineTime(ctx, nodeID2, now.Add(-time.Hour), now.Add(time.Hour))
		require.NoError(t, err)
		require.Equal(t, duration, time.Duration(0))

		// if we only check old history, node1 offline time should be 2 hours
		duration, err = downtimeDB.GetOfflineTime(ctx, nodeID1, oneYearAgo.Add(-time.Hour), oneYearAgo.Add(time.Hour))
		require.NoError(t, err)
		require.Equal(t, duration, 2*time.Hour)

		// if we only check old history, node2 offline time should be 2 hours
		duration, err = downtimeDB.GetOfflineTime(ctx, nodeID2, oneYearAgo.Add(-time.Hour), oneYearAgo.Add(time.Hour))
		require.NoError(t, err)
		require.Equal(t, duration, 2*time.Hour)

		// if we check all history (from before oneYearAgo to after now), node1 should be offline for 3 hours
		duration, err = downtimeDB.GetOfflineTime(ctx, nodeID1, oneYearAgo.Add(-time.Hour), now.Add(time.Hour))
		require.NoError(t, err)
		require.Equal(t, duration, 3*time.Hour)

		// if we check all history (from before oneYearAgo to after now), node2 should be offline for 2 hours
		duration, err = downtimeDB.GetOfflineTime(ctx, nodeID2, oneYearAgo.Add(-time.Hour), now.Add(time.Hour))
		require.NoError(t, err)
		require.Equal(t, duration, 2*time.Hour)
	})
}
