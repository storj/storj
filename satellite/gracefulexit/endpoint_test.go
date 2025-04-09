// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
)

func TestSuccess(t *testing.T) {
	const steps = 5
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// check that there are no exiting nodes.
		exitingNodes, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 0)

		exitingNode := planet.StorageNodes[0]

		simTime := time.Now()
		satellite.GracefulExit.Endpoint.SetNowFunc(func() time.Time { return simTime })
		doneTime := simTime.AddDate(0, 0, satellite.Config.GracefulExit.GracefulExitDurationInDays)
		interval := doneTime.Sub(simTime) / steps

		// we should get NotReady responses until after the GE time has elapsed.
		for simTime.Before(doneTime) {
			response, err := callProcess(ctx, exitingNode, satellite)
			require.NoError(t, err)
			require.IsType(t, (*pb.SatelliteMessage_NotReady)(nil), response.GetMessage())

			// check that the exiting node is still currently exiting.
			exitingNodes, err = satellite.DB.OverlayCache().GetExitingNodes(ctx)
			require.NoError(t, err)
			require.Len(t, exitingNodes, 1)
			require.Equal(t, exitingNode.ID(), exitingNodes[0].NodeID)

			simTime = simTime.Add(interval)
		}
		simTime = doneTime.Add(time.Second)

		// now we should get a successful finish message
		response, err := callProcess(ctx, exitingNode, satellite)
		require.NoError(t, err)
		require.IsType(t, (*pb.SatelliteMessage_ExitCompleted)(nil), response.GetMessage())

		// verify signature on exit receipt and we're done
		m := response.GetMessage().(*pb.SatelliteMessage_ExitCompleted)
		signee := signing.SigneeFromPeerIdentity(satellite.Identity.PeerIdentity())
		err = signing.VerifyExitCompleted(ctx, signee, m.ExitCompleted)
		require.NoError(t, err)
	})
}

func TestExitDisqualifiedNodeFailOnStart(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 2,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		exitingNode := planet.StorageNodes[0]

		_, err := satellite.DB.OverlayCache().DisqualifyNode(ctx, exitingNode.ID(), time.Now(), overlay.DisqualificationReasonUnknown)
		require.NoError(t, err)

		conn, err := exitingNode.Dialer.DialNodeURL(ctx, satellite.NodeURL())
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		client := pb.NewDRPCSatelliteGracefulExitClient(conn)
		processClient, err := client.Process(ctx)
		require.NoError(t, err)

		// Process endpoint should return immediately if node is disqualified
		response, err := processClient.Recv()
		require.True(t, errs2.IsRPC(err, rpcstatus.FailedPrecondition))
		require.Nil(t, response)

		require.NoError(t, processClient.Close())

		// make sure GE was not initiated for the disqualified node
		exitStatus, err := satellite.Overlay.DB.GetExitStatus(ctx, exitingNode.ID())
		require.NoError(t, err)
		require.Nil(t, exitStatus.ExitInitiatedAt)
		require.False(t, exitStatus.ExitSuccess)
	})
}

func TestExitDisabled(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 2,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.GracefulExit.Enabled = false
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		exitingNode := planet.StorageNodes[0]

		require.Nil(t, satellite.GracefulExit.Endpoint)

		conn, err := exitingNode.Dialer.DialNodeURL(ctx, satellite.NodeURL())
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		client := pb.NewDRPCSatelliteGracefulExitClient(conn)
		processClient, err := client.Process(ctx)
		require.NoError(t, err)

		// Process endpoint should return immediately if GE is disabled
		response, err := processClient.Recv()
		require.Error(t, err)
		// drpc will return "Unknown"
		require.True(t, errs2.IsRPC(err, rpcstatus.Unknown))
		require.Nil(t, response)
	})
}

