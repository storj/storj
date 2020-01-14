// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/orders"
)

func TestCleanArchive(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			StorageNode: func(index int, config *storagenode.Config) {
				config.Storage2.Orders.ArchiveTTL = 24 * time.Hour
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

		yesterday := time.Now().UTC().Add(-24 * time.Hour)
		now := time.Now().UTC()

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
