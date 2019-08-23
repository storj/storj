// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"github.com/lib/pq"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestOverlaycache_AllPieceCounts(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		// get overlay db
		overlay := db.OverlayCache()

		// get dbx db access
		_db, ok := db.(interface{ TestDBAccess() *dbx.DB })
		require.True(t, ok)

		dbAccess := _db.TestDBAccess()
		require.NotNil(t, dbAccess)

		// create test nodes in overlay db
		testNodes := createTestNodes(ctx, 10, t, dbAccess)

		// set expected piece counts
		var err error
		expectedPieceCounts := make(map[storj.NodeID]int)
		for i, node := range testNodes {
			pieceCount := math.Pow10(i + 1)
			nodeID, err := storj.NodeIDFromBytes(node.Id)
			require.NoError(t, err)

			expectedPieceCounts[nodeID] = int(pieceCount)
		}

		updateTestPieceCounts(t, dbAccess, expectedPieceCounts)
		require.NoError(t, err)

		// expected and actual piece count maps should match
		actualPieceCounts, err := overlay.AllPieceCounts(ctx)
		require.NoError(t, err)
		require.Equal(t, expectedPieceCounts, actualPieceCounts)
	})
}

func TestOverlaycache_UpdatePieceCounts(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		// get dbx db access
		_db, ok := db.(interface{ TestDBAccess() *dbx.DB })
		require.True(t, ok)

		dbAccess := _db.TestDBAccess()
		require.NotNil(t, dbAccess)

		// create test nodes in overlay db
		testNodes := createTestNodes(ctx, 10, t, dbAccess)

		// set expected piece counts
		expectedPieceCounts := testPieceCounts(t, testNodes)

		// update piece count fields on test nodes; set exponentially
		updateTestPieceCounts(t, dbAccess, expectedPieceCounts)

		// build actual piece counts map
		actualPieceCounts := make(map[storj.NodeID]int)
		rows, err := dbAccess.All_Node_Id_Node_PieceCount_By_PieceCount_Not_Number(ctx)
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

func updateTestPieceCounts(t *testing.T, dbAccess *dbx.DB, pieceCounts map[storj.NodeID]int) {
	for nodeID, pieceCount := range pieceCounts {
		var args []interface{}
		var sqlQuery string
		args = append(args, pieceCount, nodeID)
		switch dbAccess.Driver().(type) {
		case *pq.Driver:
			sqlQuery = "UPDATE nodes SET piece_count = ( ?::BIGINT ) WHERE id = ?::BYTEA;"
		default:
			sqlQuery = "UPDATE nodes SET piece_count = ( ? ) WHERE id = ?;"
		}
		_, err := dbAccess.Exec(dbAccess.Rebind(sqlQuery), args...)
		if err != nil {
			t.Fatal(err)
		}
	}
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

func createTestNodes(ctx *testcontext.Context, count int, t *testing.T, db *dbx.DB) (nodes []*dbx.Node) {
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
