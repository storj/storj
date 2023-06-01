// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestGCBFUseRangedLoop(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.GarbageCollectionBF.RunOnce = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].GCBF.Run(ctx)
		require.NoError(t, err)
	})
}
