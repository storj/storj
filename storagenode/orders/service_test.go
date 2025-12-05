// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/orders/ordersfile"
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

		bucketname := "testbucket"
		err := planet.Uplinks[0].TestingCreateBucket(ctx, satellite, bucketname)
		require.NoError(t, err)

		_, orderLimits, piecePrivateKey, err := satellite.Orders.Service.CreatePutOrderLimits(
			ctx,
			metabase.BucketLocation{ProjectID: planet.Uplinks[0].Projects[0].ID, BucketName: metabase.BucketName(bucketname)},
			[]*nodeselection.SelectedNode{
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
		order0 := &ordersfile.Info{
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

		// in phase3 the orders are only sent from the filestore
		// so we expect any orders in ordersDB will remain there
		toSend, err = node.DB.Orders().ListUnsent(ctx, 10)
		require.NoError(t, err)
		require.Len(t, toSend, 1)

		archived, err := node.DB.Orders().ListArchived(ctx, 10)
		require.NoError(t, err)
		require.Len(t, archived, 0)
	})
}

func TestOrderFileStoreSettle(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]
		satellite.Audit.Worker.Loop.Pause()
		node := planet.StorageNodes[0]
		service := node.Storage2.Orders
		service.Sender.Pause()
		service.Cleanup.Pause()
		tomorrow := time.Now().Add(24 * time.Hour)

		// upload a file to generate an order on the storagenode
		testData := testrand.Bytes(8 * memory.KiB)
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

		toSend, err := node.OrdersStore.ListUnsentBySatellite(ctx, tomorrow)
		require.NoError(t, err)
		require.Len(t, toSend, 1)
		ordersForSat := toSend[satellite.ID()]
		require.Len(t, ordersForSat.InfoList, 1)

		// trigger order send
		service.SendOrders(ctx, tomorrow)

		toSend, err = node.OrdersStore.ListUnsentBySatellite(ctx, tomorrow)
		require.NoError(t, err)
		require.Len(t, toSend, 0)

		archived, err := node.OrdersStore.ListArchived()
		require.NoError(t, err)
		require.Len(t, archived, 1)
	})
}

func TestOrderFileStoreSettle_UntrustedSatellite(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		satellite2 := planet.Satellites[1]
		uplinkPeer := planet.Uplinks[0]
		satellite.Audit.Worker.Loop.Pause()
		node := planet.StorageNodes[0]
		service := node.Storage2.Orders
		service.Sender.Pause()
		service.Cleanup.Pause()
		tomorrow := time.Now().Add(24 * time.Hour)

		// upload a file to generate an order on the storagenode
		testData := testrand.Bytes(8 * memory.KiB)
		require.NoError(t, uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData))
		testData2 := testrand.Bytes(8 * memory.KiB)
		require.NoError(t, uplinkPeer.Upload(ctx, satellite2, "testbucket", "test/path", testData2))

		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

		// mark satellite2 as untrusted
		require.NoError(t, node.Storage2.Trust.DeleteSatellite(ctx, satellite2.ID()))

		toSend, err := node.OrdersStore.ListUnsentBySatellite(ctx, tomorrow)
		require.NoError(t, err)
		require.Len(t, toSend, 2)
		ordersForSat := toSend[satellite.ID()]
		require.Len(t, ordersForSat.InfoList, 1)
		ordersForSat2 := toSend[satellite2.ID()]
		require.Len(t, ordersForSat2.InfoList, 1)

		// create new observed logger
		observedZapCore, observedLogs := observer.New(zap.DebugLevel)
		observedLogger := zap.New(observedZapCore).Named("orders")
		service.TestSetLogger(observedLogger)
		// trigger order send
		service.SendOrders(ctx, tomorrow)

		// check that the untrusted satellite was skipped
		require.NotZero(t, observedLogs.All())
		skipLogs := observedLogs.FilterMessage("skipping order settlement for untrusted satellite. Order will be archived").All()
		require.Len(t, skipLogs, 1)
		logFields := observedLogs.FilterField(zap.String("satellite ID", satellite2.ID().String())).All()
		require.Len(t, logFields, 1)

		toSend, err = node.OrdersStore.ListUnsentBySatellite(ctx, tomorrow)
		require.NoError(t, err)
		require.Len(t, toSend, 0)

		archived, err := node.OrdersStore.ListArchived()
		require.NoError(t, err)
		require.Len(t, archived, 2)
	})
}

