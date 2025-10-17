// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/shared/mud"
	"storj.io/storj/shared/mudplanet"
	"storj.io/storj/shared/mudplanet/satellitetest"
)

func TestReportPendingAudits(t *testing.T) {
	mudplanet.Run(t, satellitetest.WithDB(
		mudplanet.NewComponent("satellite", satellitetest.Satellite,
			mudplanet.WithRunning[*audit.DBReporter](),
			mudplanet.WithModule(WithoutCache()),
		),
	), func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
		reporter := mudplanet.FindFirst[*audit.DBReporter](t, run, "satellite", 0)
		containment := mudplanet.FindFirst[audit.Containment](t, run, "satellite", 0)

		nodeID := testidentity.MustPregeneratedIdentity(0, storj.LatestIDVersion()).ID

		pending := audit.ReverificationJob{
			Locator: audit.PieceLocator{
				NodeID: nodeID,
			},
		}

		report := audit.Report{PendingAudits: []*audit.ReverificationJob{&pending}}

		reporter.RecordAudits(ctx, report)

		pa, err := containment.Get(ctx, nodeID)
		require.NoError(t, err)
		assert.Equal(t, pending.Locator, pa.Locator)
	})
}

func TestRecordAuditsAtLeastOnce(t *testing.T) {
	mudplanet.Run(t, satellitetest.WithDB(
		mudplanet.NewComponent("satellite", satellitetest.Satellite,
			mudplanet.WithSelector(mud.Or(mud.SelectIfExists[*audit.DBReporter](), mud.SelectIfExists[*reputation.Service]())),
			mudplanet.WithModule(WithoutCache()),
			mudplanet.WithConfig[*reputation.Config](func(cfg *reputation.Config) {
				cfg.FlushInterval = 0
			}),
		),
	), func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
		reporter := mudplanet.FindFirst[*audit.DBReporter](t, run, "satellite", 0)
		service := mudplanet.FindFirst[*reputation.Service](t, run, "satellite", 0)

		overlayDB := mudplanet.FindFirst[overlay.DB](t, run, "satellite", 0)

		nodeID := createNode(t, ctx, overlayDB, 0)

		report := audit.Report{Successes: []storj.NodeID{nodeID}}

		// expect RecordAudits to try recording at least once (maxRetries is set to 0)
		reporter.RecordAudits(ctx, report)

		node, err := service.Get(ctx, nodeID)
		require.NoError(t, err)
		require.EqualValues(t, int64(1), node.TotalAuditCount)
	})
}

// TestRecordAuditsCorrectOutcome ensures that audit successes, failures, and unknown audits result in the correct disqualification/suspension state.
func TestRecordAuditsCorrectOutcome(t *testing.T) {
	mudplanet.Run(t, satellitetest.WithDB(
		mudplanet.NewComponent("satellite", satellitetest.Satellite,
			mudplanet.WithRunning[*audit.DBReporter](),
			mudplanet.WithModule(WithoutCache()),
			mudplanet.WithConfig[*reputation.Config](func(cfg *reputation.Config) {
				cfg.InitialAlpha = 1
				cfg.AuditLambda = 0.95
				cfg.AuditDQ = 0.6
			}),
		),
	), func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {

		reporter := mudplanet.FindFirst[*audit.DBReporter](t, run, "satellite", 0)

		overlayDB := mudplanet.FindFirst[overlay.DB](t, run, "satellite", 0)

		goodNode := createNode(t, ctx, overlayDB, 0)
		dqNode := createNode(t, ctx, overlayDB, 1)
		suspendedNode := createNode(t, ctx, overlayDB, 2)
		pendingNode := createNode(t, ctx, overlayDB, 3)
		offlineNode := createNode(t, ctx, overlayDB, 4)

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

		reporter.RecordAudits(ctx, report)

		node, err := overlayDB.Get(ctx, goodNode)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.Nil(t, node.UnknownAuditSuspended)

		node, err = overlayDB.Get(ctx, dqNode)
		require.NoError(t, err)
		require.NotNil(t, node.Disqualified)
		require.Nil(t, node.UnknownAuditSuspended)

		node, err = overlayDB.Get(ctx, suspendedNode)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.NotNil(t, node.UnknownAuditSuspended)

		node, err = overlayDB.Get(ctx, pendingNode)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.Nil(t, node.UnknownAuditSuspended)

		node, err = overlayDB.Get(ctx, offlineNode)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.Nil(t, node.UnknownAuditSuspended)
	})
}

