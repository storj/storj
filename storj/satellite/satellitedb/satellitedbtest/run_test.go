// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest_test

import (
	"testing"

	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestDatabase(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
	})
}
