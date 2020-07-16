// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
)

func TestSettlementWithWindowEndpointManyOrders(t *testing.T) {
	t.Skip("endpoint currently disabled")

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ordersDB := satellite.Orders.DB
		storagenode := planet.StorageNodes[0]
		now := time.Now().UTC()
		projectID := testrand.UUID()
		bucketname := "testbucket"

		// stop any async flushes because we want to be sure when some values are
		// written to avoid races
		satellite.Orders.Chore.Loop.Pause()

		// confirm storagenode and bucket bandwidth tables start empty
		snbw, err := ordersDB.GetStorageNodeBandwidth(ctx, satellite.ID(), time.Time{}, now)
		require.NoError(t, err)
		require.Equal(t, int64(0), snbw)
		bucketbw, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
		require.NoError(t, err)
		require.Equal(t, int64(0), bucketbw)

		// create serial number to use in test
		serialNumber1 := testrand.SerialNumber()
		bucketID := storj.JoinPaths(projectID.String(), bucketname)
		err = ordersDB.CreateSerialInfo(ctx, serialNumber1, []byte(bucketID), now.AddDate(1, 0, 10))
		serialNumber2 := testrand.SerialNumber()
		require.NoError(t, err)
		err = ordersDB.CreateSerialInfo(ctx, serialNumber2, []byte(bucketID), now.AddDate(1, 0, 10))
		require.NoError(t, err)
		piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
		require.NoError(t, err)

		var testCases = []struct {
			name          string
			dataAmount    int64
			orderCreation time.Time
			settledAmt    int64
		}{
			{"settle 2 orders, valid", int64(50), now, int64(100)},
			{"settle 2 orders, window mismatch", int64(50), now.Add(-48 * time.Hour), int64(50)},
		}

		for _, tt := range testCases {
			// create signed orderlimit or order to test with
			limit1 := &pb.OrderLimit{
				SerialNumber:    serialNumber1,
				SatelliteId:     satellite.ID(),
				UplinkPublicKey: piecePublicKey,
				StorageNodeId:   storagenode.ID(),
				PieceId:         storj.NewPieceID(),
				Action:          pb.PieceAction_PUT,
				Limit:           1000,
				PieceExpiration: time.Time{},
				OrderCreation:   tt.orderCreation,
				OrderExpiration: now.Add(24 * time.Hour),
			}
			orderLimit1, err := signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satellite.Identity), limit1)
			require.NoError(t, err)
			order1, err := signing.SignUplinkOrder(ctx, piecePrivateKey, &pb.Order{
				SerialNumber: serialNumber1,
				Amount:       tt.dataAmount,
			})
			require.NoError(t, err)
			limit2 := &pb.OrderLimit{
				SerialNumber:    serialNumber2,
				SatelliteId:     satellite.ID(),
				UplinkPublicKey: piecePublicKey,
				StorageNodeId:   storagenode.ID(),
				PieceId:         storj.NewPieceID(),
				Action:          pb.PieceAction_PUT,
				Limit:           1000,
				PieceExpiration: time.Time{},
				OrderCreation:   now,
				OrderExpiration: now.Add(24 * time.Hour),
			}
			orderLimit2, err := signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satellite.Identity), limit2)
			require.NoError(t, err)
			order2, err := signing.SignUplinkOrder(ctx, piecePrivateKey, &pb.Order{
				SerialNumber: serialNumber2,
				Amount:       tt.dataAmount,
			})
			require.NoError(t, err)

			// create connection between storagenode and satellite
			conn, err := storagenode.Dialer.DialNodeURL(ctx, storj.NodeURL{ID: satellite.ID(), Address: satellite.Addr()})
			require.NoError(t, err)
			stream, err := pb.NewDRPCOrdersClient(conn).SettlementWithWindow(ctx)
			require.NoError(t, err)
			// storagenode settles an order and orderlimit
			err = stream.Send(&pb.SettlementRequest{
				Limit: orderLimit1,
				Order: order1,
			})
			require.NoError(t, err)
			err = stream.Send(&pb.SettlementRequest{
				Limit: orderLimit2,
				Order: order2,
			})
			require.NoError(t, err)
			resp, err := stream.CloseAndRecv()
			require.NoError(t, err)

			settled := map[int32]int64{int32(pb.PieceAction_PUT): tt.settledAmt}
			require.Equal(t, &pb.SettlementWithWindowResponse{Status: pb.SettlementWithWindowResponse_ACCEPTED, ActionSettled: settled}, resp)
			// assert all the right stuff is in the satellite storagenode and bucket bandwidth tables
			snbw, err = ordersDB.GetStorageNodeBandwidth(ctx, storagenode.ID(), time.Time{}, tt.orderCreation)
			require.NoError(t, err)
			require.EqualValues(t, tt.settledAmt, snbw)

			satellite.Orders.Chore.Loop.TriggerWait()
			newBbw, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, tt.orderCreation)
			require.NoError(t, err)
			require.EqualValues(t, tt.settledAmt, newBbw)
		}
	})
}
func TestSettlementWithWindowEndpointSingleOrder(t *testing.T) {
	t.Skip("endpoint currently disabled")

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ordersDB := satellite.Orders.DB
		storagenode := planet.StorageNodes[0]
		now := time.Now().UTC()
		projectID := testrand.UUID()
		bucketname := "testbucket"

		// stop any async flushes because we want to be sure when some values are
		// written to avoid races
		satellite.Orders.Chore.Loop.Pause()

		// confirm storagenode and bucket bandwidth tables start empty
		snbw, err := ordersDB.GetStorageNodeBandwidth(ctx, satellite.ID(), time.Time{}, now)
		require.NoError(t, err)
		require.EqualValues(t, 0, snbw)
		bucketbw, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
		require.NoError(t, err)
		require.EqualValues(t, 0, bucketbw)

		// create serial number to use in test
		serialNumber := testrand.SerialNumber()
		bucketID := storj.JoinPaths(projectID.String(), bucketname)
		err = ordersDB.CreateSerialInfo(ctx, serialNumber, []byte(bucketID), now.AddDate(1, 0, 10))
		require.NoError(t, err)
		piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
		require.NoError(t, err)

		var testCases = []struct {
			name           string
			serialNumber   storj.SerialNumber
			dataAmount     int64
			expectedStatus pb.SettlementWithWindowResponse_Status
		}{
			{"first settlement", serialNumber, int64(50), pb.SettlementWithWindowResponse_ACCEPTED},
			{"settle the same a second time, matches first", serialNumber, int64(50), pb.SettlementWithWindowResponse_ACCEPTED},
			{"settle a third time, doesn't match first", serialNumber, int64(0), pb.SettlementWithWindowResponse_REJECTED},
		}

		for _, tt := range testCases {
			// create signed orderlimit or order to test with
			limit := &pb.OrderLimit{
				SerialNumber:    tt.serialNumber,
				SatelliteId:     satellite.ID(),
				UplinkPublicKey: piecePublicKey,
				StorageNodeId:   storagenode.ID(),
				PieceId:         storj.NewPieceID(),
				Action:          pb.PieceAction_PUT,
				Limit:           1000,
				PieceExpiration: time.Time{},
				OrderCreation:   now,
				OrderExpiration: now.Add(24 * time.Hour),
			}
			orderLimit, err := signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satellite.Identity), limit)
			require.NoError(t, err)
			order, err := signing.SignUplinkOrder(ctx, piecePrivateKey, &pb.Order{
				SerialNumber: tt.serialNumber,
				Amount:       tt.dataAmount,
			})
			require.NoError(t, err)

			// create connection between storagenode and satellite
			conn, err := storagenode.Dialer.DialNodeURL(ctx, storj.NodeURL{ID: satellite.ID(), Address: satellite.Addr()})
			require.NoError(t, err)
			stream, err := pb.NewDRPCOrdersClient(conn).SettlementWithWindow(ctx)
			require.NoError(t, err)
			// storagenode settles an order and orderlimit
			err = stream.Send(&pb.SettlementRequest{
				Limit: orderLimit,
				Order: order,
			})
			require.NoError(t, err)
			resp, err := stream.CloseAndRecv()
			require.NoError(t, err)
			settled := map[int32]int64{int32(pb.PieceAction_PUT): tt.dataAmount}
			if tt.expectedStatus == pb.SettlementWithWindowResponse_REJECTED {
				require.Equal(t, &pb.SettlementWithWindowResponse{Status: tt.expectedStatus, ActionSettled: nil}, resp)
			} else {
				require.Equal(t, &pb.SettlementWithWindowResponse{Status: tt.expectedStatus, ActionSettled: settled}, resp)
			}
			// assert all the right stuff is in the satellite storagenode and bucket bandwidth tables
			snbw, err = ordersDB.GetStorageNodeBandwidth(ctx, storagenode.ID(), time.Time{}, time.Now().UTC())
			require.NoError(t, err)
			if tt.expectedStatus == pb.SettlementWithWindowResponse_REJECTED {
				require.NotEqual(t, tt.dataAmount, snbw)
			} else {
				require.Equal(t, tt.dataAmount, snbw)
			}

			// wait for rollup_write_cache to flush, this on average takes 1ms to sleep to complete
			satellite.Orders.Chore.Loop.TriggerWait()
			newBbw, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, time.Now().UTC())
			require.NoError(t, err)
			if tt.expectedStatus == pb.SettlementWithWindowResponse_REJECTED {
				require.NotEqual(t, tt.dataAmount, newBbw)
			} else {
				require.Equal(t, tt.dataAmount, newBbw)
			}
		}
	})
}

