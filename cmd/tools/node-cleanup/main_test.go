// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	nodecleanup "storj.io/storj/cmd/tools/node-cleanup"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/tagsql"
)

func TestDelete(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		raw := db.Testing().RawDB()

		// insert 5 nodes (4 to delete)
		for i := 0; i < 5; i++ {
			insertNode(ctx, t, raw, i, i != 3)
		}

		requireTableCount := func(tableAndFilter string, expected int) {
			var count int
			err := raw.QueryRow(ctx, `SELECT count(*) FROM `+tableAndFilter).Scan(&count)
			require.NoError(t, err)
			require.Equal(t, expected, count)
		}

		requireTableCount("nodes", 5)
		requireTableCount("nodes WHERE last_contact_success = '0001-01-01 00:00:00+00' AND created_at <= '2022-09-01 00:00:00+00'", 4)
		requireTableCount("storagenode_paystubs", 5)
		requireTableCount("peer_identities", 5)
		requireTableCount("node_api_versions", 5)

		err := nodecleanup.DeleteFromTables(ctx, zaptest.NewLogger(t), raw, nodecleanup.Config{
			Limit:         2,
			CreatedAt:     "2022-09-01 00:00:00+00",
			MaxIterations: -1,
		})
		require.NoError(t, err)

		requireTableCount("nodes", 1)
		requireTableCount("nodes WHERE last_contact_success = '0001-01-01 00:00:00+00' AND created_at <= '2022-09-01 00:00:00+00'", 0)
		requireTableCount("storagenode_paystubs", 1)
		requireTableCount("peer_identities", 1)
		requireTableCount("node_api_versions", 1)
	})
}

var runLargeDelete = flag.Bool("run-large-node-cleanup", false, "run a benchmark for deleting nodes")

func TestLargeDelete(t *testing.T) {
	if !*runLargeDelete {
		t.Skip("use flag -run-large-node-cleanup to enable this test")
	}

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		raw := db.(interface{ DebugGetDBHandle() tagsql.DB }).DebugGetDBHandle()

		t.Log("inserting nodes")
		problematic, valid := 0, 0
		for i := 0; i < 30000; i++ {
			problem := i%10000 != 0 // every 10000 is valid
			insertNode(ctx, t, raw, i, problem)
			if problem {
				problematic++
			} else {
				valid++
				t.Log(i)
			}
		}

		requireTableCount := func(tableAndFilter string, expected int) {
			var count int
			err := raw.QueryRow(ctx, `SELECT count(*) FROM `+tableAndFilter).Scan(&count)
			require.NoError(t, err)
			require.Equal(t, expected, count)
		}

		requireTableCount("nodes", valid+problematic)
		requireTableCount("nodes WHERE last_contact_success = '0001-01-01 00:00:00+00' AND created_at <= '2022-09-01 00:00:00+00'", problematic)
		requireTableCount("storagenode_paystubs", valid+problematic)
		requireTableCount("peer_identities", valid+problematic)
		requireTableCount("node_api_versions", valid+problematic)

		err := nodecleanup.DeleteFromTables(ctx, zaptest.NewLogger(t), raw, nodecleanup.Config{
			Limit:         1000,
			CreatedAt:     "2022-09-01 00:00:00+00",
			MaxIterations: -1,
		})
		require.NoError(t, err)

		requireTableCount("nodes", valid)
		requireTableCount("nodes WHERE last_contact_success = '0001-01-01 00:00:00+00' AND created_at <= '2022-09-01 00:00:00+00'", 0)
		requireTableCount("storagenode_paystubs", valid)
		requireTableCount("peer_identities", valid)
		requireTableCount("node_api_versions", valid)
	})
}

func insertNode(ctx context.Context, t *testing.T, raw tagsql.DB, index int, problematic bool) {
	index++ // disallow 0 nodeid

	var nodeid storj.NodeID
	binary.BigEndian.PutUint64(nodeid[:], uint64(index))

	address := fmt.Sprintf("127.0.0.1:100%02d", index)

	var created_at string
	var updated_at string
	var last_contact_success string
	if problematic {
		created_at = "2022-08-31 23:59:31.028103+00"
		updated_at = "2022-10-01 01:08:31.028103+00"
		last_contact_success = "0001-01-01 00:00:00+00"
	} else {
		created_at = "2022-09-01 23:59:31.028103+00"
		updated_at = "2022-10-01 01:08:31.028103+00"
		last_contact_success = "2022-10-01 01:08:31.028103+00"
	}

	_, err := raw.ExecContext(ctx, `
		INSERT INTO "nodes" ("id", "address", "email", "last_net", "wallet", "created_at", "updated_at", "last_contact_success", "exit_success") VALUES
			($1, $2, '', '', '', $3, $4, $5, false)
	`, nodeid, address, created_at, updated_at, last_contact_success)
	require.NoError(t, err)

	_, err = raw.ExecContext(ctx, `
		INSERT INTO "storagenode_paystubs"("period", "node_id", "created_at", "codes", "usage_at_rest", "usage_get", "usage_put", "usage_get_repair", "usage_put_repair", "usage_get_audit", "comp_at_rest", "comp_get", "comp_put", "comp_get_repair", "comp_put_repair", "comp_get_audit", "surge_percent", "held", "owed", "disposed", "paid", "distributed")  VALUES
			('2022-08', $1, '2022-09-07T20:14:21.479141Z', '', 1327959864508416, 294054066688, 159031363328, 226751, 0, 836608, 2861984, 5881081, 0, 226751, 0, 8, 300, 0, 26909472, 0, 26909472, 0)
	`, nodeid)
	require.NoError(t, err)

	_, err = raw.ExecContext(ctx, `
		INSERT INTO "peer_identities"("node_id", "leaf_serial_number","chain", "updated_at") VALUES
			($1, E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014'::bytea, E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, '2022-09-07 08:07:31.335028+00')
	`, nodeid)
	require.NoError(t, err)

	_, err = raw.ExecContext(ctx, `
		INSERT INTO "node_api_versions"("id", "api_version", "created_at", "updated_at") VALUES
			($1, 1, $2, $3);
	`, nodeid, created_at, updated_at)
	require.NoError(t, err)
}
