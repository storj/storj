// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

func TestIterate(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		saPeer := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]

		// Setup: create 2 test buckets
		err := uplinkPeer.CreateBucket(ctx, saPeer, "test1")
		require.NoError(t, err)
		err = uplinkPeer.CreateBucket(ctx, saPeer, "test2")
		require.NoError(t, err)

		// Setup: upload an object in one of the buckets
		expectedData := testrand.Bytes(50 * memory.KiB)
		err = uplinkPeer.Upload(ctx, saPeer, "test2", "test/path", expectedData)
		require.NoError(t, err)

		// Test: Confirm that only the objects are in pointerDB
		// and not the bucket metadata
		var itemCount int
		metainfoSvc := saPeer.Metainfo.Service
		err = metainfoSvc.Iterate(ctx, "", "", true, false, func(ctx context.Context, it storage.Iterator) error {
			var item storage.ListItem
			for it.Next(ctx, &item) {
				itemCount++
				pathElements := storj.SplitPath(storj.Path(item.Key))
				// there should not be any objects in pointerDB with less than 4 path
				// elements. i.e buckets should not be stored in pointerDB
				require.True(t, len(pathElements) > 3)
			}
			return nil
		})
		require.NoError(t, err)
		// There should only be 1 item in pointerDB, the one object
		require.Equal(t, 1, itemCount)
	})
}
