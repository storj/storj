// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"crypto/rand"
	"storj.io/storj/internal/testidentity"
	"testing"

	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestOrders(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		ordersdb := db.Orders()

		storagenode := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())

		satellite0 := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion())

		uplink := testidentity.MustPregeneratedSignedIdentity(3, storj.LatestIDVersion())
		piece := storj.NewPieceID()

		serialNumber := newRandomSerial()

		// basic test
		emptyUnsent, err := ordersdb.ListUnsent(ctx, 100)
		require.NoError(t, err)
		require.Len(t, emptyUnsent, 0)

		emptyArchive, err := ordersdb.ListArchived(ctx, 100)
		require.NoError(t, err)
		require.Len(t, emptyArchive, 0)

		now := ptypes.TimestampNow()

		limit, err := signing.SignOrderLimit(signing.SignerFromFullIdentity(satellite0), &pb.OrderLimit2{
			SerialNumber:    serialNumber,
			SatelliteId:     satellite0.ID,
			UplinkId:        uplink.ID,
			StorageNodeId:   storagenode.ID,
			PieceId:         piece,
			Limit:           100,
			Action:          pb.PieceAction_GET,
			PieceExpiration: now,
			OrderExpiration: now,
		})
		require.NoError(t, err)

		order, err := signing.SignOrder(signing.SignerFromFullIdentity(uplink), &pb.Order2{
			SerialNumber: serialNumber,
			Amount:       50,
		})
		require.NoError(t, err)

		info := &orders.Info{
			Limit:  limit,
			Order:  order,
			Uplink: uplink.PeerIdentity(),
		}

		// basic add
		err = ordersdb.Enqueue(ctx, info)
		require.NoError(t, err)

		// duplicate add
		err = ordersdb.Enqueue(ctx, info)
		require.Error(t, err, "duplicate add")

		unsent, err := ordersdb.ListUnsent(ctx, 100)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff([]*orders.Info{info}, unsent, cmp.Comparer(pb.Equal)))

		// list by group
		unsentGrouped, err := ordersdb.ListUnsentBySatellite(ctx)
		require.NoError(t, err)

		expectedGrouped := map[storj.NodeID][]*orders.Info{
			satellite0.ID: []*orders.Info{
				{Limit: limit, Order: order},
			},
		}
		require.Empty(t, cmp.Diff(expectedGrouped, unsentGrouped, cmp.Comparer(pb.Equal)))

		// test archival
		err = ordersdb.Archive(ctx, satellite0.ID, serialNumber, orders.StatusAccepted)
		require.NoError(t, err)

		// duplicate archive
		err = ordersdb.Archive(ctx, satellite0.ID, serialNumber, orders.StatusRejected)
		require.Error(t, err)

		// shouldn't be in unsent list
		unsent, err = ordersdb.ListUnsent(ctx, 100)
		require.NoError(t, err)
		require.Len(t, unsent, 0)

		// it should now be in the archive
		archived, err := ordersdb.ListArchived(ctx, 100)
		require.NoError(t, err)
		require.Len(t, archived, 1)

		require.Empty(t, cmp.Diff([]*orders.ArchivedInfo{
			{
				Limit:  limit,
				Order:  order,
				Uplink: uplink.PeerIdentity(),

				Status:     orders.StatusAccepted,
				ArchivedAt: archived[0].ArchivedAt,
			},
		}, archived, cmp.Comparer(pb.Equal)))

	})
}

// TODO: move somewhere better
func newRandomSerial() storj.SerialNumber {
	var serial storj.SerialNumber
	_, _ = rand.Read(serial[:])
	return serial
}
