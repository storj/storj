// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metabase"
)

func TestOrderLimitsEncryptedMetadata(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const (
			bucketName = "testbucket"
			filePath   = "test/path"
		)

		var (
			satellitePeer = planet.Satellites[0]
			uplinkPeer    = planet.Uplinks[0]
			projectID     = uplinkPeer.Projects[0].ID
		)
		// Setup: Upload an object and create order limits
		require.NoError(t, uplinkPeer.Upload(ctx, satellitePeer, bucketName, filePath, testrand.Bytes(5*memory.KiB)))

		bucket := metabase.BucketLocation{ProjectID: projectID, BucketName: bucketName}

		segments, err := satellitePeer.Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, len(segments))

		limits, _, err := satellitePeer.Orders.Service.CreateGetOrderLimits(ctx, uplinkPeer.Identity.PeerIdentity(), bucket, segments[0], 0, 0)
		require.NoError(t, err)
		require.Equal(t, 3, len(limits))

		// Test: get the bucket name and project ID from the encrypted metadata and
		// compare with the old method of getting the data from the serial numbers table.
		orderLimit1 := limits[0].Limit
		// from 3 order limits only one can be nil
		if orderLimit1 == nil {
			orderLimit1 = limits[1].Limit
		}
		require.True(t, len(orderLimit1.EncryptedMetadata) > 0)

		_, err = metabase.ParseBucketPrefix(metabase.BucketPrefix(""))
		require.Error(t, err)
		var x []byte
		_, err = metabase.ParseBucketPrefix(metabase.BucketPrefix(x))
		require.Error(t, err)
		actualOrderMetadata, err := satellitePeer.Orders.Service.DecryptOrderMetadata(ctx, orderLimit1)
		require.NoError(t, err)
		actualBucketInfo, err := metabase.ParseCompactBucketPrefix(actualOrderMetadata.GetCompactProjectBucketPrefix())
		require.NoError(t, err)
		require.Equal(t, metabase.BucketName(bucketName), actualBucketInfo.BucketName)
		require.Equal(t, projectID, actualBucketInfo.ProjectID)
	})
}
