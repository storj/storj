// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/storj"
)

func TestBucketAttrs(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		var (
			encryptionKey              = "voxmachina"
			bucketName                 = "mightynein"
			segmentsSize   memory.Size = 688894
			shareSize                  = 1 * memory.KiB
			requiredShares int16       = 2
			repairShares   int16       = 3
			optimalShares  int16       = 4
			totalShares    int16       = 5

			access EncryptionAccess
		)
		copy(access.Key[:], encryptionKey)

		// we only use testUplink for the free API key, until such time
		// as testplanet knows how to provide libuplink handles :D
		testUplink := planet.Uplinks[0]
		apiKey, err := ParseAPIKey(testUplink.APIKey[satellite.ID()])
		require.NoError(t, err)

		var cfg Config
		cfg.Volatile.TLS.SkipPeerCAWhitelist = true
		u, err := NewUplink(ctx, &cfg)
		require.NoError(t, err)

		proj, err := u.OpenProject(ctx, satellite.Addr(), apiKey)
		require.NoError(t, err)

		inBucketConfig := BucketConfig{
			PathCipher: storj.EncSecretBox,
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.EncAESGCM,
				BlockSize:   512,
			},
			Volatile: struct {
				RedundancyScheme storj.RedundancyScheme
				SegmentsSize     memory.Size
			}{
				RedundancyScheme: storj.RedundancyScheme{
					Algorithm:      storj.ReedSolomon,
					ShareSize:      shareSize.Int32(),
					RequiredShares: requiredShares,
					RepairShares:   repairShares,
					OptimalShares:  optimalShares,
					TotalShares:    totalShares,
				},
				SegmentsSize: segmentsSize,
			},
		}
		before := time.Now()
		bucket, err := proj.CreateBucket(ctx, bucketName, &inBucketConfig)
		require.NoError(t, err)

		assert.Equal(t, bucketName, bucket.Name)
		assert.Falsef(t, bucket.Created.Before(before), "impossible creation time %v", bucket.Created)

		got, err := proj.OpenBucket(ctx, bucketName, &access, 0)
		require.NoError(t, err)

		assert.Equal(t, bucketName, got.Name)
		assert.Equal(t, inBucketConfig.PathCipher, got.PathCipher)
		assert.Equal(t, inBucketConfig.EncryptionParameters, got.EncryptionParameters)
		assert.Equal(t, inBucketConfig.Volatile.RedundancyScheme, got.Volatile.RedundancyScheme)
		assert.Equal(t, inBucketConfig.Volatile.SegmentsSize, got.Volatile.SegmentsSize)

		err = proj.DeleteBucket(ctx, bucketName)
		require.NoError(t, err)
	})
}