func TestIneligibleNodeAge(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// Set the required node age to 1 month.
				config.GracefulExit.NodeMinAgeInMonths = 1
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// check that there are no exiting nodes.
		exitingNodes, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 0)

		exitingNode := planet.StorageNodes[0]

		// try to initiate GE; expect to get a node ineligible error
		response, err := callProcess(ctx, exitingNode, satellite)
		require.Error(t, err)
		require.Nil(t, response)
		require.True(t, errs2.IsRPC(err, rpcstatus.FailedPrecondition))

		// check that there are still no exiting nodes
		exitingNodes, err = satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 0)
	})
}

func TestIneligibleNodeAgeOld(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 5,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					// Set the required node age to 1 month.
					config.GracefulExit.NodeMinAgeInMonths = 1
				},
				testplanet.ReconfigureRS(2, 3, 4, 4),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		nodeFullIDs := make(map[storj.NodeID]*identity.FullIdentity)
		for _, node := range planet.StorageNodes {
			nodeFullIDs[node.ID()] = node.Identity
		}

		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		// check that there are no exiting nodes.
		exitingNodes, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 0)

		exitingNode, err := findNodeToExit(ctx, planet, 1)
		require.NoError(t, err)

		// connect to satellite so we initiate the exit.
		conn, err := exitingNode.Dialer.DialNodeURL(ctx, satellite.NodeURL())
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		client := pb.NewDRPCSatelliteGracefulExitClient(conn)

		c, err := client.Process(ctx)
		require.NoError(t, err)

		_, err = c.Recv()
		// expect the node ineligible error here
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.FailedPrecondition))

		// check that there are still no exiting nodes
		exitingNodes, err = satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 0)

		// close the old client
		require.NoError(t, c.CloseSend())
	})
}

func findNodeToExit(ctx context.Context, planet *testplanet.Planet, objects int) (*testplanet.StorageNode, error) {
	satellite := planet.Satellites[0]

	pieceCountMap := make(map[storj.NodeID]int, len(planet.StorageNodes))
	for _, node := range planet.StorageNodes {
		pieceCountMap[node.ID()] = 0
	}

	segments, err := satellite.Metabase.DB.TestingAllSegments(ctx)
	if err != nil {
		return nil, err
	}
	for _, segment := range segments {
		for _, piece := range segment.Pieces {
			pieceCountMap[piece.StorageNode]++
		}
	}

	var exitingNodeID storj.NodeID
	maxCount := 0
	for k, v := range pieceCountMap {
		if exitingNodeID.IsZero() {
			exitingNodeID = k
			maxCount = v
			continue
		}
		if v > maxCount {
			exitingNodeID = k
			maxCount = v
		}
	}

	return planet.FindNode(exitingNodeID), nil
}

func TestNodeAlreadyExited(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// check that there are no exiting nodes.
		exitingNodes, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 0)

		exitingNode := planet.StorageNodes[0]

		simTime := time.Now()
		satellite.GracefulExit.Endpoint.SetNowFunc(func() time.Time { return simTime })
		doneTime := simTime.AddDate(0, 0, satellite.Config.GracefulExit.GracefulExitDurationInDays)

		// initiate GE
		response, err := callProcess(ctx, exitingNode, satellite)
		require.NoError(t, err)
		require.IsType(t, (*pb.SatelliteMessage_NotReady)(nil), response.GetMessage())

		// jump to when GE will be done
		simTime = doneTime.Add(time.Second)

		// should get ExitCompleted now
		response, err = callProcess(ctx, exitingNode, satellite)
		require.NoError(t, err)
		require.IsType(t, (*pb.SatelliteMessage_ExitCompleted)(nil), response.GetMessage())

		// now that the node has successfully exited, try doing it again! we expect to get the
		// ExitCompleted message again.
		response, err = callProcess(ctx, exitingNode, satellite)
		require.NoError(t, err)
		require.IsType(t, (*pb.SatelliteMessage_ExitCompleted)(nil), response.GetMessage())

		// check that node is not marked as exiting still
		exitingNodes, err = satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 0)
	})
}

