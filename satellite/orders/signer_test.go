// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
)

func TestSigner_EncryptedMetadata(t *testing.T) {
	ekeys, err := orders.NewEncryptionKeys(orders.EncryptionKey{
		ID:  orders.EncryptionKeyID{1},
		Key: storj.Key{1},
	})
	require.NoError(t, err)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				testplanet.ReconfigureRS(1, 1, 1, 1)(log, index, config)

				config.Orders.EncryptionKeys = *ekeys
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite, uplink, storagenode := planet.Satellites[0], planet.Uplinks[0], planet.StorageNodes[0]

		project, err := uplink.GetProject(ctx, satellite)
		require.NoError(t, err)
		bucketName := metabase.BucketName("123456789012345678901234567890123456789012345678901234567890123")
		bucketLocation := metabase.BucketLocation{
			ProjectID:  uplink.Projects[0].ID,
			BucketName: bucketName,
		}

		_, err = project.EnsureBucket(ctx, bucketLocation.BucketName.String())
		require.NoError(t, err)

		root := testrand.PieceID()
		orderCreation := time.Now()

		signer, err := orders.NewSignerGet(satellite.Orders.Service, root, orderCreation, 1e6, bucketLocation)
		require.NoError(t, err)

		addressedLimit, err := signer.Sign(ctx, &pb.Node{
			Id: storagenode.ID(),
			Address: &pb.NodeAddress{
				Address: storagenode.Addr(),
				NoiseInfo: &pb.NoiseInfo{
					PublicKey: []byte("testpublickey"),
				},
			},
		}, 1)
		require.NoError(t, err)

		require.NotEmpty(t, addressedLimit.Limit.EncryptedMetadata)
		require.NotEmpty(t, addressedLimit.Limit.EncryptedMetadataKeyId)

		require.Equal(t, ekeys.Default.ID[:], addressedLimit.Limit.EncryptedMetadataKeyId)
		require.Equal(t, addressedLimit.StorageNodeAddress.NoiseInfo.PublicKey, []byte("testpublickey"))

		metadata, err := ekeys.Default.DecryptMetadata(addressedLimit.Limit.SerialNumber, addressedLimit.Limit.EncryptedMetadata)
		require.NoError(t, err)

		bucketInfo, err := metabase.ParseCompactBucketPrefix(metadata.CompactProjectBucketPrefix)
		require.NoError(t, err)
		require.Equal(t, bucketInfo.BucketName, bucketName)
		require.Equal(t, bucketInfo.ProjectID, uplink.Projects[0].ID)
	})
}

func TestSigner_EncryptedMetadata_UploadDownload(t *testing.T) {
	ekeys, err := orders.NewEncryptionKeys(orders.EncryptionKey{
		ID:  orders.EncryptionKeyID{1},
		Key: storj.Key{1},
	})
	require.NoError(t, err)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				testplanet.ReconfigureRS(1, 1, 1, 1)(log, index, config)

				config.Orders.EncryptionKeys = *ekeys
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite, uplink := planet.Satellites[0], planet.Uplinks[0]

		const bucket = "123456789012345678901234567890123456789012345678901234567890123"

		testdata := testrand.Bytes(8 * memory.KiB)
		err := uplink.Upload(ctx, satellite, bucket, "data", testdata)
		require.NoError(t, err)

		downdata, err := uplink.Download(ctx, satellite, bucket, "data")
		require.NoError(t, err)

		require.Equal(t, testdata, downdata)
	})
}
