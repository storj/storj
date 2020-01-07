// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/private/testplanet"
)

func TestProjectListBuckets(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 1,
		UplinkCount:      1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			cfg := uplink.Config{}
			cfg.Volatile.Log = zaptest.NewLogger(t)
			cfg.Volatile.TLS.SkipPeerCAWhitelist = true

			scope, err := planet.Uplinks[0].GetConfig(planet.Satellites[0]).GetScope()
			require.NoError(t, err)

			ul, err := uplink.NewUplink(ctx, &cfg)
			require.NoError(t, err)
			defer ctx.Check(ul.Close)

			p, err := ul.OpenProject(ctx, scope.SatelliteAddr, scope.APIKey)
			require.NoError(t, err)

			// create 6 test buckets
			for i := 0; i < 6; i++ {
				_, err = p.CreateBucket(ctx, "test"+strconv.Itoa(i), nil)
				require.NoError(t, err)
			}

			// setup list options so that we only list 3 buckets
			// at a time in alphabetical order starting at ""
			list := uplink.BucketListOptions{
				Direction: storj.Forward,
				Limit:     3,
			}

			result, err := p.ListBuckets(ctx, &list)
			require.NoError(t, err)
			require.Equal(t, 3, len(result.Items))
			require.Equal(t, "test0", result.Items[0].Name)
			require.Equal(t, "test1", result.Items[1].Name)
			require.Equal(t, "test2", result.Items[2].Name)
			require.True(t, result.More)

			list = list.NextPage(result)
			result, err = p.ListBuckets(ctx, &list)
			require.NoError(t, err)
			require.Equal(t, 3, len(result.Items))
			require.Equal(t, "test3", result.Items[0].Name)
			require.Equal(t, "test4", result.Items[1].Name)
			require.Equal(t, "test5", result.Items[2].Name)
			require.False(t, result.More)

			// List with restrictions
			scope.APIKey, scope.EncryptionAccess, err =
				scope.EncryptionAccess.Restrict(scope.APIKey,
					uplink.EncryptionRestriction{Bucket: "test0"},
					uplink.EncryptionRestriction{Bucket: "test1"})
			require.NoError(t, err)

			p, err = ul.OpenProject(ctx, scope.SatelliteAddr, scope.APIKey)
			require.NoError(t, err)
			defer ctx.Check(p.Close)

			list = uplink.BucketListOptions{
				Direction: storj.Forward,
				Limit:     3,
			}
			result, err = p.ListBuckets(ctx, &list)
			require.NoError(t, err)
			require.Equal(t, 2, len(result.Items))
			require.Equal(t, "test0", result.Items[0].Name)
			require.Equal(t, "test1", result.Items[1].Name)
			require.False(t, result.More)
		})
}
