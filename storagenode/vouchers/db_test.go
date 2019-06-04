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

		storagenode := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())

		satellite0 := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion())

		expiration, err := ptypes.TimestampProto(time.Now().UTC().Add(24 * time.Hour))
		require.NoError(t, err)

		voucher := &pb.Voucher{
			SatelliteId:   satellite0.ID,
			StorageNodeId: storagenode.ID,
			Expiration:    expiration,
		}

		err = vdb.Put(ctx, voucher)
		require.NoError(t, err)

		expiring, err := vdb.GetExpiring(ctx)
		require.NoError(t, err)
		require.Equal(t, satellite0.ID, expiring[0])

		result, err := vdb.PresentVoucher(ctx, []storj.NodeID{satellite0.ID})
		require.NoError(t, err)
		require.Equal(t, voucher.SatelliteId, result.SatelliteId)
		require.Equal(t, voucher.StorageNodeId, result.StorageNodeId)
	})
}