func TestNodeSuspended(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// check that there are no exiting nodes.
		exitingNodes, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 0)

		// mark a node as suspended
		exitingNode := planet.StorageNodes[0]
		err = satellite.Reputation.Service.TestSuspendNodeUnknownAudit(ctx, exitingNode.ID(), time.Now())
		require.NoError(t, err)

		// initiate GE
		response, err := callProcess(ctx, exitingNode, satellite)
		require.Error(t, err)
		require.ErrorContains(t, err, "node is suspended")
		require.Nil(t, response)
	})
}

func TestManyNodesGracefullyExiting(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 8,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		uplink := planet.Uplinks[0]

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		// upload several objects; enough that we can reasonably expect every node to have several pieces
		const numObjects = 32
		objectData := make([][]byte, numObjects)
		for i := 0; i < numObjects; i++ {
			objectData[i] = testrand.Bytes(64 * memory.KiB)
			err := uplink.Upload(ctx, satellite, "testbucket", fmt.Sprintf("test/path/obj%d", i), objectData[i])
			require.NoError(t, err, i)
		}

		// Make half of the nodes initiate GE
		for i := 0; i < len(planet.StorageNodes)/2; i++ {
			response, err := callProcess(ctx, planet.StorageNodes[i], satellite)
			require.NoError(t, err, i)
			require.IsType(t, (*pb.SatelliteMessage_NotReady)(nil), response.GetMessage())
		}

		// run the satellite ranged loop to build the transfer queue.
		_, err := satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		// we expect ~78% of segments to be in the repair queue (the chance that a
		// segment still has at least 3 pieces in not-exiting nodes). but since things
		// will fluctuate, let's just expect half
		count, err := satellite.Repair.Queue.Count(ctx)
		require.NoError(t, err)
		require.GreaterOrEqual(t, count, numObjects/2)

		// perform the repairs, which should get every piece so that it will still be
		// reconstructable without the exiting nodes.
		satellite.Repair.Repairer.Loop.TriggerWait()
		require.NoError(t, satellite.Repair.Repairer.WaitForPendingRepairs(ctx))

		// turn off the exiting nodes entirely
		for i := 0; i < len(planet.StorageNodes)/2; i++ {
			err = planet.StopNodeAndUpdate(ctx, planet.StorageNodes[i])
			require.NoError(t, err)
		}

		// expect that we can retrieve and verify all objects
		for i, obj := range objectData {
			gotData, err := uplink.Download(ctx, satellite, "testbucket", fmt.Sprintf("test/path/obj%d", i))
			require.NoError(t, err, i)
			require.Equal(t, string(obj), string(gotData))
		}
	})
}

