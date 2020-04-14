// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package downtime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/downtime"
	"storj.io/storj/satellite/overlay"
)

// TestEstimationChoreBasic tests the basic functionality of the downtime estimation chore:
// 1. Test that when a node that had one failed ping, and one successful ping >1s later does not have recorded downtime
// 2. Test that when a node that had one failed ping, and another failed ping >1s later has at least 1s of recorded downtime
func TestEstimationChoreBasic(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Downtime.EstimationBatchSize = 2
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		satellite.DowntimeTracking.EstimationChore.Loop.Pause()

		{ // test last_contact_success is updated for nodes where last_contact_failure > last_contact_success, but node is online
			var oldNodes []*overlay.NodeDossier
			for _, node := range planet.StorageNodes {
				node.Contact.Chore.Pause(ctx)
				// mark node as failing an uptime check so the estimation chore picks it up
				_, err := satellite.DB.OverlayCache().UpdateUptime(ctx, node.ID(), false)
				require.NoError(t, err)
				oldNode, err := satellite.DB.OverlayCache().Get(ctx, node.ID())
				require.NoError(t, err)
				require.True(t, oldNode.Reputation.LastContactSuccess.Before(oldNode.Reputation.LastContactFailure))
				oldNodes = append(oldNodes, oldNode)
			}
			// run estimation chore
			time.Sleep(1 * time.Second) // wait for 1s because estimation chore truncates offline duration to seconds
			satellite.DowntimeTracking.EstimationChore.Loop.TriggerWait()
			for i, node := range planet.StorageNodes {
				// get offline time for node, expect it to be 0 since node was online when chore pinged it
				downtime, err := satellite.DB.DowntimeTracking().GetOfflineTime(ctx, node.ID(), time.Now().Add(-5*time.Hour), time.Now())
				require.NoError(t, err)
				require.True(t, downtime == 0)
				// expect node last contact success was updated
				newNode, err := satellite.DB.OverlayCache().Get(ctx, node.ID())
				require.NoError(t, err)
				require.Equal(t, oldNodes[i].Reputation.LastContactFailure, newNode.Reputation.LastContactFailure)
				require.True(t, oldNodes[i].Reputation.LastContactSuccess.Before(newNode.Reputation.LastContactSuccess))
				require.True(t, newNode.Reputation.LastContactFailure.Before(newNode.Reputation.LastContactSuccess))
			}
		}
		{ // test last_contact_failure is updated and downtime is recorded for nodes where last_contact_failure > last_contact_success and node is offline
			var oldNodes []*overlay.NodeDossier
			for _, node := range planet.StorageNodes {
				// mark node as failing an uptime check so the estimation chore picks it up
				_, err := satellite.DB.OverlayCache().UpdateUptime(ctx, node.ID(), false)
				require.NoError(t, err)
				oldNode, err := satellite.DB.OverlayCache().Get(ctx, node.ID())
				require.NoError(t, err)
				require.True(t, oldNode.Reputation.LastContactSuccess.Before(oldNode.Reputation.LastContactFailure))
				// close the node service so the ping back will fail
				err = node.Server.Close()
				require.NoError(t, err)
				oldNodes = append(oldNodes, oldNode)
			}
			// run estimation chore
			time.Sleep(1 * time.Second) // wait for 1s because estimation chore truncates offline duration to seconds
			satellite.DowntimeTracking.EstimationChore.Loop.TriggerWait()
			for i, node := range planet.StorageNodes {
				// get offline time for node, expect it to be greater than 0 since node has been offline for at least 1s
				downtime, err := satellite.DB.DowntimeTracking().GetOfflineTime(ctx, node.ID(), time.Now().Add(-5*time.Hour), time.Now())
				require.NoError(t, err)
				require.True(t, downtime > 0)
				// expect node last contact failure was updated
				newNode, err := satellite.DB.OverlayCache().Get(ctx, node.ID())
				require.NoError(t, err)
				require.Equal(t, oldNodes[i].Reputation.LastContactSuccess, newNode.Reputation.LastContactSuccess)
				require.True(t, oldNodes[i].Reputation.LastContactFailure.Before(newNode.Reputation.LastContactFailure))
			}
		}
	})
}

// TestEstimationChoreSatelliteDowntime tests the situation where downtime is estimated when the satellite was started after the last failed ping
// If a storage node has a failed ping, then another ping fails later, the estimation chore will normally take the difference between these pings and record that as the downtime.
// If the satellite was started between the old failed ping and the new failed ping, we do not want to risk including satellite downtime in our calculation - no downtime should be recorded in this case.
func TestEstimationChoreSatelliteDowntime(t *testing.T) {
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

		// mark node as failing an uptime check so the estimation chore picks it up
		_, err := satellite.DB.OverlayCache().UpdateUptime(ctx, node.ID(), false)
		require.NoError(t, err)
		oldNode, err := satellite.DB.OverlayCache().Get(ctx, node.ID())
		require.NoError(t, err)
		require.True(t, oldNode.Reputation.LastContactSuccess.Before(oldNode.Reputation.LastContactFailure))
		// close the node service so the ping back will fail
		err = node.Server.Close()
		require.NoError(t, err)

		// create new estimation chore that starts after the node's last contacted time
		newEstimationChore := downtime.NewEstimationChore(
			satellite.Log,
			downtime.Config{
				EstimationInterval:         1 * time.Second,
				EstimationBatchSize:        10,
				EstimationConcurrencyLimit: 10,
			},
			satellite.Overlay.Service,
			satellite.DowntimeTracking.Service,
			satellite.DB.DowntimeTracking(),
		)

		time.Sleep(1 * time.Second) // wait for 1s because estimation chore truncates offline duration to seconds

		var group errgroup.Group
		group.Go(func() error {
			return newEstimationChore.Run(ctx)
		})
		defer func() {
			err = newEstimationChore.Close()
			require.NoError(t, err)
			err = group.Wait()
			require.NoError(t, err)
		}()

		newEstimationChore.Loop.TriggerWait()
		// since the estimation chore was started after the last ping, the node's offline time should be 0
		downtime, err := satellite.DB.DowntimeTracking().GetOfflineTime(ctx, node.ID(), time.Now().Add(-5*time.Hour), time.Now())
		require.NoError(t, err)
		require.EqualValues(t, downtime, 0)

		// expect node last contact failure was updated
		newNode, err := satellite.DB.OverlayCache().Get(ctx, node.ID())
		require.NoError(t, err)
		require.Equal(t, oldNode.Reputation.LastContactSuccess, newNode.Reputation.LastContactSuccess)
		require.True(t, oldNode.Reputation.LastContactFailure.Before(newNode.Reputation.LastContactFailure))
	})
}
