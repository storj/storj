// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package balancer_test

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/balancer"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/taskqueue"
	"storj.io/uplink/private/piecestore"
)

func TestWorkerProcessJob(t *testing.T) {
	for _, hashAlgo := range []pb.PieceHashAlgorithm{
		pb.PieceHashAlgorithm_SHA256,
		pb.PieceHashAlgorithm_BLAKE3,
	} {
		hashAlgo := hashAlgo
		t.Run(hashAlgo.String(), func(t *testing.T) {
			testWorkerProcessJob(t, hashAlgo)
		})
	}
}

func testWorkerProcessJob(t *testing.T, hashAlgo pb.PieceHashAlgorithm) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 6,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]

		// Upload test data with a specific hash algorithm.
		testData := testrand.Bytes(8 * memory.KiB)
		err := uplinkPeer.Upload(piecestore.WithPieceHashAlgo(ctx, hashAlgo), sat, "testbucket", "test/path", testData)
		require.NoError(t, err)

		// Get the uploaded segment.
		segments, err := sat.Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		segment := segments[0]

		// Pick a source node (first piece) and find a destination node not in the segment.
		sourcePiece := segment.Pieces[0]

		segmentNodeIDs := make(map[storj.NodeID]bool)
		for _, p := range segment.Pieces {
			segmentNodeIDs[p.StorageNode] = true
		}

		var destNodeID storj.NodeID
		for _, node := range planet.StorageNodes {
			if !segmentNodeIDs[node.ID()] {
				destNodeID = node.ID()
				break
			}
		}
		require.False(t, destNodeID.IsZero(), "no available destination node")

		placements := nodeselection.PlacementDefinitions{
			storj.DefaultPlacement: {
				ID:        storj.DefaultPlacement,
				Invariant: nodeselection.AllGood(),
			},
		}

		worker := balancer.NewWorker(
			zaptest.NewLogger(t),
			balancer.WorkerConfig{
				DialTimeout:     5 * time.Second,
				DownloadTimeout: 5 * time.Minute,
				UploadTimeout:   5 * time.Minute,
			},
			taskqueue.RunnerConfig{},
			nil, // no redis client needed for direct processJob call
			sat.Metabase.DB,
			sat.Orders.Service,
			sat.Overlay.Service,
			sat.Dialer,
			placements,
		)

		job := balancer.Job{
			StreamID:   segment.StreamID,
			Position:   segment.Position.Encode(),
			SourceNode: sourcePiece.StorageNode,
			DestNode:   destNodeID,
		}

		err = worker.TestingProcessJob(ctx, job)
		require.NoError(t, err)

		// Verify: the segment should now have the destination node instead of the source.
		updatedSegments, err := sat.Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, updatedSegments, 1)

		for _, p := range updatedSegments[0].Pieces {
			require.NotEqual(t, sourcePiece.StorageNode, p.StorageNode, "source node should have been replaced")
		}

		require.True(t, slices.ContainsFunc(updatedSegments[0].Pieces, func(p metabase.Piece) bool {
			return p.StorageNode == destNodeID
		}), "destination node should be in updated segment pieces")

		// Verify: we can still download the data correctly.
		downloaded, err := uplinkPeer.Download(ctx, sat, "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, testData, downloaded)
	})
}

func TestWorkerProcessJob_SegmentNotFound(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		placements := nodeselection.PlacementDefinitions{
			storj.DefaultPlacement: {
				ID:        storj.DefaultPlacement,
				Invariant: nodeselection.AllGood(),
			},
		}

		worker := balancer.NewWorker(
			zaptest.NewLogger(t),
			balancer.WorkerConfig{
				DialTimeout:     5 * time.Second,
				DownloadTimeout: 5 * time.Minute,
				UploadTimeout:   5 * time.Minute,
			},
			taskqueue.RunnerConfig{},
			nil,
			sat.Metabase.DB,
			sat.Orders.Service,
			sat.Overlay.Service,
			sat.Dialer,
			placements,
		)

		// Job with a non-existent stream ID should be silently skipped.
		job := balancer.Job{
			StreamID:   testrand.UUID(),
			Position:   0,
			SourceNode: testrand.NodeID(),
			DestNode:   testrand.NodeID(),
		}

		err := worker.TestingProcessJob(ctx, job)
		require.NoError(t, err)
	})
}

func TestWorkerProcessJob_SourceNotInSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]

		testData := testrand.Bytes(8 * memory.KiB)
		err := uplinkPeer.Upload(ctx, sat, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segments, err := sat.Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		segment := segments[0]

		placements := nodeselection.PlacementDefinitions{
			storj.DefaultPlacement: {
				ID:        storj.DefaultPlacement,
				Invariant: nodeselection.AllGood(),
			},
		}

		worker := balancer.NewWorker(
			zaptest.NewLogger(t),
			balancer.WorkerConfig{
				DialTimeout:     5 * time.Second,
				DownloadTimeout: 5 * time.Minute,
				UploadTimeout:   5 * time.Minute,
			},
			taskqueue.RunnerConfig{},
			nil,
			sat.Metabase.DB,
			sat.Orders.Service,
			sat.Overlay.Service,
			sat.Dialer,
			placements,
		)

		// Job with a source node not in the segment should be silently skipped.
		job := balancer.Job{
			StreamID:   segment.StreamID,
			Position:   segment.Position.Encode(),
			SourceNode: testrand.NodeID(),
			DestNode:   testrand.NodeID(),
		}

		err = worker.TestingProcessJob(ctx, job)
		require.NoError(t, err)

		// Segment should be unchanged.
		updatedSegments, err := sat.Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Equal(t, segment.Pieces, updatedSegments[0].Pieces)
	})
}