func TestNodeFailingGracefulExitWithLowOnlineScore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.AuditHistory.WindowSize = 24 * time.Hour
				config.Reputation.AuditHistory.TrackingPeriod = 3 * 24 * time.Hour
				config.Reputation.FlushInterval = 0
				config.GracefulExit.MinimumOnlineScore = 0.6
				config.GracefulExit.GracefulExitDurationInDays = 4
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		exitingNode := planet.StorageNodes[0]

		exitingNode.GracefulExit.Chore.Loop.Pause()
		exitingNode.Contact.Chore.Pause(ctx)

		simTime := time.Now()
		satellite.GracefulExit.Endpoint.SetNowFunc(func() time.Time { return simTime })
		doneTime := simTime.AddDate(0, 0, satellite.Config.GracefulExit.GracefulExitDurationInDays)

		// initiate GE
		response, err := callProcess(ctx, exitingNode, satellite)
		require.NoError(t, err)
		require.IsType(t, (*pb.SatelliteMessage_NotReady)(nil), response.GetMessage())

		// set the audit history for that node to reflect a poor online score
		last := reputation.AuditSuccess
		for simTime.Before(doneTime) {
			// alternate between Success and Offline to get a ~50% score
			if last == reputation.AuditSuccess {
				last = reputation.AuditOffline
			} else {
				last = reputation.AuditSuccess
			}
			_, err := satellite.DB.Reputation().Update(ctx, reputation.UpdateRequest{
				NodeID:       exitingNode.ID(),
				AuditOutcome: last,
				Config:       satellite.Config.Reputation,
			}, simTime)
			require.NoError(t, err)

			// GE shouldn't fail until the end of the period, so node has a chance to get score back up
			response, err := callProcess(ctx, exitingNode, satellite)
			require.NoError(t, err)
			require.IsTypef(t, (*pb.SatelliteMessage_NotReady)(nil), response.GetMessage(), "simTime=%s, doneTime=%s", simTime, doneTime)

			simTime = simTime.Add(time.Hour)
		}
		err = satellite.Reputation.Service.TestFlushAllNodeInfo(ctx)
		require.NoError(t, err)

		simTime = doneTime.Add(time.Second)
		response, err = callProcess(ctx, exitingNode, satellite)
		require.NoError(t, err)
		msg := response.GetMessage()
		require.IsType(t, (*pb.SatelliteMessage_ExitFailed)(nil), msg)
		failure := msg.(*pb.SatelliteMessage_ExitFailed)

		// validate signature on failure message
		signee := signing.SigneeFromPeerIdentity(satellite.Identity.PeerIdentity())
		err = signing.VerifyExitFailed(ctx, signee, failure.ExitFailed)
		require.Equal(t, exitingNode.ID(), failure.ExitFailed.NodeId)
		// truncate to micros since the Failed time has gone through the database
		expectedFailTime := simTime.Truncate(time.Microsecond)
		require.Falsef(t, failure.ExitFailed.Failed.Before(expectedFailTime),
			"failure time should have been at or after %s: %s", simTime, failure.ExitFailed.Failed)
		require.Equal(t, satellite.ID(), failure.ExitFailed.SatelliteId)
		require.Equal(t, pb.ExitFailed_INACTIVE_TIMEFRAME_EXCEEDED, failure.ExitFailed.Reason)
		require.NoError(t, err)
	})
}

func TestSuspendedNodesFailGracefulExit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.FlushInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		exitingNode := planet.StorageNodes[0]

		simTime := time.Now()
		satellite.GracefulExit.Endpoint.SetNowFunc(func() time.Time { return simTime })
		doneTime := simTime.AddDate(0, 0, satellite.Config.GracefulExit.GracefulExitDurationInDays)

		// initiate GE
		response, err := callProcess(ctx, exitingNode, satellite)
		require.NoError(t, err)
		require.IsType(t, (*pb.SatelliteMessage_NotReady)(nil), response.GetMessage())

		// suspend the node
		err = satellite.Reputation.Service.TestSuspendNodeUnknownAudit(ctx, exitingNode.ID(), simTime)
		require.NoError(t, err)

		// expect failure when the time is up
		simTime = doneTime.Add(time.Second)

		response, err = callProcess(ctx, exitingNode, satellite)
		require.NoError(t, err)
		msg := response.GetMessage()
		require.IsType(t, (*pb.SatelliteMessage_ExitFailed)(nil), msg)
		failure := msg.(*pb.SatelliteMessage_ExitFailed)
		require.Equal(t, pb.ExitFailed_OVERALL_FAILURE_PERCENTAGE_EXCEEDED, failure.ExitFailed.Reason)
	})
}

func callProcess(ctx *testcontext.Context, exitingNode *testplanet.StorageNode, satellite *testplanet.Satellite) (*pb.SatelliteMessage, error) {
	conn, err := exitingNode.Dialer.DialNodeURL(ctx, satellite.NodeURL())
	if err != nil {
		return nil, err
	}
	defer ctx.Check(conn.Close)

	client := pb.NewDRPCSatelliteGracefulExitClient(conn)

	c, err := client.Process(ctx)
	if err != nil {
		return nil, err
	}
	defer ctx.Check(c.CloseSend)

	return c.Recv()
}
