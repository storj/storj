// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/orders"
)

func runTestWithPhases(t *testing.T, fn func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet)) {
	run := func(phase orders.WindowEndpointRolloutPhase) func(t *testing.T) {
		return func(t *testing.T) {
			testplanet.Run(t, testplanet.Config{
				SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
				Reconfigure: testplanet.Reconfigure{
					Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
						config.Orders.WindowEndpointRolloutPhase = phase
					},
				},
			}, fn)
		}
	}

	t.Run("Phase1", run(orders.WindowEndpointRolloutPhase1))
	t.Run("Phase2", run(orders.WindowEndpointRolloutPhase2))
	t.Run("Phase3", run(orders.WindowEndpointRolloutPhase3))
}

func TestSettlementWithWindowEndpointManyOrders(t *testing.T) {
	runTestWithPhases(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ordersDB := satellite.Orders.DB
		storagenode := planet.StorageNodes[0]
		now := time.Now()
		projectID := testrand.UUID()
		bucketname := "testbucket"
		bucketID := storj.JoinPaths(projectID.String(), bucketname)

		// stop any async flushes because we want to be sure when some values are
		// written to avoid races
		satellite.Orders.Chore.Loop.Pause()
		satellite.Accounting.ReportedRollup.Loop.Pause()

		// confirm storagenode and bucket bandwidth tables start empty
		snbw, err := ordersDB.GetStorageNodeBandwidth(ctx, satellite.ID(), time.Time{}, now)
		require.NoError(t, err)
		require.Equal(t, int64(0), snbw)
		bucketbw, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
		require.NoError(t, err)
		require.Equal(t, int64(0), bucketbw)

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
			func() {
				// create serial number to use in test. must be unique for each run.
				serialNumber1 := testrand.SerialNumber()
				err = ordersDB.CreateSerialInfo(ctx, serialNumber1, []byte(bucketID), now.AddDate(1, 0, 10))
				require.NoError(t, err)

				serialNumber2 := testrand.SerialNumber()
				err = ordersDB.CreateSerialInfo(ctx, serialNumber2, []byte(bucketID), now.AddDate(1, 0, 10))
				require.NoError(t, err)

				piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
				require.NoError(t, err)

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
				defer ctx.Check(conn.Close)

				stream, err := pb.NewDRPCOrdersClient(conn).SettlementWithWindow(ctx)
				require.NoError(t, err)
				defer ctx.Check(stream.Close)

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

				// the settled amount is only returned during phase3
				var settled map[int32]int64
				if satellite.Config.Orders.WindowEndpointRolloutPhase == orders.WindowEndpointRolloutPhase3 {
					settled = map[int32]int64{int32(pb.PieceAction_PUT): tt.settledAmt}
				}
				require.Equal(t, &pb.SettlementWithWindowResponse{
					Status:        pb.SettlementWithWindowResponse_ACCEPTED,
					ActionSettled: settled,
				}, resp)

				// trigger and wait for all of the chores necessary to flush the orders
				assert.NoError(t, satellite.Accounting.ReportedRollup.RunOnce(ctx, tt.orderCreation))
				satellite.Orders.Chore.Loop.TriggerWait()

				// assert all the right stuff is in the satellite storagenode and bucket bandwidth tables
				snbw, err = ordersDB.GetStorageNodeBandwidth(ctx, storagenode.ID(), time.Time{}, tt.orderCreation)
				require.NoError(t, err)
				require.EqualValues(t, tt.settledAmt, snbw)

				newBbw, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, tt.orderCreation)
				require.NoError(t, err)
				require.EqualValues(t, tt.settledAmt, newBbw)
			}()
		}
	})
}
func TestSettlementWithWindowEndpointSingleOrder(t *testing.T) {
	runTestWithPhases(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const dataAmount int64 = 50
		satellite := planet.Satellites[0]
		ordersDB := satellite.Orders.DB
		storagenode := planet.StorageNodes[0]
		now := time.Now()
		projectID := testrand.UUID()
		bucketname := "testbucket"
		bucketID := storj.JoinPaths(projectID.String(), bucketname)

		// stop any async flushes because we want to be sure when some values are
		// written to avoid races
		satellite.Orders.Chore.Loop.Pause()
		satellite.Accounting.ReportedRollup.Loop.Pause()

		// confirm storagenode and bucket bandwidth tables start empty
		snbw, err := ordersDB.GetStorageNodeBandwidth(ctx, satellite.ID(), time.Time{}, now)
		require.NoError(t, err)
		require.EqualValues(t, 0, snbw)

		bucketbw, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
		require.NoError(t, err)
		require.EqualValues(t, 0, bucketbw)

		// create serial number to use in test
		serialNumber := testrand.SerialNumber()
		err = ordersDB.CreateSerialInfo(ctx, serialNumber, []byte(bucketID), now.AddDate(1, 0, 10))
		require.NoError(t, err)

		piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
		require.NoError(t, err)

		var testCases = []struct {
			name           string
			dataAmount     int64
			expectedStatus pb.SettlementWithWindowResponse_Status
		}{
			{"first settlement", dataAmount, pb.SettlementWithWindowResponse_ACCEPTED},
			{"settle the same a second time, matches first", dataAmount, pb.SettlementWithWindowResponse_ACCEPTED},
			{"settle a third time, doesn't match first", dataAmount + 1, pb.SettlementWithWindowResponse_REJECTED},
		}

		for _, tt := range testCases {
			func() {
				// create signed orderlimit or order to test with
				limit := &pb.OrderLimit{
					SerialNumber:    serialNumber,
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
					SerialNumber: serialNumber,
					Amount:       tt.dataAmount,
				})
				require.NoError(t, err)

				// create connection between storagenode and satellite
				conn, err := storagenode.Dialer.DialNodeURL(ctx, storj.NodeURL{ID: satellite.ID(), Address: satellite.Addr()})
				require.NoError(t, err)
				defer ctx.Check(conn.Close)

				stream, err := pb.NewDRPCOrdersClient(conn).SettlementWithWindow(ctx)
				require.NoError(t, err)
				defer ctx.Check(stream.Close)

				// storagenode settles an order and orderlimit
				err = stream.Send(&pb.SettlementRequest{
					Limit: orderLimit,
					Order: order,
				})
				require.NoError(t, err)
				resp, err := stream.CloseAndRecv()
				require.NoError(t, err)

				expected := new(pb.SettlementWithWindowResponse)
				switch {
				case satellite.Config.Orders.WindowEndpointRolloutPhase != orders.WindowEndpointRolloutPhase3:
					expected.Status = pb.SettlementWithWindowResponse_ACCEPTED
					expected.ActionSettled = nil
				case tt.expectedStatus == pb.SettlementWithWindowResponse_ACCEPTED:
					expected.Status = pb.SettlementWithWindowResponse_ACCEPTED
					expected.ActionSettled = map[int32]int64{int32(pb.PieceAction_PUT): tt.dataAmount}
				default:
					expected.Status = pb.SettlementWithWindowResponse_REJECTED
					expected.ActionSettled = nil
				}
				require.Equal(t, expected, resp)

				// flush all the chores
				assert.NoError(t, satellite.Accounting.ReportedRollup.RunOnce(ctx, now))
				satellite.Orders.Chore.Loop.TriggerWait()

				// assert all the right stuff is in the satellite storagenode and bucket bandwidth tables
				snbw, err = ordersDB.GetStorageNodeBandwidth(ctx, storagenode.ID(), time.Time{}, now)
				require.NoError(t, err)
				require.Equal(t, dataAmount, snbw)

				newBbw, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
				require.NoError(t, err)
				require.Equal(t, dataAmount, newBbw)
			}()
		}
	})
}

