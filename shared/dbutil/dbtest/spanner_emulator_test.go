// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package dbtest_test

import (
	"testing"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/dbutil/dbtest"
)

func TestStartSpannerEmulator(t *testing.T) {
	ctx := testcontext.New(t)
	bin := ctx.Compile("./mock_spanner_emulator")
	connstr := dbtest.StartSpannerEmulator(t, "127.0.0.1", bin)

	if connstr != "spanner://127.0.0.1:46061?emulator" {
		t.Fatalf("unexpected connection string: %s", connstr)
	}
}
