// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/orders"
)

// TODO remove when db is removed.
func TestOrderDBSettle(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		satellite.Audit.Worker.Loop.Pause()
		node := planet.StorageNodes[0]
		service := node.Storage2.Orders
		service.Sender.Pause()
		service.Cleanup.Pause()

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))
		_, orderLimits, piecePrivateKey, err := satellite.Orders.Service.CreatePutOrderLimits(
			ctx,
			bucketID,
			[]*overlay.SelectedNode{
				{ID: node.ID(), LastIPPort: "fake", Address: new(pb.NodeAddress)},
			},
			time.Now().Add(2*time.Hour),
			2000,
		)
		require.NoError(t, err)
		require.Len(t, orderLimits, 1)

		orderLimit := orderLimits[0].Limit
		order := &pb.Order{
			SerialNumber: orderLimit.SerialNumber,
			Amount:       1000,
		}
		signedOrder, err := signing.SignUplinkOrder(ctx, piecePrivateKey, order)
		require.NoError(t, err)
		order0 := &orders.Info{
			Limit: orderLimit,
			Order: signedOrder,
		}

		// enter orders into unsent_orders
		err = node.DB.Orders().Enqueue(ctx, order0)
		require.NoError(t, err)

		toSend, err := node.DB.Orders().ListUnsent(ctx, 10)
		require.NoError(t, err)
		require.Len(t, toSend, 1)

		// trigger order send
		service.Sender.TriggerWait()

		toSend, err = node.DB.Orders().ListUnsent(ctx, 10)
		require.NoError(t, err)
		require.Len(t, toSend, 0)

		archived, err := node.DB.Orders().ListArchived(ctx, 10)
		require.NoError(t, err)
		require.Len(t, archived, 1)
	})
}

func TestOrderFileStoreSettle(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]
		satellite.Audit.Worker.Loop.Pause()
		node := planet.StorageNodes[0]
		service := node.Storage2.Orders
		service.Sender.Pause()
		service.Cleanup.Pause()

		// upload a file to generate an order on the storagenode
		testData := testrand.Bytes(8 * memory.KiB)
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		// after uploading, set orders store to have
		// gracePeriod=-1hr and maxInFlightTime=-1hr
		// so that we can immediately attempt to send orders
		node.OrdersStore.TestSetSettleBuffer(-time.Hour, -time.Hour)

		toSend, err := node.OrdersStore.ListUnsentBySatellite()
		require.NoError(t, err)
		require.Len(t, toSend, 1)
		ordersForSat := toSend[satellite.ID()]
		require.Len(t, ordersForSat.InfoList, 1)

		// trigger order send
		service.Sender.TriggerWait()

		toSend, err = node.OrdersStore.ListUnsentBySatellite()
		require.NoError(t, err)
		require.Len(t, toSend, 0)

		archived, err := node.OrdersStore.ListArchived()
		require.NoError(t, err)
		require.Len(t, archived, 1)
	})
}

// TODO remove when db is removed.
// TestOrderFileStoreAndDBSettle ensures that if orders exist in both DB and filestore, that the DB orders are settled first.
func TestOrderFileStoreAndDBSettle(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			StorageNode: func(index int, config *storagenode.Config) {
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]
		satellite.Audit.Worker.Loop.Pause()
		node := planet.StorageNodes[0]
		service := node.Storage2.Orders
		service.Sender.Pause()
		service.Cleanup.Pause()

		// add orders to orders DB
		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		bucketID := []byte(storj.JoinPaths(projects[0].ID.String(), "testbucket"))
		_, orderLimits, piecePrivateKey, err := satellite.Orders.Service.CreatePutOrderLimits(
			ctx,
			bucketID,
			[]*overlay.SelectedNode{
				{ID: node.ID(), LastIPPort: "fake", Address: new(pb.NodeAddress)},
			},
			time.Now().Add(2*time.Hour),
			2000,
		)
		require.NoError(t, err)
		require.Len(t, orderLimits, 1)

		orderLimit := orderLimits[0].Limit
		order := &pb.Order{
			SerialNumber: orderLimit.SerialNumber,
			Amount:       1000,
		}
		signedOrder, err := signing.SignUplinkOrder(ctx, piecePrivateKey, order)
		require.NoError(t, err)
		order0 := &orders.Info{
			Limit: orderLimit,
			Order: signedOrder,
		}

		// enter orders into unsent_orders
		err = node.DB.Orders().Enqueue(ctx, order0)
		require.NoError(t, err)

		toSendDB, err := node.DB.Orders().ListUnsent(ctx, 10)
		require.NoError(t, err)
		require.Len(t, toSendDB, 1)

		// upload a file to add orders to filestore
		testData := testrand.Bytes(8 * memory.KiB)
		err = uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		// after uploading, set orders store to have
		// gracePeriod=-1hr and maxInFlightTime=-1hr
		// so that we can immediately attempt to send orders
		node.OrdersStore.TestSetSettleBuffer(-time.Hour, -time.Hour)

		toSendFileStore, err := node.OrdersStore.ListUnsentBySatellite()
		require.NoError(t, err)
		require.Len(t, toSendFileStore, 1)
		ordersForSat := toSendFileStore[satellite.ID()]
		require.Len(t, ordersForSat.InfoList, 1)

		// trigger order send
		service.Sender.TriggerWait()

		// DB orders should be archived, but filestore orders should still be unsent.
		toSendDB, err = node.DB.Orders().ListUnsent(ctx, 10)
		require.NoError(t, err)
		require.Len(t, toSendDB, 0)

		archived, err := node.DB.Orders().ListArchived(ctx, 10)
		require.NoError(t, err)
		require.Len(t, archived, 1)

		toSendFileStore, err = node.OrdersStore.ListUnsentBySatellite()
		require.NoError(t, err)
		require.Len(t, toSendFileStore, 1)
		ordersForSat = toSendFileStore[satellite.ID()]
		require.Len(t, ordersForSat.InfoList, 1)

		// trigger order send again
		service.Sender.TriggerWait()

		// now FileStore orders should be archived too.
		toSendFileStore, err = node.OrdersStore.ListUnsentBySatellite()
		require.NoError(t, err)
		require.Len(t, toSendFileStore, 0)

		archived, err = node.OrdersStore.ListArchived()
		require.NoError(t, err)
		require.Len(t, archived, 1)
	})
}

