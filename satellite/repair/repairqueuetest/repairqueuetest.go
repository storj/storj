// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package repairqueuetest

import (
	"testing"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/jobq/jobqtest"
	"storj.io/storj/satellite/repair/queue"
)

// Run runs the given test function with the jobq repair queue.
func Run(t *testing.T, f func(ctx *testcontext.Context, t *testing.T, rq queue.RepairQueue)) {
	jobqtest.Run(t, f)
}
