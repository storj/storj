// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
)

func TestSettlementWithWindowEndpointManyOrders(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ordersDB := satellite.Orders.DB
		storagenode := planet.StorageNodes[0]
		now := time.Now()
		projectID := testrand.UUID()
		bucketname := metabase.BucketName("testbucket")
		bucketLocation := metabase.BucketLocation{
			ProjectID:  projectID,
			BucketName: bucketname,
		}
		key := satellite.Config.Orders.EncryptionKeys.Default

		// stop the async flush because we want to be sure when some values are
		// written to avoid races
		satellite.Orders.Chore.Loop.Pause()

		// confirm storagenode and bucket bandwidth tables start empty
		snbw, err := ordersDB.GetStorageNodeBandwidth(ctx, satellite.ID(), time.Time{}, now)
		require.NoError(t, err)
		require.Equal(t, int64(0), snbw)
		_, _, bucketbw, err := ordersDB.TestGetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
		require.NoError(t, err)
		require.Equal(t, int64(0), bucketbw)

		testCases := []struct {
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
				encrypted1, err := key.EncryptMetadata(
					serialNumber1,
					&internalpb.OrderLimitMetadata{
						CompactProjectBucketPrefix: bucketLocation.CompactPrefix(),
					},
				)
				require.NoError(t, err)

				serialNumber2 := testrand.SerialNumber()
				encrypted2, err := key.EncryptMetadata(
					serialNumber2,
					&internalpb.OrderLimitMetadata{
						CompactProjectBucketPrefix: bucketLocation.CompactPrefix(),
					},
				)
				require.NoError(t, err)

				piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
				require.NoError(t, err)

				// create signed orderlimit or order to test with
				limit1 := &pb.OrderLimit{
					SerialNumber:           serialNumber1,
					SatelliteId:            satellite.ID(),
					UplinkPublicKey:        piecePublicKey,
					StorageNodeId:          storagenode.ID(),
					PieceId:                storj.NewPieceID(),
					Action:                 pb.PieceAction_PUT,
					Limit:                  1000,
					PieceExpiration:        time.Time{},
					OrderCreation:          tt.orderCreation,
					OrderExpiration:        now.Add(24 * time.Hour),
					EncryptedMetadataKeyId: key.ID[:],
					EncryptedMetadata:      encrypted1,
				}
				orderLimit1, err := signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satellite.Identity), limit1)
				require.NoError(t, err)

				order1, err := signing.SignUplinkOrder(ctx, piecePrivateKey, &pb.Order{
					SerialNumber: serialNumber1,
					Amount:       tt.dataAmount,
				})
				require.NoError(t, err)

				limit2 := &pb.OrderLimit{
					SerialNumber:           serialNumber2,
					SatelliteId:            satellite.ID(),
					UplinkPublicKey:        piecePublicKey,
					StorageNodeId:          storagenode.ID(),
					PieceId:                storj.NewPieceID(),
					Action:                 pb.PieceAction_PUT,
					Limit:                  1000,
					PieceExpiration:        time.Time{},
					OrderCreation:          now,
					OrderExpiration:        now.Add(24 * time.Hour),
					EncryptedMetadataKeyId: key.ID[:],
					EncryptedMetadata:      encrypted2,
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

				settled := map[int32]int64{int32(pb.PieceAction_PUT): tt.settledAmt}
				require.Equal(t, &pb.SettlementWithWindowResponse{
					Status:        pb.SettlementWithWindowResponse_ACCEPTED,
					ActionSettled: settled,
				}, resp)

				satellite.Orders.Chore.Loop.TriggerWait()

				// assert all the right stuff is in the satellite storagenode and bucket bandwidth tables
				snbw, err = ordersDB.GetStorageNodeBandwidth(ctx, storagenode.ID(), time.Time{}, tt.orderCreation)
				require.NoError(t, err)
				require.EqualValues(t, tt.settledAmt, snbw)

				_, _, newBbw, err := ordersDB.TestGetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, tt.orderCreation)
				require.NoError(t, err)
				require.EqualValues(t, tt.settledAmt, newBbw)
			}()
		}
	})
}

