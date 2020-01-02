// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"bytes"
	"io/ioutil"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/private/testplanet"
)

type testConfig struct {
	uplinkCfg uplink.Config
}

func testPlanetWithLibUplink(t *testing.T, cfg testConfig,
	testFunc func(*testing.T, *testcontext.Context, *testplanet.Planet, *uplink.Project)) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// we only use testUplink for the free API key, until such time
		// as testplanet makes it easy to get another way :D
		testUplink := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		apiKey, err := uplink.ParseAPIKey(testUplink.APIKey[satellite.ID()].Serialize())
		if err != nil {
			t.Fatalf("could not parse API key from testplanet: %v", err)
		}
		up, err := uplink.NewUplink(ctx, &cfg.uplinkCfg)
		if err != nil {
			t.Fatalf("could not create new Uplink object: %v", err)
		}
		defer ctx.Check(up.Close)
		proj, err := up.OpenProject(ctx, satellite.Addr(), apiKey)
		if err != nil {
			t.Fatalf("could not open project from uplink under testplanet: %v", err)
		}
		defer ctx.Check(proj.Close)

		testFunc(t, ctx, planet, proj)
	})
}

// check that partner bucket attributes are stored and retrieved correctly.
func TestBucket_PartnerAttribution(t *testing.T) {
	var (
		access     = uplink.NewEncryptionAccessWithDefaultKey(storj.Key{0, 1, 2, 3, 4})
		bucketName = "mightynein"
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apikey, err := uplink.ParseAPIKey(planet.Uplinks[0].APIKey[satellite.ID()].Serialize())
		require.NoError(t, err)

		partnerID := testrand.UUID()

		t.Run("without partner id", func(t *testing.T) {
			config := uplink.Config{}
			config.Volatile.Log = zaptest.NewLogger(t)
			config.Volatile.TLS.SkipPeerCAWhitelist = true

			up, err := uplink.NewUplink(ctx, &config)
			require.NoError(t, err)
			defer ctx.Check(up.Close)

			project, err := up.OpenProject(ctx, satellite.Addr(), apikey)
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			bucketInfo, err := project.CreateBucket(ctx, bucketName, nil)
			require.NoError(t, err)

			assert.True(t, bucketInfo.PartnerID.IsZero())

			_, err = project.CreateBucket(ctx, bucketName, nil)
			require.Error(t, err)
		})

		t.Run("open with partner id", func(t *testing.T) {
			config := uplink.Config{}
			config.Volatile.Log = zaptest.NewLogger(t)
			config.Volatile.TLS.SkipPeerCAWhitelist = true
			config.Volatile.PartnerID = partnerID.String()

			up, err := uplink.NewUplink(ctx, &config)
			require.NoError(t, err)
			defer ctx.Check(up.Close)

			project, err := up.OpenProject(ctx, satellite.Addr(), apikey)
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			bucket, err := project.OpenBucket(ctx, bucketName, access)
			require.NoError(t, err)
			defer ctx.Check(bucket.Close)

			bucketInfo, _, err := project.GetBucketInfo(ctx, bucketName)
			require.NoError(t, err)
			assert.Equal(t, bucketInfo.PartnerID.String(), config.Volatile.PartnerID)
		})

		t.Run("open with different partner id", func(t *testing.T) {
			config := uplink.Config{}
			config.Volatile.Log = zaptest.NewLogger(t)
			config.Volatile.TLS.SkipPeerCAWhitelist = true
			config.Volatile.PartnerID = testrand.UUID().String()

			up, err := uplink.NewUplink(ctx, &config)
			require.NoError(t, err)
			defer ctx.Check(up.Close)

			project, err := up.OpenProject(ctx, satellite.Addr(), apikey)
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			bucket, err := project.OpenBucket(ctx, bucketName, access)
			require.NoError(t, err)
			defer ctx.Check(bucket.Close)

			bucketInfo, _, err := project.GetBucketInfo(ctx, bucketName)
			require.NoError(t, err)
			assert.NotEqual(t, bucketInfo.PartnerID.String(), config.Volatile.PartnerID)
		})
	})
}

