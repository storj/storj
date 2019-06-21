// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package vouchers_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestVouchersDB(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		vdb := db.Vouchers()

		satellite := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())
		storagenode := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion())

		expiration, err := ptypes.TimestampProto(time.Now().UTC().Add(24 * time.Hour))
		require.NoError(t, err)

		voucher := &pb.Voucher{
			SatelliteId:   satellite.ID,
			StorageNodeId: storagenode.ID,
			Expiration:    expiration,
		}

		// Test GetValid returns nil result and nil error when result is not found
		result, err := vdb.GetValid(ctx, []storj.NodeID{satellite.ID})
		require.NoError(t, err)
		assert.Nil(t, result)

		// Test Put returns no error
		err = vdb.Put(ctx, voucher)
		require.NoError(t, err)

		// Test GetValid returns accurate voucher
		result, err = vdb.GetValid(ctx, []storj.NodeID{satellite.ID})
		require.NoError(t, err)
		require.Equal(t, voucher.SatelliteId, result.SatelliteId)
		require.Equal(t, voucher.StorageNodeId, result.StorageNodeId)

		expectedTime, err := ptypes.Timestamp(voucher.GetExpiration())
		require.NoError(t, err)
		actualTime, err := ptypes.Timestamp(result.GetExpiration())
		require.NoError(t, err)

		require.Equal(t, expectedTime, actualTime)

		// test NeedVoucher returns true if voucher expiration falls within expirationBuffer period
		// voucher expiration is 24 hours from now
		expirationBuffer := 48 * time.Hour
		need, err := vdb.NeedVoucher(ctx, satellite.ID, expirationBuffer)
		require.NoError(t, err)
		require.True(t, need)

		// test NeedVoucher returns true if satellite ID does not exist in table
		need, err = vdb.NeedVoucher(ctx, teststorj.NodeIDFromString("testnodeID"), expirationBuffer)
		require.NoError(t, err)
		require.True(t, need)

		// test NeedVoucher returns false if satellite ID exists and expiration does not fall within expirationBuffer period
		// voucher expiration is 24 hours from now
		expirationBuffer = 1 * time.Hour
		need, err = vdb.NeedVoucher(ctx, satellite.ID, expirationBuffer)
		require.NoError(t, err)
		require.False(t, need)

		// Test Put with duplicate satellite id updates voucher info
		voucher.Expiration, err = ptypes.TimestampProto(time.Now().UTC().Add(48 * time.Hour))
		require.NoError(t, err)

		err = vdb.Put(ctx, voucher)
		require.NoError(t, err)

		result, err = vdb.GetValid(ctx, []storj.NodeID{satellite.ID})
		require.NoError(t, err)

		expectedTime, err = ptypes.Timestamp(voucher.GetExpiration())
		require.NoError(t, err)
		actualTime, err = ptypes.Timestamp(result.GetExpiration())
		require.NoError(t, err)

		require.Equal(t, expectedTime, actualTime)
	})
}

func TestVouchersService(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 5, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Vouchers.Expiration = time.Hour
				config.Overlay.Node.AuditCount = 1
				config.Audit.Interval = time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		node.Vouchers.Loop.Stop()

		// node type needs to be set to receive vouchers
		for _, sat := range planet.Satellites {
			_, err := sat.Overlay.Service.UpdateNodeInfo(ctx, node.ID(), &pb.InfoResponse{Type: pb.NodeType_STORAGE})
			require.NoError(t, err)
		}

		// run service and assert no vouchers (does not meet audit requirement)
		err := node.Vouchers.RunOnce(ctx)
		require.NoError(t, err)

		for _, sat := range planet.Satellites {
			voucher, err := node.DB.Vouchers().GetValid(ctx, []storj.NodeID{sat.ID()})
			require.NoError(t, err)
			assert.Nil(t, voucher)
		}

		// update node's audit count above reputable threshold on each satellite
		for _, sat := range planet.Satellites {
			_, err := sat.Overlay.Service.UpdateStats(ctx, &overlay.UpdateRequest{
				NodeID:       node.ID(),
				IsUp:         true,
				AuditSuccess: true,
				AuditLambda:  0,
				AuditWeight:  0,
				AuditDQ:      0,
				UptimeLambda: 0,
				UptimeWeight: 0,
				UptimeDQ:     0,
			})
			require.NoError(t, err)
		}

		// Node is now vetted. Run service and check vouchers have been received
		err = node.Vouchers.RunOnce(ctx)
		require.NoError(t, err)

		for _, sat := range planet.Satellites {
			voucher, err := node.DB.Vouchers().GetValid(ctx, []storj.NodeID{sat.ID()})
			require.NoError(t, err)
			assert.NotNil(t, voucher)
		}

		// Check expiration is updated
		oldVoucher, err := node.DB.Vouchers().GetValid(ctx, []storj.NodeID{planet.Satellites[0].ID()})
		require.NoError(t, err)

		// Run service and get new voucher with new expiration
		err = node.Vouchers.RunOnce(ctx)
		require.NoError(t, err)

		newVoucher, err := node.DB.Vouchers().GetValid(ctx, []storj.NodeID{planet.Satellites[0].ID()})
		require.NoError(t, err)

		// assert old expiration is before new expiration
		oldExpiration, err := ptypes.Timestamp(oldVoucher.GetExpiration())
		require.NoError(t, err)
		newExpiration, err := ptypes.Timestamp(newVoucher.GetExpiration())
		require.NoError(t, err)

		assert.True(t, oldExpiration.Before(newExpiration))
	})
}

