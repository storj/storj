// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"

	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestMigrate(t *testing.T) {
	if *satellitedbtest.TestPostgres == "" {
		t.Skip("Postgres flag missing, example: -postgres-test-db=" + satellitedbtest.DefaultPostgresConn)
	}
}
