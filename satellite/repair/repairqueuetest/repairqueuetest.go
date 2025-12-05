// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package repairqueuetest

import (
	"testing"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/jobq/jobqtest"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// Run runs the given test function first with the SQL-based repair queue and
// then with the jobq repair queue.
func Run(t *testing.T, f func(ctx *testcontext.Context, t *testing.T, rq queue.RepairQueue)) {
	t.Run("sql-repair-queue", func(t *testing.T) {
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			f(ctx, t, db.RepairQueue())
		})
	})
	t.Run("jobq-repair-queue", func(t *testing.T) {
		jobqtest.Run(t, f)
	})
}
