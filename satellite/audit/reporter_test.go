// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pkcrypto"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/overlay"
)

func TestReportPendingAudits(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		audits.Worker.Loop.Pause()

		nodeID := planet.StorageNodes[0].ID()

		pending := audit.PendingAudit{
			NodeID:            nodeID,
			PieceID:           storj.NewPieceID(),
			StripeIndex:       1,
			ShareSize:         1 * memory.KiB.Int32(),
			ExpectedShareHash: pkcrypto.SHA256Hash([]byte("test")),
		}

		report := audit.Report{PendingAudits: []*audit.PendingAudit{&pending}}
		containment := satellite.DB.Containment()

		failed, err := audits.Reporter.RecordAudits(ctx, report)
		require.NoError(t, err)
		assert.Zero(t, failed)

		pa, err := containment.Get(ctx, nodeID)
		require.NoError(t, err)
		assert.Equal(t, pending, *pa)
	})
}

func TestRecordAuditsAtLeastOnce(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		audits.Worker.Loop.Pause()

		nodeID := planet.StorageNodes[0].ID()

		report := audit.Report{Successes: []storj.NodeID{nodeID}}

		// expect RecordAudits to try recording at least once (maxRetries is set to 0)
		failed, err := audits.Reporter.RecordAudits(ctx, report)
		require.NoError(t, err)
		require.Zero(t, failed)

		service := satellite.Reputation.Service
		node, err := service.Get(ctx, nodeID)
		require.NoError(t, err)
		require.EqualValues(t, 1, node.TotalAuditCount)
	})
}

// TestRecordAuditsCorrectOutcome ensures that audit successes, failures, and unknown audits result in the correct disqualification/suspension state.
func TestRecordAuditsCorrectOutcome(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.InitialAlpha = 1
				config.Reputation.AuditLambda = 0.95
				config.Reputation.AuditDQ = 0.6
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		audits.Worker.Loop.Pause()

		goodNode := planet.StorageNodes[0].ID()
		dqNode := planet.StorageNodes[1].ID()
		suspendedNode := planet.StorageNodes[2].ID()
		pendingNode := planet.StorageNodes[3].ID()
		offlineNode := planet.StorageNodes[4].ID()

		report := audit.Report{
			Successes: []storj.NodeID{goodNode},
			Fails:     []storj.NodeID{dqNode},
			Unknown:   []storj.NodeID{suspendedNode},
			PendingAudits: []*audit.PendingAudit{
				{
					NodeID:            pendingNode,
					PieceID:           testrand.PieceID(),
					StripeIndex:       0,
					ShareSize:         10,
					ExpectedShareHash: []byte{},
					ReverifyCount:     0,
				},
			},
			Offlines: []storj.NodeID{offlineNode},
		}

		failed, err := audits.Reporter.RecordAudits(ctx, report)
		require.NoError(t, err)
		require.Zero(t, failed)

		overlay := satellite.Overlay.Service
		node, err := overlay.Get(ctx, goodNode)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.Nil(t, node.UnknownAuditSuspended)

		node, err = overlay.Get(ctx, dqNode)
		require.NoError(t, err)
		require.NotNil(t, node.Disqualified)
		require.Nil(t, node.UnknownAuditSuspended)

		node, err = overlay.Get(ctx, suspendedNode)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.NotNil(t, node.UnknownAuditSuspended)

		node, err = overlay.Get(ctx, pendingNode)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.Nil(t, node.UnknownAuditSuspended)

		node, err = overlay.Get(ctx, offlineNode)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.Nil(t, node.UnknownAuditSuspended)
	})
}

func TestSuspensionTimeNotResetBySuccessiveAudit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		audits.Worker.Loop.Pause()

		suspendedNode := planet.StorageNodes[0].ID()

		failed, err := audits.Reporter.RecordAudits(ctx, audit.Report{Unknown: []storj.NodeID{suspendedNode}})
		require.NoError(t, err)
		require.Zero(t, failed)

		overlay := satellite.Overlay.Service

		node, err := overlay.Get(ctx, suspendedNode)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.NotNil(t, node.UnknownAuditSuspended)

		suspendedAt := node.UnknownAuditSuspended

		failed, err = audits.Reporter.RecordAudits(ctx, audit.Report{Unknown: []storj.NodeID{suspendedNode}})
		require.NoError(t, err)
		require.Zero(t, failed)

		node, err = overlay.Get(ctx, suspendedNode)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.NotNil(t, node.UnknownAuditSuspended)
		require.Equal(t, suspendedAt, node.UnknownAuditSuspended)
	})
}

