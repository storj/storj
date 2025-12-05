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
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
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

		pending := audit.ReverificationJob{
			Locator: audit.PieceLocator{
				NodeID: nodeID,
			},
		}

		report := audit.Report{PendingAudits: []*audit.ReverificationJob{&pending}}
		containment := satellite.DB.Containment()

		audits.Reporter.RecordAudits(ctx, report)

		pa, err := containment.Get(ctx, nodeID)
		require.NoError(t, err)
		assert.Equal(t, pending.Locator, pa.Locator)
	})
}

func TestRecordAuditsAtLeastOnce(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// disable reputation write cache so changes are immediate
				config.Reputation.FlushInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		audits.Worker.Loop.Pause()

		nodeID := planet.StorageNodes[0].ID()

		report := audit.Report{Successes: []storj.NodeID{nodeID}}

		// expect RecordAudits to try recording at least once (maxRetries is set to 0)
		audits.Reporter.RecordAudits(ctx, report)

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
			Fails:     metabase.Pieces{{StorageNode: dqNode}},
			Unknown:   []storj.NodeID{suspendedNode},
			PendingAudits: []*audit.ReverificationJob{
				{
					Locator:       audit.PieceLocator{NodeID: pendingNode},
					ReverifyCount: 0,
				},
			},
			Offlines: []storj.NodeID{offlineNode},
		}

		audits.Reporter.RecordAudits(ctx, report)

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

		audits.Reporter.RecordAudits(ctx, audit.Report{Unknown: []storj.NodeID{suspendedNode}})

		overlay := satellite.Overlay.Service

		node, err := overlay.Get(ctx, suspendedNode)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.NotNil(t, node.UnknownAuditSuspended)

		suspendedAt := node.UnknownAuditSuspended

		audits.Reporter.RecordAudits(ctx, audit.Report{Unknown: []storj.NodeID{suspendedNode}})

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
		audits.Reporter.RecordAudits(ctx, report)

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

		pending := audit.ReverificationJob{
			Locator: audit.PieceLocator{
				NodeID: containedNode.ID(),
			},
		}
		report = audit.Report{
			Successes:     storj.NodeIDList{successNode.ID()},
			Fails:         metabase.Pieces{{StorageNode: failedNode.ID()}},
			Offlines:      storj.NodeIDList{offlineNode.ID()},
			PendingAudits: []*audit.ReverificationJob{&pending},
			Unknown:       storj.NodeIDList{unknownNode.ID()},
		}
		audits.Reporter.RecordAudits(ctx, report)

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
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// disable reputation write cache so changes are immediate
				config.Reputation.FlushInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		node := planet.StorageNodes[0]
		audits := satellite.Audit
		audits.Worker.Loop.Pause()
		reputationService := satellite.Core.Reputation.Service

		audits.Reporter.RecordAudits(ctx, audit.Report{Offlines: storj.NodeIDList{node.ID()}})

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

func TestReportingAuditFailureResultsInRemovalOfPiece(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					// disable reputation write cache so changes are immediate
					config.Reputation.FlushInterval = 0
				},
				testplanet.ReconfigureRS(4, 5, 6, 6),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]

		testData := testrand.Bytes(1 * memory.MiB)
		err := ul.Upload(ctx, satellite, "bucket-for-test", "path/of/testness", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, ul.Projects[0].ID, "bucket-for-test")

		report := audit.Report{
			Segment: &segment,
			Fails: metabase.Pieces{
				metabase.Piece{
					Number:      segment.Pieces[0].Number,
					StorageNode: segment.Pieces[0].StorageNode,
				},
			},
		}

		satellite.Audit.Reporter.RecordAudits(ctx, report)

		// piece marked as failed is no longer in the segment
		afterSegment, _ := getRemoteSegment(ctx, t, satellite, ul.Projects[0].ID, "bucket-for-test")
		require.Len(t, afterSegment.Pieces, len(segment.Pieces)-1)
		for i, p := range afterSegment.Pieces {
			assert.NotEqual(t, segment.Pieces[0].Number, p.Number, i)
			assert.NotEqual(t, segment.Pieces[0].StorageNode, p.StorageNode, i)
		}

		// segment is still retrievable
		gotData, err := ul.Download(ctx, satellite, "bucket-for-test", "path/of/testness")
		require.NoError(t, err)
		require.Equal(t, testData, gotData)
	})
}