// TODO remove when db is removed.
func TestCleanArchiveDB(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			StorageNode: func(index int, config *storagenode.Config) {
				config.Storage2.Orders.ArchiveTTL = 12 * time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Audit.Worker.Loop.Pause()
		satellite := planet.Satellites[0].ID()
		node := planet.StorageNodes[0]
		service := node.Storage2.Orders
		service.Sender.Pause()
		service.Cleanup.Pause()

		serialNumber0 := testrand.SerialNumber()
		serialNumber1 := testrand.SerialNumber()

		order0 := &orders.Info{
			Limit: &pb.OrderLimit{
				SatelliteId:  satellite,
				SerialNumber: serialNumber0,
			},
			Order: &pb.Order{},
		}
		order1 := &orders.Info{
			Limit: &pb.OrderLimit{
				SatelliteId:  satellite,
				SerialNumber: serialNumber1,
			},
			Order: &pb.Order{},
		}

		// enter orders into unsent_orders
		err := node.DB.Orders().Enqueue(ctx, order0)
		require.NoError(t, err)
		err = node.DB.Orders().Enqueue(ctx, order1)
		require.NoError(t, err)

		yesterday := time.Now().Add(-24 * time.Hour)
		now := time.Now()

		// archive one order yesterday, one today
		err = node.DB.Orders().Archive(ctx, yesterday, orders.ArchiveRequest{
			Satellite: satellite,
			Serial:    serialNumber0,
			Status:    orders.StatusAccepted,
		})
		require.NoError(t, err)

		err = node.DB.Orders().Archive(ctx, now, orders.ArchiveRequest{
			Satellite: satellite,
			Serial:    serialNumber1,
			Status:    orders.StatusAccepted,
		})
		require.NoError(t, err)

		// trigger cleanup
		service.Cleanup.TriggerWait()

		archived, err := node.DB.Orders().ListArchived(ctx, 10)
		require.NoError(t, err)

		require.Len(t, archived, 1)
		require.Equal(t, archived[0].Limit.SerialNumber, serialNumber1)
	})
}

func TestCleanArchiveFileStore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			StorageNode: func(index int, config *storagenode.Config) {
				config.Storage2.Orders.ArchiveTTL = 12 * time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Audit.Worker.Loop.Pause()
		satellite := planet.Satellites[0].ID()
		node := planet.StorageNodes[0]
		service := node.Storage2.Orders
		service.Sender.Pause()
		service.Cleanup.Pause()

		serialNumber0 := testrand.SerialNumber()
		createdAt0 := time.Now()
		serialNumber1 := testrand.SerialNumber()
		createdAt1 := time.Now().Add(-24 * time.Hour)

		order0 := &orders.Info{
			Limit: &pb.OrderLimit{
				SatelliteId:   satellite,
				SerialNumber:  serialNumber0,
				OrderCreation: createdAt0,
			},
			Order: &pb.Order{},
		}
		order1 := &orders.Info{
			Limit: &pb.OrderLimit{
				SatelliteId:   satellite,
				SerialNumber:  serialNumber1,
				OrderCreation: createdAt1,
			},
			Order: &pb.Order{},
		}

		// enqueue both orders; they will be placed in separate buckets because they have different creation hours
		// change settle buffer so that day-old order can be queued
		node.OrdersStore.TestSetSettleBuffer(24*time.Hour, 24*time.Hour)
		err := node.OrdersStore.Enqueue(order0)
		require.NoError(t, err)
		err = node.OrdersStore.Enqueue(order1)
		require.NoError(t, err)

		yesterday := time.Now().Add(-24 * time.Hour)
		now := time.Now()

		// archive one order yesterday, one today
		// change settle buffer so that new order can be archived
		node.OrdersStore.TestSetSettleBuffer(-1*time.Hour, -1*time.Hour)
		err = node.OrdersStore.Archive(satellite, createdAt0.Truncate(time.Hour), yesterday, pb.SettlementWithWindowResponse_ACCEPTED)
		require.NoError(t, err)
		err = node.OrdersStore.Archive(satellite, createdAt1.Truncate(time.Hour), now, pb.SettlementWithWindowResponse_ACCEPTED)
		require.NoError(t, err)

		archived, err := node.OrdersStore.ListArchived()
		require.NoError(t, err)
		require.Len(t, archived, 2)

		// trigger cleanup
		service.Cleanup.TriggerWait()

		archived, err = node.OrdersStore.ListArchived()
		require.NoError(t, err)

		require.Len(t, archived, 1)
		require.Equal(t, archived[0].Limit.SerialNumber, serialNumber1)
	})
}
