// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
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

// TestAuditSuspendBasic ensures that we can suspend a node using overlayService.SuspendNode and that we can unsuspend a node using overlayservice.UnsuspendNode.
func TestAuditSuspendBasic(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		oc := planet.Satellites[0].Overlay.DB

		node, err := oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.UnknownAuditSuspended)

		timeToSuspend := time.Now().UTC().Truncate(time.Second)
		err = oc.SuspendNodeUnknownAudit(ctx, nodeID, timeToSuspend)
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, node.UnknownAuditSuspended)
		require.True(t, node.UnknownAuditSuspended.Equal(timeToSuspend))

		err = oc.UnsuspendNodeUnknownAudit(ctx, nodeID)
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.UnknownAuditSuspended)
	})
}

// TestAuditSuspendWithUpdateStats ensures that a node goes into suspension node from getting enough unknown audits, and gets removed from getting enough successful audits.
func TestAuditSuspendWithUpdateStats(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		oc := planet.Satellites[0].Overlay.Service

		node, err := oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.UnknownAuditSuspended)
		testStartTime := time.Now()

		// give node one unknown audit - bringing unknown audit rep to 0.5, and suspending node
		_, err = oc.UpdateStats(ctx, &overlay.UpdateRequest{
			NodeID:       nodeID,
			AuditOutcome: overlay.AuditUnknown,
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
		require.NotNil(t, node.UnknownAuditSuspended)
		require.True(t, node.UnknownAuditSuspended.After(testStartTime))
		// expect node is not disqualified and that normal audit alpha/beta remain unchanged
		require.Nil(t, node.Disqualified)
		require.EqualValues(t, node.Reputation.AuditReputationAlpha, 1)
		require.EqualValues(t, node.Reputation.AuditReputationBeta, 0)

		// give node two successful audits - bringing unknown audit rep to 0.75, and unsuspending node
		for i := 0; i < 2; i++ {
			_, err = oc.UpdateStats(ctx, &overlay.UpdateRequest{
				NodeID:       nodeID,
				AuditOutcome: overlay.AuditSuccess,
				AuditLambda:  1,
				AuditWeight:  1,
				AuditDQ:      0.6,
			})
			require.NoError(t, err)
		}
		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.UnknownAuditSuspended)
	})
}

// TestAuditSuspendFailedAudit ensures that a node is not suspended for a failed audit.
func TestAuditSuspendFailedAudit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		oc := planet.Satellites[0].Overlay.DB

		node, err := oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.Nil(t, node.UnknownAuditSuspended)

		// give node one failed audit - bringing audit rep to 0.5, and disqualifying node
		// expect that suspended field and unknown audit reputation remain unchanged
		_, err = oc.UpdateStats(ctx, &overlay.UpdateRequest{
			NodeID:       nodeID,
			AuditOutcome: overlay.AuditFailure,
			AuditLambda:  1,
			AuditWeight:  1,
			AuditDQ:      0.6,
			AuditHistory: testAuditHistoryConfig(),
		}, time.Now())
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, node.Disqualified)
		require.Nil(t, node.UnknownAuditSuspended)
		require.EqualValues(t, node.Reputation.UnknownAuditReputationAlpha, 1)
		require.EqualValues(t, node.Reputation.UnknownAuditReputationBeta, 0)
	})
}

// TestAuditSuspendExceedGracePeriod ensures that a node is disqualified when it receives a failing or unknown audit after the grace period expires.
func TestAuditSuspendExceedGracePeriod(t *testing.T) {
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
			err := oc.SuspendNodeUnknownAudit(ctx, node, time.Now().Add(-2*time.Hour))
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

// TestAuditSuspendDQDisabled ensures that a node is not disqualified from suspended mode if the suspension DQ enabled flag is false.
func TestAuditSuspendDQDisabled(t *testing.T) {
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
			err := oc.SuspendNodeUnknownAudit(ctx, node, time.Now().Add(-2*time.Hour))
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
		require.Nil(t, n.UnknownAuditSuspended)
		require.Nil(t, n.Disqualified)

		// failed node should not be suspended but should be disqualified
		// (disqualified because of a failed audit, not because of exceeding suspension grace period)
		n, err = oc.Get(ctx, failNodeID)
		require.NoError(t, err)
		require.Nil(t, n.UnknownAuditSuspended)
		require.NotNil(t, n.Disqualified)

		// offline node should not be suspended or disqualified
		n, err = oc.Get(ctx, offlineNodeID)
		require.NoError(t, err)
		require.Nil(t, n.UnknownAuditSuspended)
		require.Nil(t, n.Disqualified)

		// unknown node should still be suspended but not disqualified
		n, err = oc.Get(ctx, unknownNodeID)
		require.NoError(t, err)
		require.NotNil(t, n.UnknownAuditSuspended)
		require.Nil(t, n.Disqualified)
	})
}

