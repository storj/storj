// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package vouchers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
)

func TestVouchers(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		satellite := planet.Satellites[0].Local().Node
		conn, err := planet.StorageNodes[0].Transport.DialNode(ctx, &satellite)
		require.NoError(t, err)

		client := pb.NewVouchersClient(conn)

		_, err = client.Request(ctx, &pb.VoucherRequest{})
		assert.Equal(t, err.Error(), "rpc error: code = Unknown desc = Voucher service not implemented")
	})
}