// TODO remove when db is removed.
// TestOrderFileStoreAndDBSettle ensures that if orders exist in both DB and filestore, that the DB orders and filestore are both settled.
func TestOrderFileStoreAndDBSettle(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]
		satellite.Audit.Worker.Loop.Pause()
		node := planet.StorageNodes[0]
		service := node.Storage2.Orders
		service.Sender.Pause()
		service.Cleanup.Pause()
		tomorrow := time.Now().Add(24 * time.Hour)

		bucketname := "testbucket"
		err := uplinkPeer.TestingCreateBucket(ctx, satellite, bucketname)
		require.NoError(t, err)

		// add orders to orders DB
		_, orderLimits, piecePrivateKey, err := satellite.Orders.Service.CreatePutOrderLimits(
			ctx,
			metabase.BucketLocation{ProjectID: uplinkPeer.Projects[0].ID, BucketName: metabase.BucketName(bucketname)},
			[]*nodeselection.SelectedNode{
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
		order0 := &ordersfile.Info{
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

		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

		toSendFileStore, err := node.OrdersStore.ListUnsentBySatellite(ctx, tomorrow)
		require.NoError(t, err)
		require.Len(t, toSendFileStore, 1)
		ordersForSat := toSendFileStore[satellite.ID()]
		require.Len(t, ordersForSat.InfoList, 1)

		// trigger order send
		service.SendOrders(ctx, tomorrow)

		// DB should not be archived in phase3, but and filestore orders should be archived.
		toSendDB, err = node.DB.Orders().ListUnsent(ctx, 10)
		require.NoError(t, err)
		require.Len(t, toSendDB, 1)

		archived, err := node.DB.Orders().ListArchived(ctx, 10)
		require.NoError(t, err)
		require.Len(t, archived, 0)

		toSendFileStore, err = node.OrdersStore.ListUnsentBySatellite(ctx, tomorrow)
		require.NoError(t, err)
		require.Len(t, toSendFileStore, 0)
		filestoreArchived, err := node.OrdersStore.ListArchived()
		require.NoError(t, err)
		require.Len(t, filestoreArchived, 1)
	})
}

func TestCleanArchiveFileStore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			StorageNode: func(_ int, config *storagenode.Config) {
				// A large grace period so we can write to multiple buckets at once
				config.Storage2.OrderLimitGracePeriod = 48 * time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Audit.Worker.Loop.Pause()
		satellite := planet.Satellites[0].ID()
		node := planet.StorageNodes[0]
		service := node.Storage2.Orders
		service.Sender.Pause()
		service.Cleanup.Pause()
		now := time.Now()
		yesterday := now.Add(-24 * time.Hour)

		serialNumber0 := testrand.SerialNumber()
		createdAt0 := now
		serialNumber1 := testrand.SerialNumber()
		createdAt1 := now.Add(-24 * time.Hour)

		order0 := &ordersfile.Info{
			Limit: &pb.OrderLimit{
				SatelliteId:   satellite,
				SerialNumber:  serialNumber0,
				OrderCreation: createdAt0,
			},
			Order: &pb.Order{},
		}
		order1 := &ordersfile.Info{
			Limit: &pb.OrderLimit{
				SatelliteId:   satellite,
				SerialNumber:  serialNumber1,
				OrderCreation: createdAt1,
			},
			Order: &pb.Order{},
		}

		// enqueue both orders; they will be placed in separate buckets because they have different creation hours
		err := node.OrdersStore.Enqueue(order0)
		require.NoError(t, err)
		err = node.OrdersStore.Enqueue(order1)
		require.NoError(t, err)

		// archive one order yesterday, one today
		unsentInfo := orders.UnsentInfo{Version: ordersfile.V1}
		unsentInfo.CreatedAtHour = createdAt0.Truncate(time.Hour)
		err = node.OrdersStore.Archive(satellite, unsentInfo, yesterday, pb.SettlementWithWindowResponse_ACCEPTED)
		require.NoError(t, err)
		unsentInfo.CreatedAtHour = createdAt1.Truncate(time.Hour)
		err = node.OrdersStore.Archive(satellite, unsentInfo, now, pb.SettlementWithWindowResponse_ACCEPTED)
		require.NoError(t, err)

		archived, err := node.OrdersStore.ListArchived()
		require.NoError(t, err)
		require.Len(t, archived, 2)

		// trigger cleanup of archived orders older than 12 hours
		require.NoError(t, service.CleanArchive(ctx, now.Add(-12*time.Hour)))

		archived, err = node.OrdersStore.ListArchived()
		require.NoError(t, err)

		require.Len(t, archived, 1)
		require.Equal(t, archived[0].Limit.SerialNumber, serialNumber1)
	})
}
