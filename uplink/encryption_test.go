// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/private/testplanet"
)

func TestAllowedPathPrefixListing(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		testUplink := planet.Uplinks[0]
		testSatellite := planet.Satellites[0]
		err := testUplink.CreateBucket(ctx, testSatellite, "testbucket")
		require.NoError(t, err)

		err = testUplink.Upload(ctx, testSatellite, "testbucket", "videos/status.mp4", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		upCfg := &uplink.Config{}
		upCfg.Volatile.TLS.SkipPeerCAWhitelist = true

		up, err := uplink.NewUplink(ctx, upCfg)
		require.NoError(t, err)
		defer ctx.Check(up.Close)

		uplinkConfig := testUplink.GetConfig(testSatellite)
		scope, err := uplinkConfig.GetScope()
		require.NoError(t, err)

		encryptionAccess := scope.EncryptionAccess
		func() {
			proj, err := up.OpenProject(ctx, scope.SatelliteAddr, scope.APIKey)
			require.NoError(t, err)
			defer ctx.Check(proj.Close)

			bucket, err := proj.OpenBucket(ctx, "testbucket", encryptionAccess)
			require.NoError(t, err)
			defer ctx.Check(bucket.Close)

			list, err := bucket.ListObjects(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 1, len(list.Items))
		}()

		restrictedAPIKey, restrictedEa, err := encryptionAccess.Restrict(scope.APIKey, uplink.EncryptionRestriction{
			Bucket:     "testbucket",
			PathPrefix: "videos",
		})
		require.NoError(t, err)
		func() {
			proj, err := up.OpenProject(ctx, scope.SatelliteAddr, restrictedAPIKey)
			require.NoError(t, err)
			defer ctx.Check(proj.Close)

			bucket, err := proj.OpenBucket(ctx, "testbucket", restrictedEa)
			require.NoError(t, err)
			defer ctx.Check(bucket.Close)

			list, err := bucket.ListObjects(ctx, &storj.ListOptions{
				Prefix:    "videos",
				Direction: storj.After,
			})
			require.NoError(t, err)
			require.Equal(t, 1, len(list.Items))
		}()

	})
}
