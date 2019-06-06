// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package vouchers_test

import (
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	// "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestVouchers(t *testing.T) {
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

		// basic Put test
		err = vdb.Put(ctx, voucher)
		require.NoError(t, err)

		// basic GetExpiring test
		expirationBuffer := 48 * time.Hour
		expiring, err := vdb.GetExpiring(ctx, expirationBuffer)
		require.NoError(t, err)
		require.Equal(t, satellite.ID, expiring[0])

		// basic GetValid test
		result, err := vdb.GetValid(ctx, []storj.NodeID{satellite.ID})
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