// TestAuditSuspendBatchUpdateStats ensures that suspension and alpha/beta fields are properly updated from batch update stats.
func TestAuditSuspendBatchUpdateStats(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		oc := planet.Satellites[0].Overlay.Service

		node, err := oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.UnknownAuditSuspended)
		testStartTime := time.Now()

		nodeUpdateReq := &overlay.UpdateRequest{
			NodeID:       nodeID,
			AuditOutcome: overlay.AuditSuccess,
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
		require.Nil(t, node.UnknownAuditSuspended)
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
		require.NotNil(t, node.UnknownAuditSuspended)
		require.True(t, node.Reputation.UnknownAuditSuspended.After(testStartTime))
		// node should not be disqualified and normal audit reputation should not change
		require.EqualValues(t, node.Reputation.AuditReputationAlpha, oldReputation.AuditReputationAlpha)
		require.EqualValues(t, node.Reputation.AuditReputationBeta, oldReputation.AuditReputationBeta)
		require.Nil(t, node.Disqualified)
	})
}

// TestOfflineSuspend tests that a node enters offline suspension and "under review" when online score passes below threshold.
// The node should be able to enter and exit suspension while remaining under review.
// The node should be reinstated if it has a good online score after the review period.
// The node should be disqualified if it has a bad online score after the review period.
func TestOfflineSuspend(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		oc := planet.Satellites[0].Overlay.DB

		node, err := oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.OfflineSuspended)
		require.Nil(t, node.OfflineUnderReview)
		require.Nil(t, node.Disqualified)
		require.EqualValues(t, 1, node.Reputation.OnlineScore)

		updateReq := &overlay.UpdateRequest{
			NodeID:       nodeID,
			AuditOutcome: overlay.AuditOffline,
			AuditHistory: overlay.AuditHistoryConfig{
				WindowSize:       time.Hour,
				TrackingPeriod:   2 * time.Hour,
				GracePeriod:      time.Hour,
				OfflineThreshold: 0.6,
				OfflineDQEnabled: true,
			},

			AuditLambda:               0.95,
			AuditWeight:               1,
			AuditDQ:                   0.6,
			SuspensionGracePeriod:     time.Hour,
			SuspensionDQEnabled:       true,
			AuditsRequiredForVetting:  0,
			UptimesRequiredForVetting: 0,
		}

		// give node an offline audit
		// node's score is still 1 until its first window is complete
		nextWindowTime := time.Now()
		_, err = oc.UpdateStats(ctx, updateReq, nextWindowTime)
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.OfflineSuspended)
		require.Nil(t, node.OfflineUnderReview)
		require.Nil(t, node.Disqualified)
		require.EqualValues(t, 1, node.Reputation.OnlineScore)

		nextWindowTime = nextWindowTime.Add(updateReq.AuditHistory.WindowSize)

		// node now has one full window, so its score should be 0
		// should not be suspended or DQ since it only has 1 window out of 2 for tracking period
		_, err = oc.UpdateStats(ctx, updateReq, nextWindowTime)
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.OfflineSuspended)
		require.Nil(t, node.OfflineUnderReview)
		require.Nil(t, node.Disqualified)
		require.EqualValues(t, 0, node.Reputation.OnlineScore)

		nextWindowTime = nextWindowTime.Add(updateReq.AuditHistory.WindowSize)

		nextWindowTime, err = setOnlineScore(ctx, updateReq, 0.5, time.Hour, nextWindowTime, oc)
		require.NoError(t, err)

		// node should be offline suspended and under review
		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, node.OfflineSuspended)
		require.NotNil(t, node.OfflineUnderReview)
		require.Nil(t, node.Disqualified)
		require.EqualValues(t, 0.5, node.Reputation.OnlineScore)

		// set online score to be good, but use a long grace period so that node remains under review
		nextWindowTime, err = setOnlineScore(ctx, updateReq, 1, 100*time.Hour, nextWindowTime, oc)
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.OfflineSuspended)
		require.NotNil(t, node.OfflineUnderReview)
		require.Nil(t, node.Disqualified)
		oldUnderReview := node.OfflineUnderReview
		require.EqualValues(t, 1, node.Reputation.OnlineScore)

		// suspend again, under review should be the same
		nextWindowTime, err = setOnlineScore(ctx, updateReq, 0.5, 100*time.Hour, nextWindowTime, oc)
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, node.OfflineSuspended)
		require.NotNil(t, node.OfflineUnderReview)
		require.Nil(t, node.Disqualified)
		require.Equal(t, oldUnderReview, node.OfflineUnderReview)
		require.EqualValues(t, 0.5, node.Reputation.OnlineScore)

		// node will exit review after grace period + 1 tracking window, so set grace period to be time since put under review
		// subtract one hour so that review window ends when setOnlineScore adds the last window
		gracePeriod := nextWindowTime.Sub(*node.OfflineUnderReview) - time.Hour
		nextWindowTime, err = setOnlineScore(ctx, updateReq, 1, gracePeriod, nextWindowTime, oc)
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.OfflineSuspended)
		require.Nil(t, node.OfflineUnderReview)
		require.Nil(t, node.Disqualified)
		require.EqualValues(t, 1, node.Reputation.OnlineScore)

		// put into suspension and under review again
		nextWindowTime, err = setOnlineScore(ctx, updateReq, 0.5, 100*time.Hour, nextWindowTime, oc)
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, node.OfflineSuspended)
		require.NotNil(t, node.OfflineUnderReview)
		require.Nil(t, node.Disqualified)
		require.EqualValues(t, 0.5, node.Reputation.OnlineScore)

		// if grace period + 1 tracking window passes and online score is still bad, expect node to be DQed
		nextWindowTime, err = setOnlineScore(ctx, updateReq, 0.5, 0, nextWindowTime, oc)
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, node.OfflineSuspended)
		require.NotNil(t, node.OfflineUnderReview)
		require.NotNil(t, node.Disqualified)
		require.EqualValues(t, 0.5, node.Reputation.OnlineScore)
	})
}

