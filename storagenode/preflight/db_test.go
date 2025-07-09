// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package preflight_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestPreflightSchema(t *testing.T) {
	// no change should not cause a preflight error
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		err := db.Preflight(ctx)
		require.NoError(t, err)
	})

	// adding something to the schema should cause a preflight error
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		err := db.Preflight(ctx)
		require.NoError(t, err)

		// add index to used serials db
		rawDBs := db.(*storagenodedb.DB).RawDatabases()
		satellitesDB := rawDBs[storagenodedb.SatellitesDBName]
		_, err = satellitesDB.GetDB().ExecContext(ctx, "CREATE INDEX a_new_index ON satellites(status)")
		require.NoError(t, err)

		// expect error from preflight check for addition
		err = db.Preflight(ctx)
		require.Error(t, err)
		require.True(t, storagenodedb.ErrPreflight.Has(err))
	})

	// removing something from the schema should cause a preflight error
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		err := db.Preflight(ctx)
		require.NoError(t, err)

		// remove index from orders db
		rawDBs := db.(*storagenodedb.DB).RawDatabases()
		ordersDB := rawDBs[storagenodedb.OrdersDBName]
		_, err = ordersDB.GetDB().ExecContext(ctx, "DROP INDEX idx_order_archived_at;")
		require.NoError(t, err)

		// expect error from preflight check for removal
		err = db.Preflight(ctx)
		require.Error(t, err)
		require.True(t, storagenodedb.ErrPreflight.Has(err))
	})

	// having a test table should not cause the preflight check to fail
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		err := db.Preflight(ctx)
		require.NoError(t, err)

		// add test_table to used serials db
		rawDBs := db.(*storagenodedb.DB).RawDatabases()
		bandwidthDB := rawDBs[storagenodedb.BandwidthDBName]
		_, err = bandwidthDB.GetDB().ExecContext(ctx, "CREATE TABLE test_table(id int NOT NULL, name varchar(30), PRIMARY KEY (id));")
		require.NoError(t, err)

		// expect no error from preflight check with added test_table
		err = db.Preflight(ctx)
		require.NoError(t, err)
	})
}
