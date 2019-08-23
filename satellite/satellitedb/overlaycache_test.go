// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"github.com/lib/pq"
	"math"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/dbutil/pgutil"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

//type overlaycache = satellitedb.TestOverlaycache

func TestOverlaycache_AllPieceCounts(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		// get overlay db
		overlay, ok := db.OverlayCache().(*satellitedb.Overlaycache)
		require.True(t, ok)
		require.NotNil(t, overlay)

		// create test nodes in overlay db
		testNodes := createTestNodes(10, t, ctx, overlay.DB())

		// set expected piece counts
		var err error
		expectedPieceCounts := make(map[storj.NodeID]int)
		for i, node := range testNodes {
			pieceCount := math.Pow10(i + 1)
			nodeID, err := storj.NodeIDFromBytes(node.Id)
			require.NoError(t, err)

			expectedPieceCounts[nodeID] = int(pieceCount)
		}

		switch overlay.db.Driver().(type) {
		case *pq.Driver:
			err = overlay.postgresUpdatePieceCounts(ctx, expectedPieceCounts)
		default:
			err = overlay.sqliteUpdatePieceCounts(ctx, expectedPieceCounts)
		}
		require.NoError(t, err)

		// expected and actual piece count maps should match
		actualPieceCounts, err := overlay.AllPieceCounts(ctx)
		require.NoError(t, err)
		require.Equal(t, expectedPieceCounts, actualPieceCounts)
	})
}

func TestOverlaycache_UpdatePieceCounts(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		// get overlay db
		overlay, ok := db.OverlayCache().(*satellitedb.overlaycache)
		require.True(t, ok)
		require.NotNil(t, overlay)

		// create test nodes in overlay db
		testNodes := createTestNodes(10, t, ctx, overlay.db)

		// set expected piece counts
		expectedPieceCounts := testPieceCounts(t, testNodes)

		// update piece count fields on test nodes; set exponentially
		err := overlay.UpdatePieceCounts(ctx, expectedPieceCounts)
		require.NoError(t, err)

		// build actual piece counts map
		actualPieceCounts := make(map[storj.NodeID]int)
		rows, err := overlay.db.All_Node_Id_Node_PieceCount_By_PieceCount_Not_Number(ctx)
		for _, row := range rows {
			var nodeID storj.NodeID
			copy(nodeID[:], row.Id)

			actualPieceCounts[nodeID] = int(row.PieceCount)
		}
		require.NoError(t, err)

		// expected and actual piece count maps should match
		require.Equal(t, expectedPieceCounts, actualPieceCounts)
	})
}


// testPieceCounts builds a piece count map from the node ids of `testNodes`,
// incrementing the piece count exponentially from one node to the next.
func testPieceCounts(t *testing.T, testNodes []*dbx.Node) map[storj.NodeID]int {
	pieceCounts := make(map[storj.NodeID]int)
	for i, node := range testNodes {
		pieceCount := math.Pow10(i + 1)
		nodeID, err := storj.NodeIDFromBytes(node.Id)
		if err != nil {
			t.Fatal(err)
		}

		pieceCounts[nodeID] = int(pieceCount)
	}
	return pieceCounts
}

func createTestNodes(count int, t *testing.T, ctx *testcontext.Context, db *dbx.DB) (nodes []*dbx.Node) {
	for i := 0; i < count; i++ {
		nodeID := storj.NodeID{byte(i + 1)}

		node, err := db.Create_Node(
			ctx,
			dbx.Node_Id(nodeID.Bytes()),
			dbx.Node_Address("0.0.0.0:0"),
			dbx.Node_LastNet("0.0.0.0"),
			dbx.Node_Protocol(0),
			dbx.Node_Type(int(pb.NodeType_INVALID)),
			dbx.Node_Email(""),
			dbx.Node_Wallet(""),
			dbx.Node_FreeBandwidth(-1),
			dbx.Node_FreeDisk(-1),
			dbx.Node_Major(0),
			dbx.Node_Minor(0),
			dbx.Node_Patch(0),
			dbx.Node_Hash(""),
			dbx.Node_Timestamp(time.Time{}),
			dbx.Node_Release(false),
			dbx.Node_Latency90(0),
			dbx.Node_AuditSuccessCount(0),
			dbx.Node_TotalAuditCount(0),
			dbx.Node_UptimeSuccessCount(0),
			dbx.Node_TotalUptimeCount(0),
			dbx.Node_LastContactSuccess(time.Now()),
			dbx.Node_LastContactFailure(time.Time{}),
			dbx.Node_Contained(false),
			dbx.Node_AuditReputationAlpha(0),
			dbx.Node_AuditReputationBeta(0),
			dbx.Node_UptimeReputationAlpha(0),
			dbx.Node_UptimeReputationBeta(0),
			dbx.Node_Create_Fields{
				Disqualified: dbx.Node_Disqualified_Null(),
			},
		)
		require.NoError(t, err)

		nodes = append(nodes, node)
	}

	rows, err := db.All_Node_Id(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.Len(t, rows, count)

	return nodes
}

func cleanup(t *testing.T, db satellite.DB, schema string) {
	err := db.DropSchema(schema)
	require.NoError(t, err)

	err = db.Close()
	require.NoError(t, err)

}