// TestGracefullyExitedNotUpdated verifies that a gracefully exited node's reputation, suspension,
// and disqualification flags are not updated when an audit is reported for that node.
func TestGracefullyExitedNotUpdated(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		audits.Worker.Loop.Pause()
		cache := satellite.Overlay.DB
		reputationDB := satellite.DB.Reputation()

		successNode := planet.StorageNodes[0]
		failedNode := planet.StorageNodes[1]
		containedNode := planet.StorageNodes[2]
		unknownNode := planet.StorageNodes[3]
		offlineNode := planet.StorageNodes[4]
		nodeList := []*testplanet.StorageNode{successNode, failedNode, containedNode, unknownNode, offlineNode}

		report := audit.Report{
			Successes: storj.NodeIDList{successNode.ID(), failedNode.ID(), containedNode.ID(), unknownNode.ID(), offlineNode.ID()},
		}
		failed, err := audits.Reporter.RecordAudits(ctx, report)
		require.NoError(t, err)
		assert.Zero(t, failed)

		// mark each node as having gracefully exited
		for _, node := range nodeList {
			req := &overlay.ExitStatusRequest{
				NodeID:              node.ID(),
				ExitInitiatedAt:     time.Now(),
				ExitLoopCompletedAt: time.Now(),
				ExitFinishedAt:      time.Now(),
			}
			_, err := cache.UpdateExitStatus(ctx, req)
			require.NoError(t, err)
		}

		pending := audit.PendingAudit{
			NodeID:            containedNode.ID(),
			PieceID:           storj.NewPieceID(),
			StripeIndex:       1,
			ShareSize:         1 * memory.KiB.Int32(),
			ExpectedShareHash: pkcrypto.SHA256Hash([]byte("test")),
		}
		report = audit.Report{
			Successes:     storj.NodeIDList{successNode.ID()},
			Fails:         storj.NodeIDList{failedNode.ID()},
			Offlines:      storj.NodeIDList{offlineNode.ID()},
			PendingAudits: []*audit.PendingAudit{&pending},
			Unknown:       storj.NodeIDList{unknownNode.ID()},
		}
		failed, err = audits.Reporter.RecordAudits(ctx, report)
		require.NoError(t, err)
		assert.Zero(t, failed)

		// since every node has gracefully exit, reputation, dq, and suspension should remain at default values
		for _, node := range nodeList {
			nodeCacheInfo, err := reputationDB.Get(ctx, node.ID())
			require.NoError(t, err)

			require.Nil(t, nodeCacheInfo.UnknownAuditSuspended)
			require.Nil(t, nodeCacheInfo.Disqualified)
		}
	})
}

func TestReportOfflineAudits(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		node := planet.StorageNodes[0]
		audits := satellite.Audit
		audits.Worker.Loop.Pause()
		reputationService := satellite.Core.Reputation.Service

		_, err := audits.Reporter.RecordAudits(ctx, audit.Report{Offlines: storj.NodeIDList{node.ID()}})
		require.NoError(t, err)

		info, err := reputationService.Get(ctx, node.ID())
		require.NoError(t, err)
		require.Equal(t, int64(1), info.TotalAuditCount)

		// check that other reputation stats were not incorrectly updated by offline audit
		require.EqualValues(t, 0, info.AuditSuccessCount)
		require.EqualValues(t, satellite.Config.Reputation.InitialAlpha, info.AuditReputationAlpha)
		require.EqualValues(t, satellite.Config.Reputation.InitialBeta, info.AuditReputationBeta)
		require.EqualValues(t, 1, info.UnknownAuditReputationAlpha)
		require.EqualValues(t, 0, info.UnknownAuditReputationBeta)
	})
}