func setOnlineScore(ctx context.Context, reqPtr *overlay.UpdateRequest, desiredScore float64, gracePeriod time.Duration, startTime time.Time, oc overlay.DB) (nextWindowTime time.Time, err error) {
	// for our tests, we are only using values of 1 and 0.5, so two audits per window is sufficient
	totalAudits := 2
	onlineAudits := int(float64(totalAudits) * desiredScore)
	nextWindowTime = startTime

	windowsPerTrackingPeriod := int(reqPtr.AuditHistory.TrackingPeriod.Seconds() / reqPtr.AuditHistory.WindowSize.Seconds())
	for window := 0; window < windowsPerTrackingPeriod+1; window++ {
		updateReqs := []*overlay.UpdateRequest{}
		for i := 0; i < totalAudits; i++ {
			updateReq := *reqPtr
			updateReq.AuditOutcome = overlay.AuditSuccess
			if i >= onlineAudits {
				updateReq.AuditOutcome = overlay.AuditOffline
			}
			updateReq.AuditHistory.GracePeriod = gracePeriod

			updateReqs = append(updateReqs, &updateReq)
		}
		_, err = oc.BatchUpdateStats(ctx, updateReqs, 100, nextWindowTime)
		if err != nil {
			return nextWindowTime, err
		}
		// increment nextWindowTime so in the next iteration, we are adding to a different window
		nextWindowTime = nextWindowTime.Add(reqPtr.AuditHistory.WindowSize)
	}
	return nextWindowTime, err
}
