// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uitest_test

import (
	"testing"

	"storj.io/common/testcontext"
	uitest "storj.io/storj/testsuite/playwright-ui/server"
)

func TestEdge(t *testing.T) {
	uitest.Edge(t, func(t *testing.T, ctx *testcontext.Context, planet *uitest.EdgePlanet) {
		t.Log("working")
	})
}
