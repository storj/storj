// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/dbutil/pgutil"
	"storj.io/storj/internal/dbutil/pgutil/pgtest"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

func TestOverlaycache_AllPieceCounts(t *testing.T) {
	if *pgtest.ConnStr == "" {
		t.Skip("Postgres flag missing, example: -postgres-test-db=" + pgtest.DefaultConnStr)
	}

	ctx := testcontext.New(t)

	// setup satellite db
	log := zaptest.NewLogger(t)
	schema := "overlaycache-" + pgutil.CreateRandomTestingSchemaName(8)
	connstr := pgutil.ConnstrWithSchema(*pgtest.ConnStr, schema)
	db, err := New(log, connstr)
	require.NoError(t, err)
	require.NotNil(t, db)
	defer cleanup(t, db, schema)

	err = db.CreateTables()
	require.NoError(t, err)

	// get overlay db
	overlay, ok := db.OverlayCache().(*overlaycache)
	require.True(t, ok)
	require.NotNil(t, overlay)

	// create test nodes in overlay db
	testNodes := createTestNodes(10, t, ctx, overlay.db)

	// update piece count fields on test nodes; set exponentially
	expectedPieceCounts := make(map[storj.NodeID]int)
	sqlQuery := `UPDATE nodes SET piece_count = newvals.piece_count FROM ( VALUES `
	args := make([]interface{}, 0, len(testNodes)*2)
	for i, node := range testNodes {
		nodeID, err := storj.NodeIDFromBytes(node.Id)
		pieceCount := int(math.Pow10(i))
		require.NoError(t, err)
		require.NotEqual(t, storj.NodeID{}, nodeID)

		expectedPieceCounts[nodeID] = pieceCount

		sqlQuery += `(?::BYTEA, ?::BIGINT), `
		args = append(args, nodeID, pieceCount)
	}
	sqlQuery = sqlQuery[:len(sqlQuery)-2] // trim off the last comma+space
	sqlQuery += `) newvals(nodeid, piece_count) WHERE nodes.id = newvals.nodeid`

	_, err = overlay.db.ExecContext(ctx, overlay.db.Rebind(sqlQuery), args...)
	require.NoError(t, err)

	// expected and actual piece count maps should match
	actualPieceCounts, err := overlay.AllPieceCounts(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedPieceCounts, actualPieceCounts)
}

func TestOverlaycache_UpdatePieceCounts(t *testing.T) {
	if *pgtest.ConnStr == "" {
		t.Skip("Postgres flag missing, example: -postgres-test-db=" + pgtest.DefaultConnStr)
	}

	ctx := testcontext.New(t)

	// setup satellite db
	log := zaptest.NewLogger(t)
	schema := "overlaycache-" + pgutil.CreateRandomTestingSchemaName(8)
	connstr := pgutil.ConnstrWithSchema(*pgtest.ConnStr, schema)
	db, err := New(log, connstr)
	require.NoError(t, err)
	require.NotNil(t, db)
	defer cleanup(t, db, schema)

	err = db.CreateTables()
	require.NoError(t, err)

	// get overlay db
	overlay, ok := db.OverlayCache().(*overlaycache)
	require.True(t, ok)
	require.NotNil(t, overlay)

	// create test nodes in overlay db
	testNodes := createTestNodes(10, t, ctx, overlay.db)

	// set expected piece counts
	expectedPieceCounts := make(map[storj.NodeID]int)
	for i, node := range testNodes {
		pieceCount := int(math.Pow10(i))

		var nodeID storj.NodeID
		copy(nodeID[:], node.Id)
		expectedPieceCounts[nodeID] = pieceCount
	}

	// update piece count fields on test nodes; set exponentially
	err = overlay.UpdatePieceCounts(ctx, expectedPieceCounts)
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