func TestSettlementWithWindowEndpointSingleOrder(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const dataAmount int64 = 50
		satellite := planet.Satellites[0]
		ordersDB := satellite.Orders.DB
		storagenode := planet.StorageNodes[0]
		now := time.Now()
		projectID := testrand.UUID()
		bucketname := metabase.BucketName("testbucket")
		bucketLocation := metabase.BucketLocation{
			ProjectID:  projectID,
			BucketName: bucketname,
		}
		key := satellite.Config.Orders.EncryptionKeys.Default

		// stop the async flush because we want to be sure when some values are
		// written to avoid races
		satellite.Orders.Chore.Loop.Pause()

		// confirm storagenode and bucket bandwidth tables start empty
		snbw, err := ordersDB.GetStorageNodeBandwidth(ctx, satellite.ID(), time.Time{}, now)
		require.NoError(t, err)
		require.EqualValues(t, 0, snbw)

		_, _, bucketbw, err := ordersDB.TestGetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
		require.NoError(t, err)
		require.EqualValues(t, 0, bucketbw)

		// create serial number to use in test
		serialNumber := testrand.SerialNumber()
		encrypted, err := key.EncryptMetadata(
			serialNumber,
			&internalpb.OrderLimitMetadata{
				CompactProjectBucketPrefix: bucketLocation.CompactPrefix(),
			},
		)
		require.NoError(t, err)

		piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
		require.NoError(t, err)

		testCases := []struct {
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
					SerialNumber:           serialNumber,
					SatelliteId:            satellite.ID(),
					UplinkPublicKey:        piecePublicKey,
					StorageNodeId:          storagenode.ID(),
					PieceId:                storj.NewPieceID(),
					Action:                 pb.PieceAction_PUT,
					Limit:                  1000,
					PieceExpiration:        time.Time{},
					OrderCreation:          now,
					OrderExpiration:        now.Add(24 * time.Hour),
					EncryptedMetadataKeyId: key.ID[:],
					EncryptedMetadata:      encrypted,
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
				case tt.expectedStatus == pb.SettlementWithWindowResponse_ACCEPTED:
					expected.Status = pb.SettlementWithWindowResponse_ACCEPTED
					expected.ActionSettled = map[int32]int64{int32(pb.PieceAction_PUT): tt.dataAmount}
				default:
					expected.Status = pb.SettlementWithWindowResponse_REJECTED
					expected.ActionSettled = nil
				}
				require.Equal(t, expected, resp)

				// flush the chores
				satellite.Orders.Chore.Loop.TriggerWait()

				// assert all the right stuff is in the satellite storagenode and bucket bandwidth tables
				snbw, err = ordersDB.GetStorageNodeBandwidth(ctx, storagenode.ID(), time.Time{}, now)
				require.NoError(t, err)
				require.Equal(t, dataAmount, snbw)

				_, _, newBbw, err := ordersDB.TestGetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
				require.NoError(t, err)
				require.Equal(t, dataAmount, newBbw)
			}()
		}
	})
}

func TestSettlementWithWindowEndpointErrors(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ordersDB := satellite.Orders.DB
		storagenode := planet.StorageNodes[0]
		now := time.Now()
		projectID := testrand.UUID()
		bucketname := metabase.BucketName("testbucket")
		bucketLocation := metabase.BucketLocation{
			ProjectID:  projectID,
			BucketName: bucketname,
		}

		// stop the async flush because we want to be sure when some values are
		// written to avoid races
		satellite.Orders.Chore.Loop.Pause()

		// confirm storagenode and bucket bandwidth tables start empty
		snbw, err := ordersDB.GetStorageNodeBandwidth(ctx, satellite.ID(), time.Time{}, now)
		require.NoError(t, err)
		require.EqualValues(t, 0, snbw)

		_, _, bucketbw, err := ordersDB.TestGetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
		require.NoError(t, err)
		require.EqualValues(t, 0, bucketbw)

		piecePublicKey1, piecePrivateKey1, err := storj.NewPieceKey()
		require.NoError(t, err)

		_, piecePrivateKey2, err := storj.NewPieceKey()
		require.NoError(t, err)

		serialNumber1 := testrand.SerialNumber()
		key := satellite.Config.Orders.EncryptionKeys.Default
		encrypted, err := key.EncryptMetadata(
			serialNumber1,
			&internalpb.OrderLimitMetadata{
				CompactProjectBucketPrefix: bucketLocation.CompactPrefix(),
			},
		)
		require.NoError(t, err)

		limit := pb.OrderLimit{
			SerialNumber:           serialNumber1,
			SatelliteId:            satellite.ID(),
			UplinkPublicKey:        piecePublicKey1,
			StorageNodeId:          storagenode.ID(),
			PieceId:                storj.NewPieceID(),
			Action:                 pb.PieceAction_PUT,
			Limit:                  1000,
			PieceExpiration:        time.Time{},
			OrderCreation:          now,
			OrderExpiration:        now.Add(24 * time.Hour),
			EncryptedMetadataKeyId: key.ID[:],
			EncryptedMetadata:      encrypted,
		}
		orderLimit1, err := signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satellite.Identity), &limit)
		require.NoError(t, err)

		order1, err := signing.SignUplinkOrder(ctx, piecePrivateKey1, &pb.Order{
			SerialNumber: serialNumber1,
			Amount:       int64(50),
		})
		require.NoError(t, err)

		serialNumber2 := testrand.SerialNumber()
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

		testCases := []struct {
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

				// flush the chores
				satellite.Orders.Chore.Loop.TriggerWait()

				// assert no data was added to satellite storagenode or bucket bandwidth tables
				snbw, err = ordersDB.GetStorageNodeBandwidth(ctx, storagenode.ID(), time.Time{}, now)
				require.NoError(t, err)
				require.EqualValues(t, 0, snbw)

				_, _, newBbw, err := ordersDB.TestGetBucketBandwidth(ctx, projectID, []byte(bucketname), time.Time{}, now)
				require.NoError(t, err)
				require.EqualValues(t, 0, newBbw)
			})
		}
	})
}

func TestSettlementWithWindowFinal_TrustedOrders(t *testing.T) {
	// test checks if trusted node will be able to send orders without any issue
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Orders.TrustedOrders = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].DB.OverlayCache().UpdateNodeTags(ctx, nodeselection.NodeTags{
			nodeselection.NodeTag{
				NodeID:   planet.StorageNodes[0].ID(),
				SignedAt: time.Now(),
				Signer:   storj.NodeID{30: 1},
				Name:     "trusted_orders",
				Value:    []byte("true"),
			},
		})
		require.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "object", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

		planet.StorageNodes[0].Storage2.Orders.SendOrders(ctx, time.Now().Add(24*time.Hour))

		result, err := planet.StorageNodes[0].OrdersStore.ListUnsentBySatellite(ctx, time.Now().Add(24*time.Hour))
		require.NoError(t, err)
		require.Empty(t, result)
	})
}
