// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation_test

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
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
)

// TestAuditSuspendBasic ensures that we can suspend a node using overlayService.SuspendNode and that we can unsuspend a node using overlayservice.UnsuspendNode.
func TestAuditSuspendBasic(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		repService := planet.Satellites[0].Reputation.Service
		oc := planet.Satellites[0].Overlay.DB

		node, err := oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.UnknownAuditSuspended)

		timeToSuspend := time.Now().UTC().Truncate(time.Second)
		err = repService.TestSuspendNodeUnknownAudit(ctx, nodeID, timeToSuspend)
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, node.UnknownAuditSuspended)
		require.True(t, node.UnknownAuditSuspended.Equal(timeToSuspend))

		err = repService.TestUnsuspendNodeUnknownAudit(ctx, nodeID)
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
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.AuditWeight = 1
				config.Reputation.AuditDQ = 0.6
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		satellite := planet.Satellites[0]
		oc := satellite.Overlay.Service
		repService := satellite.Reputation.Service

		node, err := oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.UnknownAuditSuspended)
		testStartTime := time.Now()

		// give node one unknown audit - bringing unknown audit rep to 0.5, and suspending node
		err = repService.ApplyAudit(ctx, nodeID, node.Reputation.Status, reputation.AuditUnknown)
		require.NoError(t, err)

		reputationInfo, err := repService.Get(ctx, nodeID)
		require.NoError(t, err)
		// expect unknown audit alpha/beta to change and suspended to be set
		require.True(t, reputationInfo.UnknownAuditReputationAlpha < 1)
		require.True(t, reputationInfo.UnknownAuditReputationBeta > 0)
		require.NotNil(t, reputationInfo.UnknownAuditSuspended)
		require.True(t, reputationInfo.UnknownAuditSuspended.After(testStartTime))
		// expect normal audit alpha/beta remain unchanged
		require.EqualValues(t, reputationInfo.AuditReputationAlpha, satellite.Config.Reputation.InitialAlpha)
		require.EqualValues(t, reputationInfo.AuditReputationBeta, satellite.Config.Reputation.InitialBeta)
		// expect node is not disqualified
		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)

		// give node two successful audits - bringing unknown audit rep to 0.75, and unsuspending node
		for i := 0; i < 2; i++ {
			err = repService.ApplyAudit(ctx, nodeID, node.Reputation.Status, reputation.AuditSuccess)
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
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.InitialAlpha = 1.0
				config.Reputation.AuditLambda = 1.0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		oc := planet.Satellites[0].Overlay.DB
		repService := planet.Satellites[0].Reputation.Service

		node, err := oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.Nil(t, node.UnknownAuditSuspended)

		// give node one failed audit - bringing audit rep to 0.5, and disqualifying node
		// expect that suspended field and unknown audit reputation remain unchanged
		err = repService.ApplyAudit(ctx, nodeID, node.Reputation.Status, reputation.AuditFailure)
		require.NoError(t, err)

		node, err = oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, node.Disqualified)
		require.Nil(t, node.UnknownAuditSuspended)
		reputationInfo, err := repService.Get(ctx, nodeID)
		require.NoError(t, err)
		require.EqualValues(t, reputationInfo.UnknownAuditReputationAlpha, 1)
		require.EqualValues(t, reputationInfo.UnknownAuditReputationBeta, 0)
	})
}

