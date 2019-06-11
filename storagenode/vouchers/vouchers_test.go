// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package vouchers_test

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestVouchersDB(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		vdb := db.Vouchers()

		satellite := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())
		storagenode := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion())

		expiration, err := ptypes.TimestampProto(time.Now().UTC().Add(24 * time.Hour))
		require.NoError(t, err)

		voucher := &pb.Voucher{
			SatelliteId:   satellite.ID,
			StorageNodeId: storagenode.ID,
			Expiration:    expiration,
		}

		// GetValid with no entry
		result, err := vdb.GetValid(ctx, []storj.NodeID{satellite.ID})
		require.NoError(t, err)
		assert.Nil(t, result)

		// basic Put test
		err = vdb.Put(ctx, voucher)
		require.NoError(t, err)

		// basic NeedVoucher test
		expirationBuffer := 48 * time.Hour
		need, err := vdb.NeedVoucher(ctx, satellite.ID, expirationBuffer)
		require.NoError(t, err)
		require.True(t, need)

		// basic GetValid test
		result, err = vdb.GetValid(ctx, []storj.NodeID{satellite.ID})
		require.NoError(t, err)
		require.Equal(t, voucher.SatelliteId, result.SatelliteId)
		require.Equal(t, voucher.StorageNodeId, result.StorageNodeId)

		expectedTime, err := ptypes.Timestamp(voucher.GetExpiration())
		require.NoError(t, err)
		actualTime, err := ptypes.Timestamp(result.GetExpiration())
		require.NoError(t, err)

		require.Equal(t, expectedTime, actualTime)

		// Test duplicate satellite id updates voucher
		voucher.Expiration, err = ptypes.TimestampProto(time.Now().UTC().Add(48 * time.Hour))
		require.NoError(t, err)

		err = vdb.Put(ctx, voucher)
		require.NoError(t, err)

		result, err = vdb.GetValid(ctx, []storj.NodeID{satellite.ID})
		require.NoError(t, err)

		expectedTime, err = ptypes.Timestamp(voucher.GetExpiration())
		require.NoError(t, err)
		actualTime, err = ptypes.Timestamp(result.GetExpiration())
		require.NoError(t, err)

		require.Equal(t, expectedTime, actualTime)
	})
}

func TestVouchersService(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 5, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Vouchers.Expiration = 1
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		node.Vouchers.Loop.Pause()
		node.Storage2.Sender.Loop.Pause()

		// run service and assert no satellites in voucherDB
		err := node.Vouchers.RunOnce(ctx)
		require.NoError(t, err)
		satellites, err := node.DB.Vouchers().ListSatellites(ctx)
		require.NoError(t, err)
		assert.Len(t, satellites, 0)

		// upload to node to get orders
		data := make([]byte, 5*memory.KiB)
		_, err = rand.Read(data)
		require.NoError(t, err)

		time.Sleep(time.Second)
		for i, _ := range planet.Satellites {
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[i], "testbucket", "testpath", data)
			require.NoError(t, err)
		}

		// archive orders
		orderInfo, err := node.DB.Orders().ListUnsent(ctx, 5)
		assert.Len(t, orderInfo, 5)
		for _, o := range orderInfo {
			node.DB.Orders().Archive(ctx, o.Limit.SatelliteId, o.Limit.SerialNumber, orders.StatusAccepted)
		}

		// run service and check vouchers have been received
		err = node.Vouchers.RunOnce(ctx)
		require.NoError(t, err)

		for _, sat := range planet.Satellites {
			voucher, err := node.DB.Vouchers().GetValid(ctx, []storj.NodeID{sat.ID()})
			require.NoError(t, err)
			assert.NotNil(t, voucher)
		}
	})
}
