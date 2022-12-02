// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package straynodes_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/overlay"
)

func TestDQStrayNodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.StrayNodes.MaxDurationWithoutContact = 24 * time.Hour
				config.Overlay.SendNodeEmails = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		strayNode := planet.StorageNodes[0]
		liveNode := planet.StorageNodes[1]
		sat := planet.Satellites[0]
		strayNode.Contact.Chore.Pause(ctx)
		sat.Overlay.DQStrayNodes.Loop.Pause()

		cache := planet.Satellites[0].Overlay.DB
		email := "test@storj.test"
		strayInfo, err := cache.Get(ctx, strayNode.ID())
		require.NoError(t, err)
		require.Nil(t, strayInfo.Disqualified)

		checkInInfo := overlay.NodeCheckInInfo{
			NodeID: strayNode.ID(),
			IsUp:   true,
			Address: &pb.NodeAddress{
				Address: "1.2.3.4",
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
			Operator: &pb.NodeOperator{
				Email: email,
			},
		}

		// set strayNode last_contact_success to 48 hours ago
		require.NoError(t, sat.Overlay.DB.UpdateCheckIn(ctx, checkInInfo, time.Now().Add(-48*time.Hour), sat.Config.Overlay.Node))

		sat.Overlay.DQStrayNodes.Loop.TriggerWait()

		ne, err := sat.DB.NodeEvents().GetLatestByEmailAndEvent(ctx, email, nodeevents.Disqualified)
		require.NoError(t, err)
		require.Equal(t, email, ne.Email)
		require.Equal(t, strayNode.ID(), ne.NodeID)

		strayInfo, err = cache.Get(ctx, strayNode.ID())
		require.NoError(t, err)
		require.NotNil(t, strayInfo.Disqualified)

		liveInfo, err := cache.Get(ctx, liveNode.ID())
		require.NoError(t, err)
		require.Nil(t, liveInfo.Disqualified)
	})
}

// We had a bug in the stray nodes chore where nodes who had not been seen
// in several months were not being DQd. We figured out that this was
// happening because we were using two queries: The first to grab
// nodes where last_contact_success < some cutoff, the second to DQ them
// unless last_contact_success == '0001-01-01 00:00:00+00'. The problem
// is that if all of the nodes returned from the first query had
// last_contact_success of '0001-01-01 00:00:00+00', we would pass them to
// the second query which would not DQ them. This would result in the stray
// nodes DQ loop ending with no DQs. This test consitently failed until the fix
// was implemented.
func TestNodesWithNoLastContactSuccessDoNotBlockDQOfOtherNodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.StrayNodes.MaxDurationWithoutContact = 24 * time.Hour
				config.StrayNodes.Limit = 1
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node1 := planet.StorageNodes[0]
		node2 := planet.StorageNodes[1]
		sat := planet.Satellites[0]
		node1.Contact.Chore.Pause(ctx)
		node2.Contact.Chore.Pause(ctx)
		sat.Overlay.DQStrayNodes.Loop.Pause()

		cache := planet.Satellites[0].Overlay.DB

		checkInInfo := overlay.NodeCheckInInfo{
			NodeID: node1.ID(),
			IsUp:   true,
			Address: &pb.NodeAddress{
				Address: "1.2.3.4",
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
		}

		require.NoError(t, sat.Overlay.DB.UpdateCheckIn(ctx, checkInInfo, time.Time{}, sat.Config.Overlay.Node))
		checkInInfo.NodeID = node2.ID()
		require.NoError(t, sat.Overlay.DB.UpdateCheckIn(ctx, checkInInfo, time.Now().Add(-48*time.Hour), sat.Config.Overlay.Node))

		sat.Overlay.DQStrayNodes.Loop.TriggerWait()

		n1Info, err := cache.Get(ctx, node1.ID())
		require.NoError(t, err)
		require.Nil(t, n1Info.Disqualified)

		n2Info, err := cache.Get(ctx, node2.ID())
		require.NoError(t, err)
		require.NotNil(t, n2Info.Disqualified)
	})
}
