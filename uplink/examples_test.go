// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/private/testplanet"
)

func TestBucketExamples(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 1,
		UplinkCount:      1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			cfg := uplink.Config{}
			cfg.Volatile.Log = zaptest.NewLogger(t)
			cfg.Volatile.TLS.SkipPeerCAWhitelist = true

			satelliteAddr := planet.Satellites[0].Local().Address.Address
			apiKey := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()].Serialize()

			out := bytes.NewBuffer(nil)
			err := ListBucketsExample(ctx, satelliteAddr, apiKey, &cfg, out)
			require.NoError(t, err)
			require.Equal(t, out.String(), "")

			out = bytes.NewBuffer(nil)
			err = CreateBucketExample(ctx, satelliteAddr, apiKey, &cfg, out)
			require.NoError(t, err)
			require.Equal(t, out.String(), "success!\n")

			out = bytes.NewBuffer(nil)
			err = ListBucketsExample(ctx, satelliteAddr, apiKey, &cfg, out)
			require.NoError(t, err)
			require.Equal(t, out.String(), "Bucket: testbucket\n")

			out = bytes.NewBuffer(nil)
			err = DeleteBucketExample(ctx, satelliteAddr, apiKey, &cfg, out)
			require.NoError(t, err)
			require.Equal(t, out.String(), "success!\n")

			out = bytes.NewBuffer(nil)
			err = ListBucketsExample(ctx, satelliteAddr, apiKey, &cfg, out)
			require.NoError(t, err)
			require.Equal(t, out.String(), "")

			out = bytes.NewBuffer(nil)
			access, err := CreateEncryptionKeyExampleByAdmin1(ctx, satelliteAddr, apiKey, &cfg, out)
			require.NoError(t, err)
			require.Equal(t, out.String(), "success!\n")

			out = bytes.NewBuffer(nil)
			err = CreateEncryptionKeyExampleByAdmin2(ctx, satelliteAddr, apiKey, access, &cfg, out)
			require.NoError(t, err)
			require.Equal(t, out.String(), "hello world\n")

			out = bytes.NewBuffer(nil)
			userScope, err := RestrictAccessExampleByAdmin(ctx, satelliteAddr, apiKey, access, &cfg, out)
			require.NoError(t, err)
			require.Equal(t, out.String(), "success!\n")

			out = bytes.NewBuffer(nil)
			err = RestrictAccessExampleByUser(ctx, userScope, &cfg, out)
			require.NoError(t, err)
			require.Equal(t, out.String(), "hello world\n")
		})
}
