// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/overlay"
)

// TestSuspendBasic ensures that we can suspend a node using overlayService.SuspendNode and that we can unsuspend a node using overlayservice.UnsuspendNode
func TestSuspendBasic(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		oc := planet.Satellites[0].Overlay.DB

		node, err := oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.Suspended)

		timeToSuspend := time.Now().UTC().Truncate(time.Second)
		err = oc.SuspendNode(ctx, nodeID, timeToSuspend)
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, node.Suspended)
		require.True(t, node.Suspended.Equal(timeToSuspend))

		err = oc.UnsuspendNode(ctx, nodeID)
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.Suspended)
	})
}

// TestSuspendWithUpdateStats ensures that a node goes into suspension node from getting enough unknown audits, and gets removed from getting enough successful audits.
func TestSuspendWithUpdateStats(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		oc := planet.Satellites[0].Overlay.Service

		node, err := oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.Suspended)
		testStartTime := time.Now()

		// give node one unknown audit - bringing unknown audit rep to 0.5, and suspending node
		_, err = oc.UpdateStats(ctx, &overlay.UpdateRequest{
			NodeID:       nodeID,
			AuditOutcome: overlay.AuditUnknown,
			IsUp:         true,
			AuditLambda:  1,
			AuditWeight:  1,
			AuditDQ:      0.6,
		})
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		// expect unknown audit alpha/beta to change and suspended to be set
		require.True(t, node.Reputation.UnknownAuditReputationAlpha < 1)
		require.True(t, node.Reputation.UnknownAuditReputationBeta > 0)
		require.NotNil(t, node.Suspended)
		require.True(t, node.Suspended.After(testStartTime))
		// expect node is not disqualified and that normal audit alpha/beta remain unchanged
		require.Nil(t, node.Disqualified)
		require.EqualValues(t, node.Reputation.AuditReputationAlpha, 1)
		require.EqualValues(t, node.Reputation.AuditReputationBeta, 0)

		// give node two successful audits - bringing unknown audit rep to 0.75, and unsuspending node
		for i := 0; i < 2; i++ {
			_, err = oc.UpdateStats(ctx, &overlay.UpdateRequest{
				NodeID:       nodeID,
				AuditOutcome: overlay.AuditSuccess,
				IsUp:         true,
				AuditLambda:  1,
				AuditWeight:  1,
				AuditDQ:      0.6,
			})
			require.NoError(t, err)
		}
		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.Suspended)
	})
}

// TestSuspendFailedAudit ensures that a node is not suspended for a failed audit.
func TestSuspendFailedAudit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		oc := planet.Satellites[0].Overlay.DB

		node, err := oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.Nil(t, node.Suspended)

		// give node one failed audit - bringing audit rep to 0.5, and disqualifying node
		// expect that suspended field and unknown audit reputation remain unchanged
		_, err = oc.UpdateStats(ctx, &overlay.UpdateRequest{
			NodeID:       nodeID,
			AuditOutcome: overlay.AuditFailure,
			IsUp:         true,
			AuditLambda:  1,
			AuditWeight:  1,
			AuditDQ:      0.6,
		})
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, node.Disqualified)
		require.Nil(t, node.Suspended)
		require.EqualValues(t, node.Reputation.UnknownAuditReputationAlpha, 1)
		require.EqualValues(t, node.Reputation.UnknownAuditReputationBeta, 0)
	})
}

// TestSuspendExceedGracePeriod ensures that a node is disqualified when it receives a failing or unknown audit after the grace period expires.
func TestSuspendExceedGracePeriod(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.SuspensionGracePeriod = time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		successNodeID := planet.StorageNodes[0].ID()
		failNodeID := planet.StorageNodes[1].ID()
		offlineNodeID := planet.StorageNodes[2].ID()
		unknownNodeID := planet.StorageNodes[3].ID()

		// suspend each node two hours ago (more than grace period)
		oc := planet.Satellites[0].DB.OverlayCache()
		for _, node := range (storj.NodeIDList{successNodeID, failNodeID, offlineNodeID, unknownNodeID}) {
			err := oc.SuspendNode(ctx, node, time.Now().Add(-2*time.Hour))
			require.NoError(t, err)
		}

		// no nodes should be disqualified
		for _, node := range (storj.NodeIDList{successNodeID, failNodeID, offlineNodeID, unknownNodeID}) {
			n, err := oc.Get(ctx, node)
			require.NoError(t, err)
			require.Nil(t, n.Disqualified)
		}

		// give one node a successful audit, one a failed audit, one an offline audit, and one an unknown audit
		report := audit.Report{
			Successes: storj.NodeIDList{successNodeID},
			Fails:     storj.NodeIDList{failNodeID},
			Offlines:  storj.NodeIDList{offlineNodeID},
			Unknown:   storj.NodeIDList{unknownNodeID},
		}
		auditService := planet.Satellites[0].Audit
		_, err := auditService.Reporter.RecordAudits(ctx, report, "")
		require.NoError(t, err)

		// success and offline nodes should not be disqualified
		// fail and unknown nodes should be disqualified
		for _, node := range (storj.NodeIDList{successNodeID, offlineNodeID}) {
			n, err := oc.Get(ctx, node)
			require.NoError(t, err)
			require.Nil(t, n.Disqualified)
		}
		for _, node := range (storj.NodeIDList{failNodeID, unknownNodeID}) {
			n, err := oc.Get(ctx, node)
			require.NoError(t, err)
			require.NotNil(t, n.Disqualified)
		}
	})
}

