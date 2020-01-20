// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package downtime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestEstimationChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Downtime.EstimationBatchSize = 1
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		satellite := planet.Satellites[0]
		node.Contact.Chore.Pause(ctx)
		satellite.DowntimeTracking.EstimationChore.Loop.Pause()
		{ // test estimation chore updates uptime correctly for an online node
			// mark node as failing an uptime check so the estimation chore picks it up
			_, err := satellite.DB.OverlayCache().UpdateUptime(ctx, node.ID(), false)
			require.NoError(t, err)
			oldNode, err := satellite.DB.OverlayCache().Get(ctx, node.ID())
			require.NoError(t, err)
			require.True(t, oldNode.Reputation.LastContactSuccess.Before(oldNode.Reputation.LastContactFailure))
			// run estimation chore
			time.Sleep(1 * time.Second) // wait for 1s because estimation chore truncates offline duration to seconds
			satellite.DowntimeTracking.EstimationChore.Loop.TriggerWait()
			// get offline time for node, expect it to be 0 since node was online when chore pinged it
			downtime, err := satellite.DB.DowntimeTracking().GetOfflineTime(ctx, node.ID(), time.Now().Add(-5*time.Hour), time.Now())
			require.NoError(t, err)
			require.True(t, downtime == 0)
			// expect node last contact success was updated
			newNode, err := satellite.DB.OverlayCache().Get(ctx, node.ID())
			require.NoError(t, err)
			require.Equal(t, oldNode.Reputation.LastContactFailure, newNode.Reputation.LastContactFailure)
			require.True(t, oldNode.Reputation.LastContactSuccess.Before(newNode.Reputation.LastContactSuccess))
			require.True(t, newNode.Reputation.LastContactFailure.Before(newNode.Reputation.LastContactSuccess))
		}
		{ // test estimation chore correctly aggregates offline time
			// mark node as failing an uptime check so the estimation chore picks it up
			_, err := satellite.DB.OverlayCache().UpdateUptime(ctx, node.ID(), false)
			require.NoError(t, err)
			oldNode, err := satellite.DB.OverlayCache().Get(ctx, node.ID())
			require.NoError(t, err)
			require.True(t, oldNode.Reputation.LastContactSuccess.Before(oldNode.Reputation.LastContactFailure))
			// close the node service so the ping back will fail
			err = node.Server.Close()
			require.NoError(t, err)
			// run estimation chore
			time.Sleep(1 * time.Second) // wait for 1s because estimation chore truncates offline duration to seconds
			satellite.DowntimeTracking.EstimationChore.Loop.TriggerWait()
			// get offline time for node, expect it to be greater than 0 since node has been offline for at least 1s
			downtime, err := satellite.DB.DowntimeTracking().GetOfflineTime(ctx, node.ID(), time.Now().Add(-5*time.Hour), time.Now())
			require.NoError(t, err)
			require.True(t, downtime > 0)
			// expect node last contact failure was updated
			newNode, err := satellite.DB.OverlayCache().Get(ctx, node.ID())
			require.NoError(t, err)
			require.Equal(t, oldNode.Reputation.LastContactSuccess, newNode.Reputation.LastContactSuccess)
			require.True(t, oldNode.Reputation.LastContactFailure.Before(newNode.Reputation.LastContactFailure))
		}
	})
}
