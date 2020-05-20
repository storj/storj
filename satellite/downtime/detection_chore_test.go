// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package downtime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/overlay"
)

func TestDetectionChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		satellite := planet.Satellites[0]

		node.Contact.Chore.Pause(ctx)
		satellite.DowntimeTracking.DetectionChore.Loop.Pause()

		// setup
		nodeInfo := planet.StorageNodes[0].Contact.Service.Local()
		info := overlay.NodeCheckInInfo{
			NodeID: nodeInfo.ID,
			IsUp:   true,
			Address: &pb.NodeAddress{
				Address: nodeInfo.Address,
			},
			Operator: &nodeInfo.Operator,
			Version:  &nodeInfo.Version,
		}

		sixtyOneMinutes := 61 * time.Minute
		{ // test node ping back success
			// check-in 1 hours, 1 minute ago for that node
			oldCheckinTime := time.Now().Add(-sixtyOneMinutes)
			err := satellite.DB.OverlayCache().UpdateCheckIn(ctx, info, oldCheckinTime, overlay.NodeSelectionConfig{})
			require.NoError(t, err)

			// get successful nodes that haven't checked in with the hour. should return 1
			nodeLastContacts, err := satellite.DB.OverlayCache().GetSuccesfulNodesNotCheckedInSince(ctx, time.Hour)
			require.NoError(t, err)
			require.Len(t, nodeLastContacts, 1)
			require.WithinDuration(t, oldCheckinTime, nodeLastContacts[0].LastContactSuccess, time.Second)

			// run detection chore
			satellite.DowntimeTracking.DetectionChore.Loop.TriggerWait()

			// node should not be in "offline" list or "successful, not checked in" list
			nodeLastContacts, err = satellite.DB.OverlayCache().GetSuccesfulNodesNotCheckedInSince(ctx, time.Hour)
			require.NoError(t, err)
			require.Len(t, nodeLastContacts, 0)

			nodesOffline, err := satellite.DB.OverlayCache().GetOfflineNodesLimited(ctx, 10)
			require.NoError(t, err)
			require.Len(t, nodesOffline, 0)
		}

		{ // test node ping back failure
			// check-in 1 hour, 1 minute ago for that node - again
			oldCheckinTime := time.Now().Add(-sixtyOneMinutes)
			err := satellite.DB.OverlayCache().UpdateCheckIn(ctx, info, oldCheckinTime, overlay.NodeSelectionConfig{})
			require.NoError(t, err)

			// close the node service so the ping back will fail
			err = node.Server.Close()
			require.NoError(t, err)

			// get successful nodes that haven't checked in with the hour. should return 1 - again
			nodeLastContacts, err := satellite.DB.OverlayCache().GetSuccesfulNodesNotCheckedInSince(ctx, time.Hour)
			require.NoError(t, err)
			require.Len(t, nodeLastContacts, 1)
			require.WithinDuration(t, oldCheckinTime, nodeLastContacts[0].LastContactSuccess, time.Second)

			// run detection chore - again
			satellite.DowntimeTracking.DetectionChore.Loop.TriggerWait()

			// node should be in "offline" list but not in "successful, not checked in" list
			nodeLastContacts, err = satellite.DB.OverlayCache().GetSuccesfulNodesNotCheckedInSince(ctx, time.Hour)
			require.NoError(t, err)
			require.Len(t, nodeLastContacts, 0)

			nodesOffline, err := satellite.DB.OverlayCache().GetOfflineNodesLimited(ctx, 10)
			require.NoError(t, err)
			require.Len(t, nodesOffline, 1)
		}
	})
}
