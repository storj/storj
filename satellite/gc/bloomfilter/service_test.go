// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter_test

import (
	"testing"

	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestGarbageCollectionBloomFilters(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.GarbageCollectionBF.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// TODO test will be replaced with something more meaningful when service
		// will be fully implemented
		planet.Satellites[0].GarbageCollection.BloomFilters.Loop.Pause()
		planet.Satellites[0].GarbageCollection.BloomFilters.Loop.TriggerWait()
	})
}
