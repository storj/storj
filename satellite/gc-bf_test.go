// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/revocation"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestGCBFUseRangedLoop(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		config := planet.Satellites[0].Config

		revocationDB, err := revocation.OpenDBFromCfg(ctx, config.Server.Config)
		require.NoError(t, err)
		defer ctx.Check(revocationDB.Close)

		config.GarbageCollectionBF.RunOnce = true

		gcbf, err := satellite.NewGarbageCollectionBF(
			planet.Log().Named("test-gcbf"),
			// hopefully we can share the databases
			planet.Satellites[0].GCBF.DB,
			planet.Satellites[0].Metabase.DB,
			revocationDB,
			planet.NewVersionInfo(),
			&config,
			nil,
		)
		require.NoError(t, err)
		defer ctx.Check(gcbf.Close)

		err = gcbf.Run(ctx)
		require.NoError(t, err)
	})
}
