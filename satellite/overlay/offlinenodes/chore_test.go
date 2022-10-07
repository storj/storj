// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package offlinenodes_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/overlay"
)

func TestOfflineNodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.SendNodeEmails = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		offlineNode := planet.StorageNodes[0]
		onlineNode := planet.StorageNodes[1]
		sat := planet.Satellites[0]
		offlineNode.Contact.Chore.Pause(ctx)
		cache := planet.Satellites[0].Overlay.DB

		offlineInfo, err := cache.Get(ctx, offlineNode.ID())
		require.NoError(t, err)
		require.Nil(t, offlineInfo.LastOfflineEmail)

		checkInInfo := overlay.NodeCheckInInfo{
			NodeID: offlineInfo.Id,
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
				Email: "offline@storj.test",
			},
		}

		// offline node checks in 48 hours ago
		require.NoError(t, sat.Overlay.DB.UpdateCheckIn(ctx, checkInInfo, time.Now().Add(-48*time.Hour), sat.Config.Overlay.Node))

		// online node checks in now
		checkInInfo.NodeID = onlineNode.ID()
		require.NoError(t, sat.Overlay.DB.UpdateCheckIn(ctx, checkInInfo, time.Now(), sat.Config.Overlay.Node))

		sat.Overlay.OfflineNodeEmails.Loop.TriggerWait()

		// offline node gets an email
		offlineInfo, err = cache.Get(ctx, offlineNode.ID())
		require.NoError(t, err)
		lastEmail := offlineInfo.LastOfflineEmail
		require.NotNil(t, lastEmail)

		ne, err := planet.Satellites[0].DB.NodeEvents().GetLatestByEmailAndEvent(ctx, offlineInfo.Operator.Email, nodeevents.Offline)
		require.NoError(t, err)
		require.Equal(t, offlineNode.ID(), ne.NodeID)
		require.Equal(t, offlineInfo.Operator.Email, ne.Email)
		require.Equal(t, nodeevents.Offline, ne.Event)

		firstEventTime := ne.CreatedAt

		// online node does not get an email
		onlineInfo, err := cache.Get(ctx, onlineNode.ID())
		require.NoError(t, err)
		require.Nil(t, onlineInfo.LastOfflineEmail)

		// run chore again and check that offline node does not get another email before cooldown has passed
		sat.Overlay.OfflineNodeEmails.Loop.TriggerWait()

		offlineInfo, err = cache.Get(ctx, offlineNode.ID())
		require.NoError(t, err)
		require.Equal(t, lastEmail, offlineInfo.LastOfflineEmail)

		// change last_offline_email so that cooldown has passed and email should be sent again
		require.NoError(t, cache.UpdateLastOfflineEmail(ctx, []storj.NodeID{offlineNode.ID()}, time.Now().Add(-48*time.Hour)))

		sat.Overlay.OfflineNodeEmails.Loop.TriggerWait()

		ne, err = planet.Satellites[0].DB.NodeEvents().GetLatestByEmailAndEvent(ctx, offlineInfo.Operator.Email, nodeevents.Offline)
		require.NoError(t, err)
		require.Equal(t, offlineNode.ID(), ne.NodeID)
		require.Equal(t, offlineInfo.Operator.Email, ne.Email)
		require.Equal(t, nodeevents.Offline, ne.Event)
		require.True(t, firstEventTime.Before(ne.CreatedAt))

		offlineInfo, err = cache.Get(ctx, offlineNode.ID())
		require.NoError(t, err)
		require.True(t, offlineInfo.LastOfflineEmail.After(*lastEmail))
	})
}
