// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"bytes"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/storj"
)

type testConfig struct {
	uplinkCfg Config
}

func testPlanetWithLibUplink(t *testing.T, cfg testConfig, encKey *storj.Key,
	testFunc func(*testing.T, *testcontext.Context, *testplanet.Planet, *Project)) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// we only use testUplink for the free API key, until such time
		// as testplanet makes it easy to get another way :D
		testUplink := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		cfg.uplinkCfg.Volatile.TLS.SkipPeerCAWhitelist = true

		apiKey, err := ParseAPIKey(testUplink.APIKey[satellite.ID()])
		if err != nil {
			t.Fatalf("could not parse API key from testplanet: %v", err)
		}
		uplink, err := NewUplink(ctx, &cfg.uplinkCfg)
		if err != nil {
			t.Fatalf("could not create new Uplink object: %v", err)
		}
		defer ctx.Check(uplink.Close)
		var projectOptions ProjectOptions
		projectOptions.Volatile.EncryptionKey = encKey
		proj, err := uplink.OpenProject(ctx, satellite.Addr(), apiKey, &projectOptions)
		if err != nil {
			t.Fatalf("could not open project from libuplink under testplanet: %v", err)
		}
		defer ctx.Check(proj.Close)

		testFunc(t, ctx, planet, proj)
	})
}

func simpleEncryptionAccess(encKey string) (access EncryptionAccess) {
	copy(access.Key[:], encKey)
	return access
}

// check that bucket attributes are stored and retrieved correctly.
func TestBucketAttrs(t *testing.T) {
	var (
		access         = simpleEncryptionAccess("voxmachina")
		bucketName     = "mightynein"
		inBucketConfig = BucketConfig{
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
					ShareSize:      memory.KiB.Int32(),
					RequiredShares: 2,
					RepairShares:   3,
					OptimalShares:  4,
					TotalShares:    5,
				},
				SegmentsSize: 688894,
			},
		}
	)

	testPlanetWithLibUplink(t, testConfig{}, &access.Key,
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, proj *Project) {
			before := time.Now()
			bucket, err := proj.CreateBucket(ctx, bucketName, &inBucketConfig)
			require.NoError(t, err)

			assert.Equal(t, bucketName, bucket.Name)
			assert.Falsef(t, bucket.Created.Before(before), "impossible creation time %v", bucket.Created)

			got, err := proj.OpenBucket(ctx, bucketName, &access)
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
		access         = simpleEncryptionAccess("howdoyouwanttodothis")
		bucketName     = "dodecahedron"
		objectPath1    = "vax/vex/vox"
		objectContents = "Willingham,Ray,Jaffe,Johnson,Riegel,O'Brien,Bailey,Mercer"
		inBucketConfig = BucketConfig{
			PathCipher: storj.EncSecretBox,
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.EncSecretBox,
				BlockSize:   768,
			},
			Volatile: struct {
				RedundancyScheme storj.RedundancyScheme
				SegmentsSize     memory.Size
			}{
				RedundancyScheme: storj.RedundancyScheme{
					Algorithm:      storj.ReedSolomon,
					ShareSize:      (3 * memory.KiB).Int32(),
					RequiredShares: 3,
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

	testPlanetWithLibUplink(t, testConfig, &access.Key,
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, proj *Project) {
			_, err := proj.CreateBucket(ctx, bucketName, &inBucketConfig)
			require.NoError(t, err)

			bucket, err := proj.OpenBucket(ctx, bucketName, &access)
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