func TestSuspensionTimeNotResetBySuccessiveAudit(t *testing.T) {
	mudplanet.Run(t, satellitetest.WithDB(
		mudplanet.NewComponent("satellite", satellitetest.Satellite,
			mudplanet.WithRunning[*audit.DBReporter](),
			mudplanet.WithModule(WithoutCache()),
		),
	), func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
		reporter := mudplanet.FindFirst[*audit.DBReporter](t, run, "satellite", 0)
		overlayDB := mudplanet.FindFirst[overlay.DB](t, run, "satellite", 0)

		suspendedNode := createNode(t, ctx, overlayDB, 4)

		reporter.RecordAudits(ctx, audit.Report{Unknown: []storj.NodeID{suspendedNode}})

		node, err := overlayDB.Get(ctx, suspendedNode)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.NotNil(t, node.UnknownAuditSuspended)

		suspendedAt := node.UnknownAuditSuspended

		reporter.RecordAudits(ctx, audit.Report{Unknown: []storj.NodeID{suspendedNode}})

		node, err = overlayDB.Get(ctx, suspendedNode)
		require.NoError(t, err)
		require.Nil(t, node.Disqualified)
		require.NotNil(t, node.UnknownAuditSuspended)
		require.Equal(t, suspendedAt, node.UnknownAuditSuspended)
	})
}

// TestGracefullyExitedNotUpdated verifies that a gracefully exited node's reputation, suspension,
// and disqualification flags are not updated when an audit is reported for that node.
func TestGracefullyExitedNotUpdated(t *testing.T) {
	mudplanet.Run(t, satellitetest.WithDB(
		mudplanet.NewComponent("satellite", satellitetest.Satellite,
			mudplanet.WithRunning[*audit.DBReporter](),
			mudplanet.WithModule(WithoutCache()),
		),
	), func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
		reporter := mudplanet.FindFirst[*audit.DBReporter](t, run, "satellite", 0)
		overlayDB := mudplanet.FindFirst[overlay.DB](t, run, "satellite", 0)

		successNode := createNode(t, ctx, overlayDB, 4)
		failedNode := createNode(t, ctx, overlayDB, 5)
		containedNode := createNode(t, ctx, overlayDB, 6)
		unknownNode := createNode(t, ctx, overlayDB, 7)
		offlineNode := createNode(t, ctx, overlayDB, 8)

		nodeIDs := storj.NodeIDList{successNode, failedNode, containedNode, unknownNode, offlineNode}

		report := audit.Report{
			Successes: nodeIDs,
		}
		reporter.RecordAudits(ctx, report)

		// mark each node as having gracefully exited
		for _, node := range nodeIDs {
			req := &overlay.ExitStatusRequest{
				NodeID:              node,
				ExitInitiatedAt:     time.Now(),
				ExitLoopCompletedAt: time.Now(),
				ExitFinishedAt:      time.Now(),
			}
			_, err := overlayDB.UpdateExitStatus(ctx, req)
			require.NoError(t, err)
		}

		pending := audit.ReverificationJob{
			Locator: audit.PieceLocator{
				NodeID: containedNode,
			},
		}
		report = audit.Report{
			Successes:     storj.NodeIDList{successNode},
			Fails:         metabase.Pieces{{StorageNode: failedNode}},
			Offlines:      storj.NodeIDList{offlineNode},
			PendingAudits: []*audit.ReverificationJob{&pending},
			Unknown:       storj.NodeIDList{unknownNode},
		}
		reporter.RecordAudits(ctx, report)

		reputationDB := mudplanet.FindFirst[reputation.DB](t, run, "satellite", 0)
		// since every node has gracefully exit, reputation, dq, and suspension should remain at default values
		for _, node := range nodeIDs {
			nodeCacheInfo, err := reputationDB.Get(ctx, node)
			require.NoError(t, err)

			require.Nil(t, nodeCacheInfo.UnknownAuditSuspended)
			require.Nil(t, nodeCacheInfo.Disqualified)
		}
	})
}

