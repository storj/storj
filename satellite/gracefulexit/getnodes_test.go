// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestSatelliteDBSetup(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		testGetExitingNodes(ctx, t, db.OverlayCache())
	})
}

func testGetExitingNodes(ctx context.Context, t *testing.T, cache overlay.DB) {

	_, err := cache.GetExitingNodes(ctx)
	require.NoError(t, err)

}
