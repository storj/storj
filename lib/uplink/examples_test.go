// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/lib/uplink"
)

func TestBucketExamples(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 1,
		UplinkCount:      1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			cfg := uplink.Config{}
			cfg.Volatile.TLS.SkipPeerCAWhitelist = true

			out := bytes.NewBuffer(nil)
			err := ListBucketsExample(ctx, planet.Satellites[0].Local().Address.Address, planet.Uplinks[0].APIKey[planet.Satellites[0].ID()], &cfg, out)
			require.NoError(t, err)
			require.Equal(t, out.String(), "")

			out = bytes.NewBuffer(nil)
			err = CreateBucketExample(ctx, planet.Satellites[0].Local().Address.Address, planet.Uplinks[0].APIKey[planet.Satellites[0].ID()], &cfg, out)
			require.NoError(t, err)
			require.Equal(t, out.String(), "success!\n")

			out = bytes.NewBuffer(nil)
			err = ListBucketsExample(ctx, planet.Satellites[0].Local().Address.Address, planet.Uplinks[0].APIKey[planet.Satellites[0].ID()], &cfg, out)
			require.NoError(t, err)
			require.Equal(t, out.String(), "Bucket: testbucket\n")

			out = bytes.NewBuffer(nil)
			err = DeleteBucketExample(ctx, planet.Satellites[0].Local().Address.Address, planet.Uplinks[0].APIKey[planet.Satellites[0].ID()], &cfg, out)
			require.NoError(t, err)
			require.Equal(t, out.String(), "success!\n")

			out = bytes.NewBuffer(nil)
			err = ListBucketsExample(ctx, planet.Satellites[0].Local().Address.Address, planet.Uplinks[0].APIKey[planet.Satellites[0].ID()], &cfg, out)
			require.NoError(t, err)
			require.Equal(t, out.String(), "")
		})
}