// check that partner bucket attributes are stored and retrieved correctly.
func TestBucket_UserAgent(t *testing.T) {
	var (
		access     = uplink.NewEncryptionAccessWithDefaultKey(storj.Key{0, 1, 2, 3, 4})
		bucketName = "mightynein"
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apikey, err := uplink.ParseAPIKey(planet.Uplinks[0].APIKey[satellite.ID()].Serialize())
		require.NoError(t, err)

		t.Run("without user agent", func(t *testing.T) {
			config := uplink.Config{}
			config.Volatile.Log = zaptest.NewLogger(t)
			config.Volatile.TLS.SkipPeerCAWhitelist = true

			up, err := uplink.NewUplink(ctx, &config)
			require.NoError(t, err)
			defer ctx.Check(up.Close)

			project, err := up.OpenProject(ctx, satellite.Addr(), apikey)
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			bucketInfo, err := project.CreateBucket(ctx, bucketName, nil)
			require.NoError(t, err)

			assert.True(t, bucketInfo.PartnerID.IsZero())

			_, err = project.CreateBucket(ctx, bucketName, nil)
			require.Error(t, err)
		})

		t.Run("open with user agent", func(t *testing.T) {
			config := uplink.Config{}
			config.Volatile.Log = zaptest.NewLogger(t)
			config.Volatile.TLS.SkipPeerCAWhitelist = true
			config.Volatile.UserAgent = "Zenko"

			up, err := uplink.NewUplink(ctx, &config)
			require.NoError(t, err)
			defer ctx.Check(up.Close)

			project, err := up.OpenProject(ctx, satellite.Addr(), apikey)
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			bucket, err := project.OpenBucket(ctx, bucketName, access)
			require.NoError(t, err)
			defer ctx.Check(bucket.Close)

			bucketInfo, _, err := project.GetBucketInfo(ctx, bucketName)
			require.NoError(t, err)
			partnerID, err := uuid.Parse("8cd605fa-ad00-45b6-823e-550eddc611d6")
			require.NoError(t, err)
			assert.Equal(t, *partnerID, bucketInfo.PartnerID)
		})

		t.Run("open with different user agent", func(t *testing.T) {
			config := uplink.Config{}
			config.Volatile.Log = zaptest.NewLogger(t)
			config.Volatile.TLS.SkipPeerCAWhitelist = true
			config.Volatile.UserAgent = "Temporal"

			up, err := uplink.NewUplink(ctx, &config)
			require.NoError(t, err)
			defer ctx.Check(up.Close)

			project, err := up.OpenProject(ctx, satellite.Addr(), apikey)
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			bucket, err := project.OpenBucket(ctx, bucketName, access)
			require.NoError(t, err)
			defer ctx.Check(bucket.Close)

			bucketInfo, _, err := project.GetBucketInfo(ctx, bucketName)
			require.NoError(t, err)
			partnerID, err := uuid.Parse("8cd605fa-ad00-45b6-823e-550eddc611d6")
			require.NoError(t, err)
			assert.Equal(t, *partnerID, bucketInfo.PartnerID)
		})
	})
}

// check that bucket attributes are stored and retrieved correctly.
func TestBucketAttrs(t *testing.T) {
	var (
		access          = uplink.NewEncryptionAccessWithDefaultKey(storj.Key{0, 1, 2, 3, 4})
		bucketName      = "mightynein"
		shareSize       = memory.KiB.Int32()
		requiredShares  = 2
		stripeSize      = shareSize * int32(requiredShares)
		stripesPerBlock = 2
		inBucketConfig  = uplink.BucketConfig{
			PathCipher: storj.EncSecretBox,
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.EncAESGCM,
				BlockSize:   int32(stripesPerBlock) * stripeSize,
			},
			Volatile: struct {
				RedundancyScheme storj.RedundancyScheme
				SegmentsSize     memory.Size
			}{
				RedundancyScheme: storj.RedundancyScheme{
					Algorithm:      storj.ReedSolomon,
					ShareSize:      shareSize,
					RequiredShares: int16(requiredShares),
					RepairShares:   3,
					OptimalShares:  4,
					TotalShares:    5,
				},
				SegmentsSize: 688894,
			},
		}
	)

	cfg := testConfig{}
	cfg.uplinkCfg.Volatile.TLS.SkipPeerCAWhitelist = true

	testPlanetWithLibUplink(t, cfg,
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, proj *uplink.Project) {
			before := time.Now()
			bucket, err := proj.CreateBucket(ctx, bucketName, &inBucketConfig)
			require.NoError(t, err)

			assert.Equal(t, bucketName, bucket.Name)
			assert.Falsef(t, bucket.Created.Before(before), "impossible creation time %v", bucket.Created)

			got, err := proj.OpenBucket(ctx, bucketName, access)
			require.NoError(t, err)
			defer ctx.Check(got.Close)

			assert.Equal(t, bucketName, got.Name)
			assert.Equal(t, inBucketConfig.PathCipher, got.PathCipher)
			assert.Equal(t, inBucketConfig.EncryptionParameters, got.EncryptionParameters)
			assert.Equal(t, inBucketConfig.Volatile.RedundancyScheme, got.Volatile.RedundancyScheme)
			assert.Equal(t, inBucketConfig.Volatile.SegmentsSize, got.Volatile.SegmentsSize)
			assert.Equal(t, inBucketConfig, got.BucketConfig)

			err = proj.DeleteBucket(ctx, bucketName)
			require.NoError(t, err)
		})
}

