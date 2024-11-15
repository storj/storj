// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
)

func TestRepairSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 20, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 4, 6, 8),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		expectedData := testrand.Bytes(20 * memory.KiB)
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "object", expectedData)
		require.NoError(t, err)

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)

		repairSegment(ctx, zaptest.NewLogger(t), satellite.Repairer, satellite.Metabase.DB, segments[0])

		data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "bucket", "object")
		require.NoError(t, err)
		require.Equal(t, expectedData, data)

		segmentsAfter, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segmentsAfter, 1)

		// verify that there are no nodes from before repair as we replacing all of them
		require.NotEqual(t, segments[0].Pieces, segmentsAfter[0].Pieces)
		oldNodes := map[storj.NodeID]struct{}{}
		for _, piece := range segments[0].Pieces {
			oldNodes[piece.StorageNode] = struct{}{}
		}

		for _, piece := range segmentsAfter[0].Pieces {
			_, found := oldNodes[piece.StorageNode]
			require.False(t, found)
		}

		// delete all pieces
		for _, node := range planet.StorageNodes {
			node.Storage2.PieceBackend.TestingDeleteAllPiecesForSatellite(planet.Satellites[0].ID())
		}

		// we cannot download segment so repair is not possible
		observedZapCore, observedLogs := observer.New(zap.ErrorLevel)
		observedLogger := zap.New(observedZapCore)
		repairSegment(ctx, observedLogger, satellite.Repairer, satellite.Metabase.DB, segments[0])
		require.Contains(t, "download failed", observedLogs.All()[observedLogs.Len()-1].Message)

		// TODO add more detailed tests
	})
}