func TestReportOfflineAudits(t *testing.T) {
	mudplanet.Run(t, satellitetest.WithDB(
		mudplanet.NewComponent("satellite", satellitetest.Satellite,
			mudplanet.WithSelector(mud.Or(mud.SelectIfExists[*audit.DBReporter](), mud.SelectIfExists[*reputation.Service]())),
			mudplanet.WithModule(WithoutCache()),
		),
	), func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
		reporter := mudplanet.FindFirst[*audit.DBReporter](t, run, "satellite", 0)
		overlayDB := mudplanet.FindFirst[overlay.DB](t, run, "satellite", 0)
		node := createNode(t, ctx, overlayDB, 1)

		reporter.RecordAudits(ctx, audit.Report{Offlines: storj.NodeIDList{node}})

		reputationService := mudplanet.FindFirst[*reputation.Service](t, run, "satellite", 0)
		info, err := reputationService.Get(ctx, node)
		require.NoError(t, err)
		require.Equal(t, int64(1), info.TotalAuditCount)

		cfg := mudplanet.FindFirst[*reputation.Config](t, run, "satellite", 0)
		// check that other reputation stats were not incorrectly updated by offline audit
		require.EqualValues(t, 0, info.AuditSuccessCount)
		require.EqualValues(t, cfg.InitialAlpha, info.AuditReputationAlpha)
		require.EqualValues(t, cfg.InitialBeta, info.AuditReputationBeta)
		require.EqualValues(t, 1, info.UnknownAuditReputationAlpha)
		require.EqualValues(t, 0, info.UnknownAuditReputationBeta)
	})
}

func TestReportingAuditFailureResultsInRemovalOfPiece(t *testing.T) {
	mudplanet.Run(t, satellitetest.WithDB(
		mudplanet.NewComponent("satellite", satellitetest.Satellite,
			mudplanet.WithRunning[*audit.DBReporter](),
			mudplanet.WithModule(WithoutCache()),
		),
	), func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
		overlayDB := mudplanet.FindFirst[overlay.DB](t, run, "satellite", 0)

		var pieces []metabase.Piece
		for i := 0; i < 12; i++ {
			pieces = append(pieces, metabase.Piece{
				Number:      uint16(i),
				StorageNode: createNode(t, ctx, overlayDB, i),
			})
		}
		segment := metabase.SegmentForAudit{
			StreamID: testrand.UUID(),
			Pieces:   pieces,
			Redundancy: storj.RedundancyScheme{
				Algorithm:      storj.ReedSolomon,
				ShareSize:      128,
				RequiredShares: 10,
				RepairShares:   12,
				OptimalShares:  14,
				TotalShares:    16,
			},
		}

		metabaseDB := mudplanet.FindFirst[*metabase.DB](t, run, "satellite", 0)

		err := metabaseDB.TestingBatchInsertSegments(ctx, []metabase.RawSegment{
			{
				StreamID:          segment.StreamID,
				Position:          metabase.SegmentPosition{},
				CreatedAt:         segment.CreatedAt,
				RootPieceID:       segment.RootPieceID,
				Pieces:            segment.Pieces,
				EncryptedKeyNonce: testrand.Bytes(32),
				EncryptedKey:      testrand.Bytes(32),
				Redundancy:        segment.Redundancy,
			},
		})
		require.NoError(t, err)

		report := audit.Report{
			Segment: &segment,
			Fails: metabase.Pieces{
				metabase.Piece{
					Number:      segment.Pieces[0].Number,
					StorageNode: segment.Pieces[0].StorageNode,
				},
			},
		}

		reporter := mudplanet.FindFirst[*audit.DBReporter](t, run, "satellite", 0)
		reporter.RecordAudits(ctx, report)

		// check if piece marked as failed is no longer in the segment
		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		afterSegment := segments[0]

		require.Len(t, afterSegment.Pieces, len(segment.Pieces)-1)
		for i, p := range afterSegment.Pieces {
			assert.NotEqual(t, segment.Pieces[0].Number, p.Number, i)
			assert.NotEqual(t, segment.Pieces[0].StorageNode, p.StorageNode, i)
		}

	})
}

//nolint:revive,context-as-argument
func createNode(t *testing.T, ctx context.Context, db overlay.DB, idx int) storj.NodeID {
	id := testidentity.MustPregeneratedIdentity(idx, storj.LatestIDVersion()).ID
	err := db.TestAddNodes(ctx, []*overlay.NodeDossier{
		{
			Node: pb.Node{
				Id: id,
				Address: &pb.NodeAddress{
					Address: "127.0.0.1:1234",
				},
			},
		},
	})
	require.NoError(t, err)
	return id
}

func WithoutCache() func(ball *mud.Ball) {
	return func(ball *mud.Ball) {
		mudplanet.WithModule(func(ball *mud.Ball) {
			mud.ReplaceDependency[reputation.DB, reputation.DirectDB](ball)
		})
	}
}