// TestAuditSuspendExceedGracePeriod ensures that a node is disqualified when it receives a failing or unknown audit after the grace period expires.
func TestAuditSuspendExceedGracePeriod(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.SuspensionGracePeriod = time.Hour
				config.Reputation.InitialAlpha = 1
				config.Reputation.AuditLambda = 0.95
				config.Reputation.AuditDQ = 0.6
				// disable write cache so changes are immediate
				config.Reputation.FlushInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		successNodeID := planet.StorageNodes[0].ID()
		failNodeID := planet.StorageNodes[1].ID()
		offlineNodeID := planet.StorageNodes[2].ID()
		unknownNodeID := planet.StorageNodes[3].ID()

		// suspend each node two hours ago (more than grace period)
		repService := planet.Satellites[0].Reputation.Service
		for _, node := range (storj.NodeIDList{successNodeID, failNodeID, offlineNodeID, unknownNodeID}) {
			err := repService.TestSuspendNodeUnknownAudit(ctx, node, time.Now().Add(-2*time.Hour))
			require.NoError(t, err)
		}

		nodesStatus := make(map[storj.NodeID]overlay.ReputationStatus)
		// no nodes should be disqualified
		for _, node := range (storj.NodeIDList{successNodeID, failNodeID, offlineNodeID, unknownNodeID}) {
			n, err := repService.Get(ctx, node)
			require.NoError(t, err)
			require.Nil(t, n.Disqualified)
			nodesStatus[node] = overlay.ReputationStatus{
				Disqualified:          n.Disqualified,
				UnknownAuditSuspended: n.UnknownAuditSuspended,
				OfflineSuspended:      n.OfflineSuspended,
				VettedAt:              n.VettedAt,
			}
		}

		// give one node a successful audit, one a failed audit, one an offline audit, and one an unknown audit
		report := audit.Report{
			Successes:       storj.NodeIDList{successNodeID},
			Fails:           metabase.Pieces{{StorageNode: failNodeID}},
			Offlines:        storj.NodeIDList{offlineNodeID},
			Unknown:         storj.NodeIDList{unknownNodeID},
			NodesReputation: nodesStatus,
		}
		auditService := planet.Satellites[0].Audit
		auditService.Reporter.RecordAudits(ctx, report)

		// success and offline nodes should not be disqualified
		// fail and unknown nodes should be disqualified
		for _, node := range (storj.NodeIDList{successNodeID, offlineNodeID}) {
			n, err := repService.Get(ctx, node)
			require.NoError(t, err)
			require.Nil(t, n.Disqualified)
		}
		for _, node := range (storj.NodeIDList{failNodeID, unknownNodeID}) {
			n, err := repService.Get(ctx, node)
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
				config.Reputation.SuspensionGracePeriod = time.Hour
				config.Reputation.SuspensionDQEnabled = false
				config.Reputation.InitialAlpha = 1
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		successNodeID := planet.StorageNodes[0].ID()
		failNodeID := planet.StorageNodes[1].ID()
		offlineNodeID := planet.StorageNodes[2].ID()
		unknownNodeID := planet.StorageNodes[3].ID()

		// suspend each node two hours ago (more than grace period)
		oc := planet.Satellites[0].DB.OverlayCache()
		repService := planet.Satellites[0].Reputation.Service
		for _, node := range (storj.NodeIDList{successNodeID, failNodeID, offlineNodeID, unknownNodeID}) {
			err := repService.TestSuspendNodeUnknownAudit(ctx, node, time.Now().Add(-2*time.Hour))
			require.NoError(t, err)
		}
		nodesStatus := make(map[storj.NodeID]overlay.ReputationStatus)

		// no nodes should be disqualified
		for _, node := range (storj.NodeIDList{successNodeID, failNodeID, offlineNodeID, unknownNodeID}) {
			n, err := oc.Get(ctx, node)
			require.NoError(t, err)
			require.Nil(t, n.Disqualified)
			nodesStatus[node] = overlay.ReputationStatus{
				Disqualified:          n.Disqualified,
				UnknownAuditSuspended: n.UnknownAuditSuspended,
				OfflineSuspended:      n.OfflineSuspended,
				VettedAt:              n.Reputation.Status.VettedAt,
			}

		}

		// give one node a successful audit, one a failed audit, one an offline audit, and one an unknown audit
		report := audit.Report{
			Successes:       storj.NodeIDList{successNodeID},
			Fails:           metabase.Pieces{{StorageNode: failNodeID}},
			Offlines:        storj.NodeIDList{offlineNodeID},
			Unknown:         storj.NodeIDList{unknownNodeID},
			NodesReputation: nodesStatus,
		}
		auditService := planet.Satellites[0].Audit
		auditService.Reporter.RecordAudits(ctx, report)

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

// TestOfflineAuditSuspensionDisabled ensures that a node is not suspended if the offline suspension enabled flag is false.
func TestOfflineAuditSuspensionDisabled(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.AuditHistory.OfflineSuspensionEnabled = false
				config.Reputation.AuditHistory.WindowSize = time.Hour
				config.Reputation.AuditHistory.TrackingPeriod = 2 * time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		oc := planet.Satellites[0].Overlay.DB
		reputationdb := planet.Satellites[0].DB.Reputation()
		config := planet.Satellites[0].Config.Reputation.AuditHistory

		node, err := oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.OfflineSuspended)
		require.Nil(t, node.OfflineUnderReview)
		require.Nil(t, node.Disqualified)

		windowSize := config.WindowSize
		trackingPeriodLength := config.TrackingPeriod
		currentWindow := time.Now()

		req := reputation.UpdateRequest{
			NodeID:       nodeID,
			AuditOutcome: reputation.AuditOffline,
			Config: reputation.Config{
				AuditHistory: config,
			},
		}

		// check that unsuspended node does not get suspended
		for i := 0; i <= int(trackingPeriodLength/windowSize); i++ {
			_, err = reputationdb.Update(ctx, req, currentWindow)
			require.NoError(t, err)
			currentWindow = currentWindow.Add(windowSize)
		}

		reputationInfo, err := reputationdb.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, reputationInfo.OfflineSuspended)
		require.Nil(t, reputationInfo.UnderReview)
		require.Less(t, reputationInfo.OnlineScore, config.OfflineThreshold)

		// check that enabling flag suspends the node
		req.AuditHistory.OfflineSuspensionEnabled = true
		_, err = reputationdb.Update(ctx, req, currentWindow)
		require.NoError(t, err)

		reputationInfo, err = reputationdb.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, reputationInfo.OfflineSuspended)
		require.NotNil(t, reputationInfo.UnderReview)
		require.Less(t, reputationInfo.OnlineScore, config.OfflineThreshold)

		// check that disabling flag clears suspension and under review
		req.AuditHistory.OfflineSuspensionEnabled = false
		_, err = reputationdb.Update(ctx, req, currentWindow)
		require.NoError(t, err)

		reputationInfo, err = reputationdb.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Less(t, reputationInfo.OnlineScore, config.OfflineThreshold)
		require.Nil(t, reputationInfo.OfflineSuspended)
		require.Nil(t, reputationInfo.UnderReview)
	})
}