// TestSuspendDQDisabled ensures that a node is not disqualified from suspended mode if the suspension DQ enabled flag is false.
func TestSuspendDQDisabled(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.SuspensionGracePeriod = time.Hour
				config.Overlay.Node.SuspensionDQEnabled = false
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		successNodeID := planet.StorageNodes[0].ID()
		failNodeID := planet.StorageNodes[1].ID()
		offlineNodeID := planet.StorageNodes[2].ID()
		unknownNodeID := planet.StorageNodes[3].ID()

		// suspend each node two hours ago (more than grace period)
		oc := planet.Satellites[0].DB.OverlayCache()
		for _, node := range (storj.NodeIDList{successNodeID, failNodeID, offlineNodeID, unknownNodeID}) {
			err := oc.SuspendNode(ctx, node, time.Now().Add(-2*time.Hour))
			require.NoError(t, err)
		}

		// no nodes should be disqualified
		for _, node := range (storj.NodeIDList{successNodeID, failNodeID, offlineNodeID, unknownNodeID}) {
			n, err := oc.Get(ctx, node)
			require.NoError(t, err)
			require.Nil(t, n.Disqualified)
		}

		// give one node a successful audit, one a failed audit, one an offline audit, and one an unknown audit
		report := audit.Report{
			Successes: storj.NodeIDList{successNodeID},
			Fails:     storj.NodeIDList{failNodeID},
			Offlines:  storj.NodeIDList{offlineNodeID},
			Unknown:   storj.NodeIDList{unknownNodeID},
		}
		auditService := planet.Satellites[0].Audit
		_, err := auditService.Reporter.RecordAudits(ctx, report, "")
		require.NoError(t, err)

		// successful node should not be suspended or disqualified
		n, err := oc.Get(ctx, successNodeID)
		require.NoError(t, err)
		require.Nil(t, n.Suspended)
		require.Nil(t, n.Disqualified)

		// failed node should not be suspended but should be disqualified
		// (disqualified because of a failed audit, not because of exceeding suspension grace period)
		n, err = oc.Get(ctx, failNodeID)
		require.NoError(t, err)
		require.Nil(t, n.Suspended)
		require.NotNil(t, n.Disqualified)

		// offline node should still be suspended but not disqualified
		n, err = oc.Get(ctx, offlineNodeID)
		require.NoError(t, err)
		require.NotNil(t, n.Suspended)
		require.Nil(t, n.Disqualified)

		// unknown node should still be suspended but not disqualified
		n, err = oc.Get(ctx, unknownNodeID)
		require.NoError(t, err)
		require.NotNil(t, n.Suspended)
		require.Nil(t, n.Disqualified)
	})
}

// TestSuspendBatchUpdateStats ensures that suspension and alpha/beta fields are properly updated from batch update stats
func TestSuspendBatchUpdateStats(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		oc := planet.Satellites[0].Overlay.Service

		node, err := oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.Suspended)
		testStartTime := time.Now()

		nodeUpdateReq := &overlay.UpdateRequest{
			NodeID:       nodeID,
			AuditOutcome: overlay.AuditSuccess,
			IsUp:         true,
			AuditLambda:  1,
			AuditWeight:  1,
			AuditDQ:      0.6,
		}

		// give node successful audit - expect alpha to be > 1 and beta to be 0
		_, err = oc.BatchUpdateStats(ctx, []*overlay.UpdateRequest{nodeUpdateReq})
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		// expect unknown audit alpha/beta to change and suspended to be nil
		require.True(t, node.Reputation.UnknownAuditReputationAlpha > 1)
		require.True(t, node.Reputation.UnknownAuditReputationBeta == 0)
		require.Nil(t, node.Suspended)
		// expect audit alpha/beta to change and disqualified to be nil
		require.True(t, node.Reputation.AuditReputationAlpha > 1)
		require.True(t, node.Reputation.AuditReputationBeta == 0)
		require.Nil(t, node.Disqualified)
		require.EqualValues(t, node.Reputation.AuditReputationAlpha, 1)
		require.EqualValues(t, node.Reputation.AuditReputationBeta, 0)

		oldReputation := node.Reputation

		// give node two unknown audits to suspend node
		nodeUpdateReq.AuditOutcome = overlay.AuditUnknown
		_, err = oc.BatchUpdateStats(ctx, []*overlay.UpdateRequest{nodeUpdateReq})
		require.NoError(t, err)
		_, err = oc.BatchUpdateStats(ctx, []*overlay.UpdateRequest{nodeUpdateReq})
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.True(t, node.Reputation.UnknownAuditReputationAlpha < oldReputation.UnknownAuditReputationAlpha)
		require.True(t, node.Reputation.UnknownAuditReputationBeta > oldReputation.UnknownAuditReputationBeta)
		require.NotNil(t, node.Suspended)
		require.True(t, node.Reputation.Suspended.After(testStartTime))
		// node should not be disqualified and normal audit reputation should not change
		require.EqualValues(t, node.Reputation.AuditReputationAlpha, oldReputation.AuditReputationAlpha)
		require.EqualValues(t, node.Reputation.AuditReputationBeta, oldReputation.AuditReputationBeta)
		require.Nil(t, node.Disqualified)
	})
}
