// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package downtime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/overlay"
)

func TestDetectionChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		nodeDossier := planet.StorageNodes[0].Local()
		satellite := planet.Satellites[0]

		node.Contact.Chore.Pause(ctx)
		satellite.DowntimeTracking.DetectionChore.Loop.Pause()

		// setup
		info := overlay.NodeCheckInInfo{
			NodeID:   nodeDossier.Id,
			IsUp:     true,
			Address:  nodeDossier.Address,
			Operator: &nodeDossier.Operator,
			Version:  &nodeDossier.Version,
		}

		config := overlay.NodeSelectionConfig{
			UptimeReputationLambda: 0.99,
			UptimeReputationWeight: 1.0,
			UptimeReputationDQ:     0,
		}
		sixtyOneMinutes := 61 * time.Minute
		{ // test node ping back success
			// check-in 1 hours, 1 minute ago for that node
			oldCheckinTime := time.Now().UTC().Add(-sixtyOneMinutes)
			err := satellite.DB.OverlayCache().UpdateCheckIn(ctx, info, oldCheckinTime, config)
			require.NoError(t, err)

			// get successful nodes that haven't checked in with the hour. should return 1
			nodeLastContacts, err := satellite.DB.OverlayCache().GetSuccesfulNodesNotCheckedInSince(ctx, time.Hour)
			require.NoError(t, err)
			require.Len(t, nodeLastContacts, 1)
			require.Equal(t, oldCheckinTime.Truncate(time.Second), nodeLastContacts[0].LastContactSuccess.Truncate(time.Second)) // truncate to avoid flakiness

			// run detection chore
			satellite.DowntimeTracking.DetectionChore.Loop.TriggerWait()

			nodeLastContacts, err = satellite.DB.OverlayCache().GetSuccesfulNodesNotCheckedInSince(ctx, time.Hour)
			require.NoError(t, err)
			require.Len(t, nodeLastContacts, 0)

			// downtime duration should be 0 for the node
			downtime, err := satellite.DB.DowntimeTracking().GetOfflineTime(ctx, node.ID(), time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
			require.NoError(t, err)
			require.EqualValues(t, 0, downtime)
		}

		{ // test node ping back failure
			// check-in 1 hour, 1 minute ago for that node - again
			oldCheckinTime := time.Now().UTC().Add(-sixtyOneMinutes)
			err := satellite.DB.OverlayCache().UpdateCheckIn(ctx, info, oldCheckinTime, config)
			require.NoError(t, err)

			// close the node service so the ping back will fail
			err = node.Server.Close()
			require.NoError(t, err)

			// get successful nodes that haven't checked in with the hour. should return 1 - again
			nodeLastContacts, err := satellite.DB.OverlayCache().GetSuccesfulNodesNotCheckedInSince(ctx, time.Hour)
			require.NoError(t, err)
			require.Len(t, nodeLastContacts, 1)
			require.Equal(t, oldCheckinTime.Truncate(time.Second), nodeLastContacts[0].LastContactSuccess.Truncate(time.Second)) // truncate to avoid flakiness

			// run detection chore - again
			satellite.DowntimeTracking.DetectionChore.Loop.TriggerWait()

			// downtime duration should be > 1hr
			downtime, err := satellite.DB.DowntimeTracking().GetOfflineTime(ctx, node.ID(), time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
			require.NoError(t, err)
			require.EqualValues(t, time.Hour, downtime.Truncate(time.Hour)) // truncate to avoid flakiness
		}
	})
}
