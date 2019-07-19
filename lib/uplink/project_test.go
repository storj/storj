// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/lib/uplink"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink/setup"
)

func TestProjectListBuckets(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 1,
		UplinkCount:      1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			cfg := uplink.Config{}
			cfg.Volatile.TLS.SkipPeerCAWhitelist = true

			satelliteAddr := planet.Satellites[0].Local().Address.Address
			apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

			ul, err := uplink.NewUplink(ctx, &cfg)
			require.NoError(t, err)
			defer ctx.Check(ul.Close)

			key, err := uplink.ParseAPIKey(apiKey)
			require.NoError(t, err)

			p, err := ul.OpenProject(ctx, satelliteAddr, key)
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
			restriction := libuplink.EncryptionRestriction{
				Bucket: "test0",
			}
			access, err := setup.LoadEncryptionAccess(ctx,
				planet.Uplinks[0].GetConfig(planet.Satellites[0]).Enc,
			)
			require.NoError(t, err)
			key, access, err = access.Restrict(key, restriction)
			require.NoError(t, err)

			caveat := macaroon.Caveat{}
			caveat.DisallowReads = true
			caveat.AllowedPaths = append(caveat.AllowedPaths,
				&macaroon.Caveat_Path{
					Bucket: []byte("test1"),
				},
			)

			key, err = key.Restrict(caveat)
			require.NoError(t, err)

			p, err = ul.OpenProject(ctx, satelliteAddr, key)
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
