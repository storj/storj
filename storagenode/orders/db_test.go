// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
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

		serialNumber := testrand.SerialNumber()

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

		info := &orders.Info{
			Limit: limit,
			Order: order,
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
			satellite0.ID: {
				{Limit: limit, Order: order},
			},
		}
		require.Empty(t, cmp.Diff(expectedGrouped, unsentGrouped, cmp.Comparer(pb.Equal)))

		// test archival
		err = ordersdb.Archive(ctx, orders.ArchiveRequest{satellite0.ID, serialNumber, orders.StatusAccepted})
		require.NoError(t, err)

		// duplicate archive
		err = ordersdb.Archive(ctx, orders.ArchiveRequest{satellite0.ID, serialNumber, orders.StatusRejected})
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
				Limit: limit,
				Order: order,

				Status:     orders.StatusAccepted,
				ArchivedAt: archived[0].ArchivedAt,
			},
		}, archived, cmp.Comparer(pb.Equal)))

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
			_, err := db.Orders().ListUnsent(ctx, 1)
			require.NoError(t, err)
		}

		{ // Ensure ListUnsentBySatellite works at all
			_, err := db.Orders().ListUnsentBySatellite(ctx)
			require.NoError(t, err)
		}

		{ // Ensure Archive works at all
			err := db.Orders().Archive(ctx, orders.ArchiveRequest{satelliteID, serial, orders.StatusAccepted})
			require.NoError(t, err)
		}

		{ // Ensure ListArchived works at all
			_, err := db.Orders().ListArchived(ctx, 1)
			require.NoError(t, err)
		}
	})
}
