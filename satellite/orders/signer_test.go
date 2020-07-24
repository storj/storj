// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/orders"
)

func TestSigner_EncryptedMetadata(t *testing.T) {
	encryptionKey := orders.EncryptionKey{
		ID:  orders.EncryptionKeyID{1},
		Key: storj.Key{1},
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				testplanet.ReconfigureRS(1, 1, 1, 1)(log, index, config)

				config.Orders.IncludeEncryptedMetadata = true
				config.Orders.EncryptionKeys.Default = encryptionKey
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite, uplink, storagenode := planet.Satellites[0], planet.Uplinks[0], planet.StorageNodes[0]

		project, err := uplink.GetProject(ctx, satellite)
		require.NoError(t, err)

		bucketLocation := metabase.BucketLocation{
			ProjectID:  uplink.Projects[0].ID,
			BucketName: "testbucket",
		}

		_, err = project.EnsureBucket(ctx, bucketLocation.BucketName)
		require.NoError(t, err)

		root := testrand.PieceID()
		orderCreation := time.Now()

		signer, err := orders.NewSignerGet(satellite.Orders.Service, root, orderCreation, 1e6, bucketLocation)
		require.NoError(t, err)

		addressedLimit, err := signer.Sign(ctx, storj.NodeURL{
			ID:      storagenode.ID(),
			Address: storagenode.Addr(),
		}, 1)
		require.NoError(t, err)

		require.NotEmpty(t, addressedLimit.Limit.EncryptedMetadata)
		require.NotEmpty(t, addressedLimit.Limit.EncryptedMetadataKeyId)

		require.Equal(t, encryptionKey.ID[:], addressedLimit.Limit.EncryptedMetadataKeyId)

		metadata, err := encryptionKey.DecryptMetadata(addressedLimit.Limit.SerialNumber, addressedLimit.Limit.EncryptedMetadata)
		require.NoError(t, err)

		bucketID, err := satellite.DB.Buckets().GetBucketID(ctx, bucketLocation)
		require.NoError(t, err)

		require.Equal(t, bucketID[:], metadata.BucketId)
	})
}

func TestSigner_EncryptedMetadata_UploadDownload(t *testing.T) {
	encryptionKey := orders.EncryptionKey{
		ID:  orders.EncryptionKeyID{1},
		Key: storj.Key{1},
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				testplanet.ReconfigureRS(1, 1, 1, 1)(log, index, config)

				config.Orders.IncludeEncryptedMetadata = true
				config.Orders.EncryptionKeys.Default = encryptionKey
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite, uplink := planet.Satellites[0], planet.Uplinks[0]

		testdata := testrand.Bytes(8 * memory.KiB)
		err := uplink.Upload(ctx, satellite, "testbucket", "data", testdata)
		require.NoError(t, err)

		downdata, err := uplink.Download(ctx, satellite, "testbucket", "data")
		require.NoError(t, err)

		require.Equal(t, testdata, downdata)
	})
}
