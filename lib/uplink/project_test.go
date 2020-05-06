// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"bytes"
	"io"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/private/testplanet"
	newuplink "storj.io/uplink"
)

func TestProjectListBuckets(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 1,
		UplinkCount:      1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			cfg := uplink.Config{}
			cfg.Volatile.Log = zaptest.NewLogger(t)
			cfg.Volatile.TLS.SkipPeerCAWhitelist = true

			access, err := planet.Uplinks[0].GetConfig(planet.Satellites[0]).GetAccess()
			require.NoError(t, err)

			ul, err := uplink.NewUplink(ctx, &cfg)
			require.NoError(t, err)
			defer ctx.Check(ul.Close)

			p, err := ul.OpenProject(ctx, access.SatelliteAddr, access.APIKey)
			require.NoError(t, err)

			// create 6 test buckets
			for i := 0; i < 6; i++ {
				_, err = p.CreateBucket(ctx, "test"+strconv.Itoa(i), nil)
				require.NoError(t, err)
			}

			// setup list options so that we only list 3 buckets
			// at a time in alphabetical order starting at ""
			list := uplink.BucketListOptions{
				Direction: storj.Forward,
				Limit:     3,
			}

			result, err := p.ListBuckets(ctx, &list)
			require.NoError(t, err)
			require.Equal(t, 3, len(result.Items))
			require.Equal(t, "test0", result.Items[0].Name)
			require.Equal(t, "test1", result.Items[1].Name)
			require.Equal(t, "test2", result.Items[2].Name)
			require.True(t, result.More)

			list = list.NextPage(result)
			result, err = p.ListBuckets(ctx, &list)
			require.NoError(t, err)
			require.Equal(t, 3, len(result.Items))
			require.Equal(t, "test3", result.Items[0].Name)
			require.Equal(t, "test4", result.Items[1].Name)
			require.Equal(t, "test5", result.Items[2].Name)
			require.False(t, result.More)

			// List with restrictions
			access.APIKey, access.EncryptionAccess, err =
				access.EncryptionAccess.Restrict(access.APIKey,
					uplink.EncryptionRestriction{Bucket: "test0"},
					uplink.EncryptionRestriction{Bucket: "test1"})
			require.NoError(t, err)

			p, err = ul.OpenProject(ctx, access.SatelliteAddr, access.APIKey)
			require.NoError(t, err)
			defer ctx.Check(p.Close)

			list = uplink.BucketListOptions{
				Direction: storj.Forward,
				Limit:     3,
			}
			result, err = p.ListBuckets(ctx, &list)
			require.NoError(t, err)
			require.Equal(t, 2, len(result.Items))
			require.Equal(t, "test0", result.Items[0].Name)
			require.Equal(t, "test1", result.Items[1].Name)
			require.False(t, result.More)
		})
}

func TestProjectOpenNewBucket(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[satellite.ID()]
		uplinkConfig := newuplink.Config{}
		access, err := uplinkConfig.RequestAccessWithPassphrase(ctx, satellite.URL(), apiKey.Serialize(), "mypassphrase")
		require.NoError(t, err)

		project, err := uplinkConfig.OpenProject(ctx, access)
		require.NoError(t, err)

		// create bucket and upload a file with new libuplink
		bucketName := "a-bucket"
		bucket, err := project.CreateBucket(ctx, bucketName)
		require.NoError(t, err)
		require.NotNil(t, bucket)

		upload, err := project.UploadObject(ctx, bucketName, "test-file.dat", nil)
		require.NoError(t, err)

		expectedData := testrand.Bytes(1 * memory.KiB)
		_, err = io.Copy(upload, bytes.NewBuffer(expectedData))
		require.NoError(t, err)

		err = upload.Commit()
		require.NoError(t, err)

		serializedAccess, err := access.Serialize()
		require.NoError(t, err)

		// download uploaded file with old libuplink
		oldUplink, err := planet.Uplinks[0].NewLibuplink(ctx)
		require.NoError(t, err)

		scope, err := uplink.ParseScope(serializedAccess)
		require.NoError(t, err)

		oldProject, err := oldUplink.OpenProject(ctx, scope.SatelliteAddr, scope.APIKey)
		require.NoError(t, err)
		defer ctx.Check(oldProject.Close)

		oldBucket, err := oldProject.OpenBucket(ctx, bucketName, scope.EncryptionAccess)
		require.NoError(t, err)
		defer ctx.Check(oldBucket.Close)

		rc, err := oldBucket.Download(ctx, "test-file.dat")
		require.NoError(t, err)

		var downloaded bytes.Buffer
		_, err = io.Copy(&downloaded, rc)
		require.NoError(t, err)

		require.Equal(t, expectedData, downloaded.Bytes())
	})
}