// TestOfflineSuspend tests that a node enters offline suspension and "under review" when online score passes below threshold.
// The node should be able to enter and exit suspension while remaining under review.
// The node should be reinstated if it has a good online score after the review period.
// The node should be disqualified if it has a bad online score after the review period.
func TestOfflineSuspend(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.AuditHistory.OfflineSuspensionEnabled = false
				config.Reputation.AuditHistory.WindowSize = time.Hour
				config.Reputation.AuditHistory.TrackingPeriod = 2 * time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeID := planet.StorageNodes[0].ID()
		reputationdb := planet.Satellites[0].DB.Reputation()
		oc := planet.Satellites[0].DB.OverlayCache()

		node, err := oc.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, node.OfflineSuspended)
		require.Nil(t, node.Disqualified)

		updateReq := reputation.UpdateRequest{
			NodeID:       nodeID,
			AuditOutcome: reputation.AuditOffline,
			Config: reputation.Config{
				AuditHistory: reputation.AuditHistoryConfig{
					WindowSize:               time.Hour,
					TrackingPeriod:           2 * time.Hour,
					GracePeriod:              time.Hour,
					OfflineThreshold:         0.6,
					OfflineDQEnabled:         true,
					OfflineSuspensionEnabled: true,
				},

				AuditLambda:           0.95,
				AuditWeight:           1,
				AuditDQ:               0.6,
				InitialAlpha:          1000,
				InitialBeta:           0,
				UnknownAuditDQ:        0.6,
				UnknownAuditLambda:    0.95,
				SuspensionGracePeriod: time.Hour,
				SuspensionDQEnabled:   true,
				AuditCount:            0,
			},
		}

		// give node an offline audit
		// node's score is still 1 until its first window is complete
		nextWindowTime := time.Now()
		_, err = reputationdb.Update(ctx, updateReq, nextWindowTime)
		require.NoError(t, err)

		reputationInfo, err := reputationdb.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, reputationInfo.OfflineSuspended)
		require.Nil(t, reputationInfo.UnderReview)
		require.Nil(t, reputationInfo.Disqualified)
		require.EqualValues(t, 1, reputationInfo.OnlineScore)

		nextWindowTime = nextWindowTime.Add(updateReq.AuditHistory.WindowSize)

		// node now has one full window, so its score should be 0
		// should not be suspended or DQ since it only has 1 window out of 2 for tracking period
		_, err = reputationdb.Update(ctx, updateReq, nextWindowTime)
		require.NoError(t, err)

		reputationInfo, err = reputationdb.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, reputationInfo.OfflineSuspended)
		require.Nil(t, reputationInfo.UnderReview)
		require.Nil(t, reputationInfo.Disqualified)
		require.EqualValues(t, 0, reputationInfo.OnlineScore)

		nextWindowTime = nextWindowTime.Add(updateReq.AuditHistory.WindowSize)

		nextWindowTime, err = setOnlineScore(ctx, updateReq, 0.5, time.Hour, nextWindowTime, reputationdb)
		require.NoError(t, err)

		// node should be offline suspended and under review
		reputationInfo, err = reputationdb.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, reputationInfo.OfflineSuspended)
		require.NotNil(t, reputationInfo.UnderReview)
		require.Nil(t, node.Disqualified)
		require.EqualValues(t, 0.5, reputationInfo.OnlineScore)

		// set online score to be good, but use a long grace period so that node remains under review
		nextWindowTime, err = setOnlineScore(ctx, updateReq, 1, 100*time.Hour, nextWindowTime, reputationdb)
		require.NoError(t, err)

		reputationInfo, err = reputationdb.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, reputationInfo.OfflineSuspended)
		require.NotNil(t, reputationInfo.UnderReview)
		require.Nil(t, reputationInfo.Disqualified)
		oldUnderReview := reputationInfo.UnderReview
		require.EqualValues(t, 1, reputationInfo.OnlineScore)

		// suspend again, under review should be the same
		nextWindowTime, err = setOnlineScore(ctx, updateReq, 0.5, 100*time.Hour, nextWindowTime, reputationdb)
		require.NoError(t, err)

		reputationInfo, err = reputationdb.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, reputationInfo.OfflineSuspended)
		require.NotNil(t, reputationInfo.UnderReview)
		require.Nil(t, node.Disqualified)
		require.Equal(t, oldUnderReview, reputationInfo.UnderReview)
		require.EqualValues(t, 0.5, reputationInfo.OnlineScore)

		// node will exit review after grace period + 1 tracking window, so set grace period to be time since put under review
		// subtract one hour so that review window ends when setOnlineScore adds the last window
		gracePeriod := nextWindowTime.Sub(*reputationInfo.UnderReview) - time.Hour
		nextWindowTime, err = setOnlineScore(ctx, updateReq, 1, gracePeriod, nextWindowTime, reputationdb)
		require.NoError(t, err)

		reputationInfo, err = reputationdb.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Nil(t, reputationInfo.OfflineSuspended)
		require.Nil(t, reputationInfo.UnderReview)
		require.Nil(t, reputationInfo.Disqualified)
		require.EqualValues(t, 1, reputationInfo.OnlineScore)

		// put into suspension and under review again
		nextWindowTime, err = setOnlineScore(ctx, updateReq, 0.5, 100*time.Hour, nextWindowTime, reputationdb)
		require.NoError(t, err)

		reputationInfo, err = reputationdb.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, reputationInfo.OfflineSuspended)
		require.NotNil(t, reputationInfo.UnderReview)
		require.Nil(t, node.Disqualified)
		require.EqualValues(t, 0.5, reputationInfo.OnlineScore)

		// if grace period + 1 tracking window passes and online score is still bad, expect node to be DQed
		_, err = setOnlineScore(ctx, updateReq, 0.5, 0, nextWindowTime, reputationdb)
		require.NoError(t, err)

		reputationInfo, err = reputationdb.Get(ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, reputationInfo.OfflineSuspended)
		require.NotNil(t, reputationInfo.UnderReview)
		require.NotNil(t, reputationInfo.Disqualified)
		require.EqualValues(t, 0.5, reputationInfo.OnlineScore)
	})
}

