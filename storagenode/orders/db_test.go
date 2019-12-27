// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestDB(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		ordersdb := db.Orders()

		storagenode := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())

		satellite0 := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion())

		piece := storj.NewPieceID()

		// basic test
		emptyUnsent, err := ordersdb.ListUnsent(ctx, 100)
		require.NoError(t, err)
		require.Len(t, emptyUnsent, 0)

		emptyArchive, err := ordersdb.ListArchived(ctx, 100)
		require.NoError(t, err)
		require.Len(t, emptyArchive, 0)

		now := time.Now()

		piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
		require.NoError(t, err)

		infos := make([]*orders.Info, 2)
		for i := 0; i < len(infos); i++ {

			serialNumber := testrand.SerialNumber()
			limit, err := signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satellite0), &pb.OrderLimit{
				SerialNumber:    serialNumber,
				SatelliteId:     satellite0.ID,
				UplinkPublicKey: piecePublicKey,
				StorageNodeId:   storagenode.ID,
				PieceId:         piece,
				Limit:           100,
				Action:          pb.PieceAction_GET,
				OrderCreation:   now.AddDate(0, 0, -1),
				PieceExpiration: now,
				OrderExpiration: now,
			})
			require.NoError(t, err)

			order, err := signing.SignUplinkOrder(ctx, piecePrivateKey, &pb.Order{
				SerialNumber: serialNumber,
				Amount:       50,
			})
			require.NoError(t, err)

			infos[i] = &orders.Info{
				Limit: limit,
				Order: order,
			}
		}

		// basic add
		err = ordersdb.Enqueue(ctx, infos[0])
		require.NoError(t, err)

		// duplicate add
		err = ordersdb.Enqueue(ctx, infos[0])
		require.Error(t, err, "duplicate add")

		unsent, err := ordersdb.ListUnsent(ctx, 100)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff([]*orders.Info{infos[0]}, unsent, cmp.Comparer(pb.Equal)))

		// Another add
		err = ordersdb.Enqueue(ctx, infos[1])
		require.NoError(t, err)

		unsent, err = ordersdb.ListUnsent(ctx, 100)
		require.NoError(t, err)
		require.Empty(t,
			cmp.Diff([]*orders.Info{infos[0], infos[1]}, unsent, cmp.Comparer(pb.Equal)),
		)

		// list by group
		unsentGrouped, err := ordersdb.ListUnsentBySatellite(ctx)
		require.NoError(t, err)

		expectedGrouped := map[storj.NodeID][]*orders.Info{
			satellite0.ID: {
				{Limit: infos[0].Limit, Order: infos[0].Order},
				{Limit: infos[1].Limit, Order: infos[1].Order},
			},
		}
		require.Empty(t, cmp.Diff(expectedGrouped, unsentGrouped, cmp.Comparer(pb.Equal)))

		// test archival
		archivedAt := time.Now().UTC()
		err = ordersdb.Archive(ctx, archivedAt, orders.ArchiveRequest{
			Satellite: satellite0.ID,
			Serial:    infos[0].Limit.SerialNumber,
			Status:    orders.StatusAccepted,
		})
		require.NoError(t, err)

		// duplicate archive
		err = ordersdb.Archive(ctx, archivedAt, orders.ArchiveRequest{
			Satellite: satellite0.ID,
			Serial:    infos[0].Limit.SerialNumber,
			Status:    orders.StatusRejected,
		})
		require.Error(t, err)
		require.True(t,
			orders.OrderNotFoundError.Has(err),
			"expected orders.OrderNotFoundError class",
		)

		// one new archive and one duplicated
		err = ordersdb.Archive(ctx, archivedAt, orders.ArchiveRequest{
			Satellite: satellite0.ID,
			Serial:    infos[0].Limit.SerialNumber,
			Status:    orders.StatusRejected,
		}, orders.ArchiveRequest{
			Satellite: satellite0.ID,
			Serial:    infos[1].Limit.SerialNumber,
			Status:    orders.StatusRejected,
		})
		require.Error(t, err)
		require.True(t,
			orders.OrderNotFoundError.Has(err),
			"expected ErrUnsentOrderNotFoundError class",
		)

		// shouldn't be in unsent list
		unsent, err = ordersdb.ListUnsent(ctx, 100)
		require.NoError(t, err)
		require.Len(t, unsent, 0)

		// it should now be in the archive
		archived, err := ordersdb.ListArchived(ctx, 100)
		require.NoError(t, err)
		require.Len(t, archived, 2)

		require.Empty(t, cmp.Diff([]*orders.ArchivedInfo{
			{
				Limit: infos[0].Limit,
				Order: infos[0].Order,

				Status:     orders.StatusAccepted,
				ArchivedAt: archived[0].ArchivedAt,
			},
			{
				Limit: infos[1].Limit,
				Order: infos[1].Order,

				Status:     orders.StatusRejected,
				ArchivedAt: archived[1].ArchivedAt,
			},
		}, archived, cmp.Comparer(pb.Equal)))

		// with 1 hour ttl, archived order should not be deleted
		n, err := db.Orders().CleanArchive(ctx, time.Hour)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		// with 1 nanosecond ttl, archived order should be deleted
		n, err = db.Orders().CleanArchive(ctx, time.Nanosecond)
		require.NoError(t, err)
		require.Equal(t, 2, n)
	})
}

func TestDB_Trivial(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		satelliteID, serial := testrand.NodeID(), testrand.SerialNumber()

		{ // Ensure Enqueue works at all
			err := db.Orders().Enqueue(ctx, &orders.Info{
				Order: &pb.Order{},
				Limit: &pb.OrderLimit{
					SatelliteId:     satelliteID,
					SerialNumber:    serial,
					OrderExpiration: time.Now(),
				},
			})
			require.NoError(t, err)
		}

		{ // Ensure ListUnsent works at all
			infos, err := db.Orders().ListUnsent(ctx, 1)
			require.NoError(t, err)
			require.Len(t, infos, 1)
		}

		{ // Ensure ListUnsentBySatellite works at all
			infos, err := db.Orders().ListUnsentBySatellite(ctx)
			require.NoError(t, err)
			require.Len(t, infos, 1)
			require.Contains(t, infos, satelliteID)
			require.Len(t, infos[satelliteID], 1)
		}

		{ // Ensure Archive works at all
			err := db.Orders().Archive(ctx, time.Now().UTC(), orders.ArchiveRequest{satelliteID, serial, orders.StatusAccepted})
			require.NoError(t, err)
		}

		{ // Ensure ListArchived works at all
			infos, err := db.Orders().ListArchived(ctx, 1)
			require.NoError(t, err)
			require.Len(t, infos, 1)
		}
		{ // Ensure CleanArchive works at all
			n, err := db.Orders().CleanArchive(ctx, time.Nanosecond)
			require.NoError(t, err)
			require.Equal(t, 1, n)
		}
	})
}
