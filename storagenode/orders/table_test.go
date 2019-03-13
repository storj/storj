// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"testing"

	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func TestOrders(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	db, err := storagenodedb.NewInfoInMemory()
	require.NoError(t, err)
	defer ctx.Check(db.Close)

	require.NoError(t, db.CreateTables(log))

	ordersdb := db.Orders()

	storagenode := testplanet.MustPregeneratedSignedIdentity(0)

	satellite0 := testplanet.MustPregeneratedSignedIdentity(1)

	uplink := testplanet.MustPregeneratedSignedIdentity(3)
	piece := storj.NewPieceID()

	serialNumber := newRandomSerial()

	// basic test
	_, err = ordersdb.ListUnsent(ctx, 100)
	require.NoError(t, err)

	now := ptypes.TimestampNow()

	limit, err := signing.SignOrderLimit(signing.SignerFromFullIdentity(satellite0), &pb.OrderLimit2{
		SerialNumber:    serialNumber,
		SatelliteId:     satellite0.ID,
		UplinkId:        uplink.ID,
		StorageNodeId:   storagenode.ID,
		PieceId:         piece,
		Limit:           100,
		Action:          pb.Action_GET,
		PieceExpiration: now,
		OrderExpiration: now,
	})
	require.NoError(t, err)

	order, err := signing.SignOrder(signing.SignerFromFullIdentity(uplink), &pb.Order2{
		SerialNumber: serialNumber,
		Amount:       50,
	})
	require.NoError(t, err)

	info := &storagenodedb.OrderInfo{limit, order, uplink.PeerIdentity()}

	// basic add
	err = ordersdb.Enqueue(ctx, info)
	require.NoError(t, err)

	// duplicate add
	err = ordersdb.Enqueue(ctx, info)
	require.Error(t, err, "duplicate add")

	orders, err := ordersdb.ListUnsent(ctx, 100)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff([]*storagenodedb.OrderInfo{info}, orders, cmp.Comparer(pb.Equal)))
}
