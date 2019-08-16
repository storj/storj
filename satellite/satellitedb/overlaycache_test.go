// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestOverlaycache_AllPieceCounts(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 10,
		UplinkCount:      0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		overlay := planet.Satellites[0].Overlay.DB

		// TODO: update piece counts

		{ // Get piece counts
			pieceCounts, err := overlay.AllPieceCounts(ctx)
			require.NoError(t, err)
			//require.NotNil(t, pieceCounts)
			_ = pieceCounts
		}
	})
}