func setOnlineScore(ctx context.Context, reqPtr reputation.UpdateRequest, desiredScore float64, gracePeriod time.Duration, startTime time.Time, reputationdb reputation.DB) (nextWindowTime time.Time, err error) {
	// for our tests, we are only using values of 1 and 0.5, so two audits per window is sufficient
	totalAudits := 2
	onlineAudits := int(float64(totalAudits) * desiredScore)
	nextWindowTime = startTime

	windowsPerTrackingPeriod := int(reqPtr.AuditHistory.TrackingPeriod.Seconds() / reqPtr.AuditHistory.WindowSize.Seconds())
	for window := 0; window < windowsPerTrackingPeriod+1; window++ {
		for i := 0; i < totalAudits; i++ {
			updateReq := reqPtr
			updateReq.AuditOutcome = reputation.AuditSuccess
			if i >= onlineAudits {
				updateReq.AuditOutcome = reputation.AuditOffline
			}
			updateReq.AuditHistory.GracePeriod = gracePeriod

			_, err = reputationdb.Update(ctx, updateReq, nextWindowTime)
			if err != nil {
				return nextWindowTime, err
			}
		}
		// increment nextWindowTime so in the next iteration, we are adding to a different window
		nextWindowTime = nextWindowTime.Add(reqPtr.AuditHistory.WindowSize)
	}
	return nextWindowTime, err
}