func TestSettlementWithWindowEndpointErrors(t *testing.T) {
	t.Skip("endpoint currently disabled")

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ordersDB := satellite.Orders.DB
		storagenode := planet.StorageNodes[0]
		now := time.Now().UTC()
		projectID := testrand.UUID()
		bucketname := "testbucket"

		// stop any async flushes because we want to be sure when some values are
		// written to avoid races
		satellite.Orders.Chore.Loop.Pause()

		// confirm storagenode and bucket bandwidth tables start empty
		snbw, err := ordersDB.GetStorageNodeBandwidth(ctx, satellite.ID(), time.Time{}, now)
		require.NoError(t, err)
		require.EqualValues(t, 0, snbw)
		bucketbw, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
		require.NoError(t, err)
		require.EqualValues(t, 0, bucketbw)

		// create serial number to use in test
		serialNumber1 := testrand.SerialNumber()
		serialNumber2 := testrand.SerialNumber()
		bucketID := storj.JoinPaths(projectID.String(), bucketname)
		err = ordersDB.CreateSerialInfo(ctx, serialNumber1, []byte(bucketID), now.AddDate(1, 0, 10))
		require.NoError(t, err)
		err = ordersDB.CreateSerialInfo(ctx, serialNumber2, []byte(bucketID), now.AddDate(1, 0, 10))
		require.NoError(t, err)
		piecePublicKey1, piecePrivateKey1, err := storj.NewPieceKey()
		require.NoError(t, err)
		_, piecePrivateKey2, err := storj.NewPieceKey()
		require.NoError(t, err)
		limit := pb.OrderLimit{
			SerialNumber:    serialNumber1,
			SatelliteId:     satellite.ID(),
			UplinkPublicKey: piecePublicKey1,
			StorageNodeId:   storagenode.ID(),
			PieceId:         storj.NewPieceID(),
			Action:          pb.PieceAction_PUT,
			Limit:           1000,
			PieceExpiration: time.Time{},
			OrderCreation:   now,
			OrderExpiration: now.Add(24 * time.Hour),
		}
		orderLimit1, err := signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satellite.Identity), &limit)
		require.NoError(t, err)
		order1, err := signing.SignUplinkOrder(ctx, piecePrivateKey1, &pb.Order{
			SerialNumber: serialNumber1,
			Amount:       int64(50),
		})
		require.NoError(t, err)
		order2, err := signing.SignUplinkOrder(ctx, piecePrivateKey1, &pb.Order{
			SerialNumber: serialNumber2,
			Amount:       int64(50),
		})
		require.NoError(t, err)
		order3, err := signing.SignUplinkOrder(ctx, piecePrivateKey2, &pb.Order{
			SerialNumber: serialNumber2,
			Amount:       int64(50),
		})
		require.NoError(t, err)

		var testCases = []struct {
			name       string
			order      *pb.Order
			orderLimit *pb.OrderLimit
		}{
			{"no order", nil, orderLimit1},
			{"no order limit", order1, nil},
			{"mismatch serial number", order2, orderLimit1},
			{"mismatch uplink signature", order3, orderLimit1},
		}

		for _, tt := range testCases {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				conn, err := storagenode.Dialer.DialNodeURL(ctx, storj.NodeURL{ID: satellite.ID(), Address: satellite.Addr()})
				require.NoError(t, err)
				stream, err := pb.NewDRPCOrdersClient(conn).SettlementWithWindow(ctx)
				require.NoError(t, err)
				err = stream.Send(&pb.SettlementRequest{
					Limit: tt.orderLimit,
					Order: tt.order,
				})
				require.NoError(t, err)
				resp, err := stream.CloseAndRecv()
				require.NoError(t, err)
				require.Equal(t, &pb.SettlementWithWindowResponse{Status: pb.SettlementWithWindowResponse_REJECTED, ActionSettled: nil}, resp)

				// assert no data was added to satellite storagenode or bucket bandwidth tables
				snbw, err = ordersDB.GetStorageNodeBandwidth(ctx, storagenode.ID(), time.Time{}, time.Now().UTC())
				require.NoError(t, err)
				require.EqualValues(t, 0, snbw)

				// wait for rollup_write_cache to flush
				satellite.Orders.Chore.Loop.TriggerWait()
				newBbw, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, time.Now().UTC())
				require.NoError(t, err)
				require.EqualValues(t, 0, newBbw)
			})
		}
	})
}