func TestVerifyVoucher(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		service := planet.StorageNodes[0].Vouchers
		service.Loop.Pause()

		satellite0 := planet.Satellites[0]
		satellite1 := planet.Satellites[1]
		storagenode := planet.StorageNodes[0]

		tests := []struct {
			satelliteID      storj.NodeID
			storagenodeID    storj.NodeID
			expiration       time.Time
			invalidSignature bool
			err              string
		}{
			{ // passing
				satelliteID:      satellite0.ID(),
				storagenodeID:    storagenode.ID(),
				expiration:       time.Now().UTC().Add(24 * time.Hour),
				invalidSignature: false,
				err:              "",
			},
			{ // incorrect satellite ID
				satelliteID:      teststorj.NodeIDFromString("satellite"),
				storagenodeID:    storagenode.ID(),
				expiration:       time.Now().UTC().Add(24 * time.Hour),
				invalidSignature: false,
				err:              fmt.Sprintf("verification: Satellite ID does not match expected: (%v) (%v)", teststorj.NodeIDFromString("satellite"), satellite0.ID()),
			},
			{ // incorrect storagenode ID
				satelliteID:      satellite0.ID(),
				storagenodeID:    teststorj.NodeIDFromString("storagenode"),
				expiration:       time.Now().UTC().Add(24 * time.Hour),
				invalidSignature: false,
				err:              fmt.Sprintf("verification: Storage node ID does not match expected: (%v) (%v)", teststorj.NodeIDFromString("storagenode"), storagenode.ID()),
			},
			{ // expired voucher
				satelliteID:      satellite0.ID(),
				storagenodeID:    storagenode.ID(),
				expiration:       time.Now().UTC().Add(-24 * time.Hour),
				invalidSignature: false,
				err:              "verification: Voucher is already expired",
			},
			{ // invalid signature
				satelliteID:      satellite0.ID(),
				storagenodeID:    storagenode.ID(),
				expiration:       time.Now().UTC().Add(24 * time.Hour),
				invalidSignature: true,
				err:              fmt.Sprintf("verification: invalid voucher signature: signature verification error: signature is not valid"),
			},
		}

		for _, tt := range tests {
			expiration, err := ptypes.TimestampProto(tt.expiration)
			require.NoError(t, err)

			var signer signing.Signer
			if tt.invalidSignature {
				signer = signing.SignerFromFullIdentity(satellite1.Identity)
			} else {
				signer = signing.SignerFromFullIdentity(satellite0.Identity)
			}

			voucher, err := signing.SignVoucher(ctx, signer, &pb.Voucher{
				SatelliteId:   tt.satelliteID,
				StorageNodeId: tt.storagenodeID,
				Expiration:    expiration,
			})
			require.NoError(t, err)

			err = service.VerifyVoucher(ctx, satellite0.ID(), voucher)
			if tt.err != "" {
				require.Equal(t, tt.err, err.Error())
			} else {
				require.NoError(t, err)
			}
		}
	})
}
