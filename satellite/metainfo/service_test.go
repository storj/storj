// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
)

func TestCountBuckets(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		saPeer := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]
		projectID := planet.Uplinks[0].Projects[0].ID
		count, err := saPeer.Metainfo.Service.CountBuckets(ctx, projectID)
		require.NoError(t, err)
		require.Equal(t, 0, count)
		// Setup: create 2 test buckets
		err = uplinkPeer.CreateBucket(ctx, saPeer, "test1")
		require.NoError(t, err)
		count, err = saPeer.Metainfo.Service.CountBuckets(ctx, projectID)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		err = uplinkPeer.CreateBucket(ctx, saPeer, "test2")
		require.NoError(t, err)
		count, err = saPeer.Metainfo.Service.CountBuckets(ctx, projectID)
		require.NoError(t, err)
		require.Equal(t, 2, count)
	})
}

func TestIsBucketEmpty(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]

		err := uplinkPeer.CreateBucket(ctx, satellite, "bucket")
		require.NoError(t, err)

		empty, err := satellite.Metainfo.Service.IsBucketEmpty(ctx, uplinkPeer.Projects[0].ID, []byte("bucket"))
		require.NoError(t, err)
		require.True(t, empty)

		err = uplinkPeer.Upload(ctx, satellite, "bucket", "test/path", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		empty, err = satellite.Metainfo.Service.IsBucketEmpty(ctx, uplinkPeer.Projects[0].ID, []byte("bucket"))
		require.NoError(t, err)
		require.False(t, empty)
	})
}
