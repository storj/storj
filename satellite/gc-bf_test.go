// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/revocation"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestGCBFUseRangedLoop(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		UplinkCount:    1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		config := planet.Satellites[0].Config

		revocationDB, err := revocation.OpenDBFromCfg(ctx, config.Server.Config)
		require.NoError(t, err)
		defer ctx.Check(revocationDB.Close)

		accessGrant, err := planet.Uplinks[0].Access[planet.Satellites[0].ID()].Serialize()
		require.NoError(t, err)

		// the run fails when an observer does, and the bloom filter observer
		// refuses to start without somewhere to upload to
		config.GarbageCollectionBF.RunOnce = true
		config.GarbageCollectionBF.AccessGrant = accessGrant
		config.GarbageCollectionBF.Bucket = "bloomfilters"
		config.GarbageCollectionBF.ShardCount = 3

		gcbf, err := satellite.NewGarbageCollectionBF(
			planet.Log().Named("test-gcbf"),
			// hopefully we can share the databases
			planet.Satellites[0].GCBF.DB,
			planet.Satellites[0].Metabase.DB,
			revocationDB,
			planet.NewVersionInfo(),
			&config,
			nil,
			time.Time{},
		)
		require.NoError(t, err)
		defer ctx.Check(gcbf.Close)

		err = gcbf.Run(ctx)
		require.NoError(t, err)
	})
}
