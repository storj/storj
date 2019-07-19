// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

func TestUploadDownload(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 6,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		cfg := uplink.Config{}
		cfg.Volatile.TLS.SkipPeerCAWhitelist = true

		satelliteAddr := planet.Satellites[0].Addr()
		apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		ul, err := uplink.NewUplink(ctx, &cfg)
		require.NoError(t, err)
		defer ctx.Check(ul.Close)

		// setup key
		key, err := uplink.ParseAPIKey(apiKey)
		require.NoError(t, err)

		// open project
		project, err := ul.OpenProject(ctx, satelliteAddr, key)
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		// setup bucket
		config := &uplink.BucketConfig{}
		config.PathCipher = storj.EncAESGCM
		config.EncryptionParameters.CipherSuite = storj.EncAESGCM
		config.EncryptionParameters.BlockSize = 2048

		config.Volatile.RedundancyScheme.Algorithm = storj.ReedSolomon
		config.Volatile.RedundancyScheme.ShareSize = 1024
		config.Volatile.RedundancyScheme.RequiredShares = 2
		config.Volatile.RedundancyScheme.RepairShares = 4
		config.Volatile.RedundancyScheme.OptimalShares = 5
		config.Volatile.RedundancyScheme.TotalShares = 6

		_, err = project.CreateBucket(ctx, "main", config)
		require.NoError(t, err)

		// Make a key
		encKey, err := project.SaltedKeyFromPassphrase(ctx, "my secret passphrase")
		require.NoError(t, err)

		// Make an encryption context
		access := uplink.NewEncryptionAccessWithDefaultKey(*encKey)

		// open bucket
		bucket, err := project.OpenBucket(ctx, "main", access)
		require.NoError(t, err)
		defer ctx.Check(bucket.Close)

		// upload data
		uploadOptions := &uplink.UploadOptions{}
		uploadOptions.ContentType = "text/plain"
		uploadOptions.Expires = time.Now().Add(600 * 24 * time.Hour)

		upload, err := bucket.NewWriter(ctx, "a", uploadOptions)
		require.NoError(t, err)
		defer ctx.Check(upload.Close)

		data := testrand.Bytes(1024 * 1024)
		for len(data) > 0 {
			write := len(data)
			if write > 256 {
				write = 256
			}

			written, err := upload.Write(data[:write])
			require.NoError(t, err)
			data = data[:written]
		}

		require.NoError(t, upload.Close())

		// download data
		download, err := bucket.NewReader(ctx, "a")
		require.NoError(t, err)
		defer ctx.Check(download.Close)

		downloaded := make([]byte, 3*1024*1024)
		head := 0
		for {
			read, err := download.Read(downloaded[head : head+256])
			head += read
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		require.Equal(t, data, downloaded[:head])
	})
}