// check that when uploading objects without any specific RS or encryption
// config, the bucket attributes apply. also when uploading objects _with_ more
// specific config, the specific config applies and not the bucket attrs.
func TestBucketAttrsApply(t *testing.T) {
	var (
		access          = uplink.NewEncryptionAccessWithDefaultKey(storj.Key{0, 1, 2, 3, 4})
		bucketName      = "dodecahedron"
		objectPath1     = "vax/vex/vox"
		objectContents  = "Willingham,Ray,Jaffe,Johnson,Riegel,O'Brien,Bailey,Mercer"
		shareSize       = 3 * memory.KiB.Int32()
		requiredShares  = 3
		stripeSize      = shareSize * int32(requiredShares)
		stripesPerBlock = 2
		inBucketConfig  = uplink.BucketConfig{
			PathCipher: storj.EncSecretBox,
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.EncSecretBox,
				BlockSize:   int32(stripesPerBlock) * stripeSize,
			},
			Volatile: struct {
				RedundancyScheme storj.RedundancyScheme
				SegmentsSize     memory.Size
			}{
				RedundancyScheme: storj.RedundancyScheme{
					Algorithm:      storj.ReedSolomon,
					ShareSize:      shareSize,
					RequiredShares: int16(requiredShares),
					RepairShares:   4,
					OptimalShares:  5,
					TotalShares:    5,
				},
				SegmentsSize: 1536,
			},
		}
		testConfig testConfig
	)

	// so our test object will not be inlined (otherwise it will lose its RS params)
	testConfig.uplinkCfg.Volatile.MaxInlineSize = 1
	testConfig.uplinkCfg.Volatile.TLS.SkipPeerCAWhitelist = true

	testPlanetWithLibUplink(t, testConfig,
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, proj *uplink.Project) {
			_, err := proj.CreateBucket(ctx, bucketName, &inBucketConfig)
			require.NoError(t, err)

			bucket, err := proj.OpenBucket(ctx, bucketName, access)
			require.NoError(t, err)
			defer ctx.Check(bucket.Close)

			{
				buf := bytes.NewBufferString(objectContents)
				err := bucket.UploadObject(ctx, objectPath1, buf, nil)
				require.NoError(t, err)
			}

			readBack, err := bucket.OpenObject(ctx, objectPath1)
			require.NoError(t, err)
			defer ctx.Check(readBack.Close)

			assert.Equal(t, inBucketConfig.EncryptionParameters, readBack.Meta.Volatile.EncryptionParameters)
			assert.Equal(t, inBucketConfig.Volatile.RedundancyScheme, readBack.Meta.Volatile.RedundancyScheme)
			assert.Equal(t, inBucketConfig.Volatile.SegmentsSize.Int64(), readBack.Meta.Volatile.SegmentsSize)

			strm, err := readBack.DownloadRange(ctx, 0, -1)
			require.NoError(t, err)
			defer ctx.Check(strm.Close)

			contents, err := ioutil.ReadAll(strm)
			require.NoError(t, err)
			assert.Equal(t, string(contents), objectContents)
		})
}