func TestSettlementWithWindowEndpointErrors(t *testing.T) {
	runTestWithPhases(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ordersDB := satellite.Orders.DB
		storagenode := planet.StorageNodes[0]
		now := time.Now()
		projectID := testrand.UUID()
		bucketname := "testbucket"
		bucketID := storj.JoinPaths(projectID.String(), bucketname)

		// stop any async flushes because we want to be sure when some values are
		// written to avoid races
		satellite.Orders.Chore.Loop.Pause()
		satellite.Accounting.ReportedRollup.Loop.Pause()

		// confirm storagenode and bucket bandwidth tables start empty
		snbw, err := ordersDB.GetStorageNodeBandwidth(ctx, satellite.ID(), time.Time{}, now)
		require.NoError(t, err)
		require.EqualValues(t, 0, snbw)

		bucketbw, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
		require.NoError(t, err)
		require.EqualValues(t, 0, bucketbw)

		// create serial number to use in test
		serialNumber1 := testrand.SerialNumber()
		err = ordersDB.CreateSerialInfo(ctx, serialNumber1, []byte(bucketID), now.AddDate(1, 0, 10))
		require.NoError(t, err)

		serialNumber2 := testrand.SerialNumber()
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
				defer ctx.Check(conn.Close)

				stream, err := pb.NewDRPCOrdersClient(conn).SettlementWithWindow(ctx)
				require.NoError(t, err)
				defer ctx.Check(stream.Close)

				err = stream.Send(&pb.SettlementRequest{
					Limit: tt.orderLimit,
					Order: tt.order,
				})
				require.NoError(t, err)

				resp, err := stream.CloseAndRecv()
				require.NoError(t, err)
				require.Equal(t, &pb.SettlementWithWindowResponse{
					Status:        pb.SettlementWithWindowResponse_REJECTED,
					ActionSettled: nil,
				}, resp)

				// flush all the chores
				assert.NoError(t, satellite.Accounting.ReportedRollup.RunOnce(ctx, now))
				satellite.Orders.Chore.Loop.TriggerWait()

				// assert no data was added to satellite storagenode or bucket bandwidth tables
				snbw, err = ordersDB.GetStorageNodeBandwidth(ctx, storagenode.ID(), time.Time{}, now)
				require.NoError(t, err)
				require.EqualValues(t, 0, snbw)

				newBbw, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
				require.NoError(t, err)
				require.EqualValues(t, 0, newBbw)
			})
		}
	})
}

