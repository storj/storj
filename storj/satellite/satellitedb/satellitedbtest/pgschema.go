// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest

import (
	"github.com/zeebo/errs"

	"storj.io/storj/private/dbutil/pgutil/pgtest"
)

// PostgresDefined returns an error when the --postgres-test-db or STORJ_POSTGRES_TEST is not set for tests.
func PostgresDefined() error {
	if *pgtest.ConnStr == "" {
		return errs.New("flag --postgres-test-db or environment variable STORJ_POSTGRES_TEST not defined for PostgreSQL test database")
	}
	return nil
}
