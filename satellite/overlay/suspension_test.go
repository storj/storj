// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
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
