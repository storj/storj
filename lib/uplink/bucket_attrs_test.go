// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"bytes"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

// hackyGetBucketAttribution exists to read the unexported Attribution field on a bucket.
// It should be removed once there's an exported way to do this.
func hackyGetBucketAttribution(bucket *uplink.Bucket) string {
	return reflect.ValueOf(bucket).
		Elem().
		FieldByName("bucket").
		FieldByName("Attribution").
		String()
}

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
		cfg.uplinkCfg.Volatile.TLS.SkipPeerCAWhitelist = true

		apiKey, err := uplink.ParseAPIKey(testUplink.APIKey[satellite.ID()])
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
func TestPartnerBucketAttrs(t *testing.T) {
	var (
		access     = uplink.NewEncryptionAccessWithDefaultKey(storj.Key{0, 1, 2, 3, 4})
		bucketName = "mightynein"
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apikey, err := uplink.ParseAPIKey(planet.Uplinks[0].APIKey[satellite.ID()])
		require.NoError(t, err)

		partnerID := testrand.UUID()

		t.Run("without partner id", func(t *testing.T) {
			config := uplink.Config{}
			config.Volatile.TLS.SkipPeerCAWhitelist = true

			up, err := uplink.NewUplink(ctx, &config)
			require.NoError(t, err)
			defer ctx.Check(up.Close)

			project, err := up.OpenProject(ctx, satellite.Addr(), apikey)
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			bucketInfo, err := project.CreateBucket(ctx, bucketName, nil) // TODO: by specifying config here it can be rolled into the testAttrs test
			require.NoError(t, err)

			assert.Equal(t, bucketInfo.Attribution, "")
		})

		t.Run("open with partner id", func(t *testing.T) {
			config := uplink.Config{}
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

			assert.Equal(t, hackyGetBucketAttribution(bucket), partnerID.String())
		})

		t.Run("open with different partner id", func(t *testing.T) {
			config := uplink.Config{}
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

			// shouldn't change
			assert.Equal(t, hackyGetBucketAttribution(bucket), partnerID.String())
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

	testPlanetWithLibUplink(t, testConfig{},
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
