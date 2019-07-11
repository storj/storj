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
	"storj.io/storj/pkg/storj"
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

			key, err := uplink.ParseAPIKey(apiKey)
			require.NoError(t, err)

			p, err := ul.OpenProject(ctx, satelliteAddr, key)
			require.NoError(t, err)

			bucketCount := make([]int, 6)
			for i := range bucketCount {
				_, err = p.CreateBucket(ctx, "test"+strconv.Itoa(i), nil)
				require.NoError(t, err)
			}

			list := uplink.BucketListOptions{
				Direction: storj.Forward,
				Limit:     3,
			}

			var count int
			for {
				count++
				result, err := p.ListBuckets(ctx, &list)
				require.NoError(t, err)
				require.Equal(t, 3, len(result.Items))
				for _, bucket := range result.Items {
					switch count {
					case 1:
						require.Contains(t, []string{"test0", "test1", "test2"}, bucket.Name)
					case 2:
						require.Contains(t, []string{"test3", "test4", "test5"}, bucket.Name)
					}
				}
				if !result.More {
					break
				}
				list = list.NextPage(result)
			}
			require.Equal(t, 2, count)
		})
}
