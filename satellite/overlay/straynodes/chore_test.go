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
	"storj.io/storj/satellite/overlay"
)

func TestDQStrayNodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.StrayNodes.MaxDurationWithoutContact = 24 * time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		strayNode := planet.StorageNodes[0]
		liveNode := planet.StorageNodes[1]
		sat := planet.Satellites[0]
		strayNode.Contact.Chore.Pause(ctx)
		sat.Overlay.DQStrayNodes.Loop.Pause()

		cache := planet.Satellites[0].Overlay.DB

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
		}

		// set strayNode last_contact_success to 48 hours ago
		require.NoError(t, sat.Overlay.DB.UpdateCheckIn(ctx, checkInInfo, time.Now().Add(-48*time.Hour), sat.Config.Overlay.Node))

		sat.Overlay.DQStrayNodes.Loop.TriggerWait()

		strayInfo, err = cache.Get(ctx, strayNode.ID())
		require.NoError(t, err)
		require.NotNil(t, strayInfo.Disqualified)

		liveInfo, err := cache.Get(ctx, liveNode.ID())
		require.NoError(t, err)
		require.Nil(t, liveInfo.Disqualified)
	})
}