func TestSettlementEndpointSingleOrder(t *testing.T) {
	runTestWithPhases(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const dataAmount int64 = 50
		satellite := planet.Satellites[0]
		ordersDB := satellite.Orders.DB
		storagenode := planet.StorageNodes[0]
		now := time.Now()
		projectID := testrand.UUID()
		bucketname := "testbucket"
		bucketID := storj.JoinPaths(projectID.String(), bucketname)

		// stop any async flushes because we want to be sure when some values are
		// written to avoid races
		satellite.Orders.Chore.Loop.Pause()
		satellite.Accounting.ReportedRollup.Loop.Pause()

		// confirm storagenode and bucket bandwidth tables start empty
		snbw, err := ordersDB.GetStorageNodeBandwidth(ctx, satellite.ID(), time.Time{}, now)
		require.NoError(t, err)
		require.EqualValues(t, 0, snbw)

		bucketbw, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
		require.NoError(t, err)
		require.EqualValues(t, 0, bucketbw)

		// create serial number to use in test
		serialNumber := testrand.SerialNumber()
		err = ordersDB.CreateSerialInfo(ctx, serialNumber, []byte(bucketID), now.AddDate(1, 0, 10))
		require.NoError(t, err)

		piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
		require.NoError(t, err)

		// create signed orderlimit or order to test with
		limit := &pb.OrderLimit{
			SerialNumber:    serialNumber,
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
			SerialNumber: serialNumber,
			Amount:       dataAmount,
		})
		require.NoError(t, err)

		// create connection between storagenode and satellite
		conn, err := storagenode.Dialer.DialNodeURL(ctx, storj.NodeURL{ID: satellite.ID(), Address: satellite.Addr()})
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		stream, err := pb.NewDRPCOrdersClient(conn).Settlement(ctx)
		require.NoError(t, err)
		defer ctx.Check(stream.Close)

		// storagenode settles an order and orderlimit
		var resp *pb.SettlementResponse
		if satellite.Config.Orders.WindowEndpointRolloutPhase == orders.WindowEndpointRolloutPhase1 {
			err = stream.Send(&pb.SettlementRequest{
				Limit: orderLimit,
				Order: order,
			})
			require.NoError(t, err)
			require.NoError(t, stream.CloseSend())

			resp, err = stream.Recv()
			require.NoError(t, err)
		} else {
			// in phase2 and phase3, the endpoint is disabled. depending on how fast the
			// server sends that error message, we may see an io.EOF on the Send call, or
			// we may see no error at all. In either case, we have to call stream.Recv to
			// see the actual error. gRPC semantics are funky.
			err = stream.Send(&pb.SettlementRequest{
				Limit: orderLimit,
				Order: order,
			})
			if err != io.EOF {
				require.NoError(t, err)
			}
			require.NoError(t, stream.CloseSend())

			_, err = stream.Recv()
			require.Error(t, err)
			require.Equal(t, rpcstatus.Unavailable, rpcstatus.Code(err))
			return
		}

		require.Equal(t, &pb.SettlementResponse{
			SerialNumber: serialNumber,
			Status:       pb.SettlementResponse_ACCEPTED,
		}, resp)

		// flush all the chores
		assert.NoError(t, satellite.Accounting.ReportedRollup.RunOnce(ctx, now))
		satellite.Orders.Chore.Loop.TriggerWait()

		// assert all the right stuff is in the satellite storagenode and bucket bandwidth tables
		snbw, err = ordersDB.GetStorageNodeBandwidth(ctx, storagenode.ID(), time.Time{}, now)
		require.NoError(t, err)
		require.Equal(t, dataAmount, snbw)

		newBbw, err := ordersDB.GetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
		require.NoError(t, err)
		require.Equal(t, dataAmount, newBbw)
	})
}
