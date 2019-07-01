// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedbtest_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/golang/protobuf/ptypes"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"

	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/orders"
)

func TestDatabase(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
	})
}

func TestConcurrency(t *testing.T) {
	t.Run("Sqlite", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		log := zaptest.NewLogger(t)

		db, err := storagenodedb.NewInMemory(log, ctx.Dir("storage"))
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Check(db.Close)

		err = db.CreateTables()
		if err != nil {
			t.Fatal(err)
		}

		orders := make(map[string]orders.Info)
		err = createOrders(t, ctx, orders, 1000)
		require.NoError(t, err)

		err = insertOrders(t, ctx, db, orders)
		require.NoError(t, err)

		err = verifyOrders(t, ctx, db, orders)
		require.NoError(t, err)

		fmt.Printf("Orders: %v", orders)
	})
}

func insertOrders(t *testing.T, ctx *testcontext.Context, db *storagenodedb.DB, os map[string]orders.Info) (err error) {
	var wg sync.WaitGroup
	for _, order := range os {
		wg.Add(1)
		o := order
		go func(wg *sync.WaitGroup, order *orders.Info) {
			defer wg.Done()
			err = db.Orders().Enqueue(ctx, order)
			require.NoError(t, err)
		}(&wg, &o)

	}
	wg.Wait()
	return nil
}

func verifyOrders(t *testing.T, ctx *testcontext.Context, db *storagenodedb.DB, orders map[string]orders.Info) (err error) {
	dbOrders, _ := db.Orders().ListUnsent(ctx, 10000)
	found := 0
	for _, order := range orders {
		for _, dbOrder := range dbOrders {
			if order.Order.SerialNumber == dbOrder.Order.SerialNumber {
				fmt.Printf("Found %v\n", order.Order.SerialNumber)
				found++
			}
		}
	}
	require.Equal(t, len(orders), found, "Number found must equal the length of the test data")
	return nil
}

func createOrders(t *testing.T, ctx *testcontext.Context, orders map[string]orders.Info, count int) (err error) {
	for i := 0; i < count; i++ {
		key, err := uuid.New()
		if err != nil {
			return err
		}
		order := createOrder(t, ctx)
		orders[key.String()] = *order
	}
	return nil
}
func createOrder(t *testing.T, ctx *testcontext.Context) (info *orders.Info) {

	storageNodeIdentity := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())

	satelliteIdentity := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion())

	uplink := testidentity.MustPregeneratedSignedIdentity(3, storj.LatestIDVersion())
	piece := storj.NewPieceID()

	serialNumber := testrand.SerialNumber()
	exp := ptypes.TimestampNow()

	limit, err := signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satelliteIdentity), &pb.OrderLimit{
		SerialNumber:    serialNumber,
		SatelliteId:     satelliteIdentity.ID,
		UplinkId:        uplink.ID,
		StorageNodeId:   storageNodeIdentity.ID,
		PieceId:         piece,
		Limit:           100,
		Action:          pb.PieceAction_GET,
		PieceExpiration: exp,
		OrderExpiration: exp,
	})
	require.NoError(t, err)

	order, err := signing.SignOrder(ctx, signing.SignerFromFullIdentity(uplink), &pb.Order{
		SerialNumber: serialNumber,
		Amount:       50,
	})
	require.NoError(t, err)

	return &orders.Info{
		Limit:  limit,
		Order:  order,
		Uplink: uplink.PeerIdentity(),
	}
}
