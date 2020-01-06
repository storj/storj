// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"

	"storj.io/storj/private/dbutil/pgutil/pgtest"
)

func TestMigrateCockroach(t *testing.T) {
	if *pgtest.CrdbConnStr == "" {
		t.Skip("Cockroach flag missing, example: -cockroach-test-db=" + pgtest.DefaultCrdbConnStr)
	}
	t.Parallel()
	pgMigrateTest(t, *pgtest.CrdbConnStr)
}
