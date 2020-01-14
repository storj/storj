// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedbtest_test

import (
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/storagenodedb"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestDatabase(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
	})
}

func TestFileConcurrency(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	db, err := storagenodedb.New(log, storagenodedb.Config{
		Pieces: ctx.Dir("storage"),
		Info2:  ctx.Dir("storage") + "/info.db",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(db.Close)

	testConcurrency(t, ctx, db)
}

func TestInMemoryConcurrency(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	storageDir := ctx.Dir("storage")
	cfg := storagenodedb.Config{
		Pieces:  storageDir,
		Storage: storageDir,
		Info:    filepath.Join(storageDir, "piecestore.db"),
		Info2:   filepath.Join(storageDir, "info.db"),
	}

	db, err := storagenodedb.New(log, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(db.Close)

	testConcurrency(t, ctx, db)
}

func testConcurrency(t *testing.T, ctx *testcontext.Context, db *storagenodedb.DB) {
	t.Run("Sqlite", func(t *testing.T) {
		runtime.GOMAXPROCS(2)

		err := db.CreateTables(ctx)
		if err != nil {
			t.Fatal(err)
		}

		ordersMap := make(map[string]orders.Info)
		err = createOrders(t, ctx, ordersMap, 1000)
		require.NoError(t, err)

		err = insertOrders(t, ctx, db, ordersMap)
		require.NoError(t, err)

		err = verifyOrders(t, ctx, db, ordersMap)
		require.NoError(t, err)
	})
}

func insertOrders(t *testing.T, ctx *testcontext.Context, db *storagenodedb.DB, ordersMap map[string]orders.Info) (err error) {
	var wg sync.WaitGroup
	for _, order := range ordersMap {
		wg.Add(1)
		o := order
		go insertOrder(t, ctx, db, &wg, &o)

	}
	wg.Wait()
	return nil
}

func insertOrder(t *testing.T, ctx *testcontext.Context, db *storagenodedb.DB, wg *sync.WaitGroup, order *orders.Info) {
	defer wg.Done()
	err := db.Orders().Enqueue(ctx, order)
	require.NoError(t, err)
}

func verifyOrders(t *testing.T, ctx *testcontext.Context, db *storagenodedb.DB, orders map[string]orders.Info) (err error) {
	dbOrders, _ := db.Orders().ListUnsent(ctx, 10000)
	found := 0
	for _, order := range orders {
		for _, dbOrder := range dbOrders {
			if order.Order.SerialNumber == dbOrder.Order.SerialNumber {
				//fmt.Printf("Found %v\n", order.Order.SerialNumber)
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

	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	require.NoError(t, err)

	piece := testrand.PieceID()
	serialNumber := testrand.SerialNumber()
	expiration := time.Now()

	limit, err := signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satelliteIdentity), &pb.OrderLimit{
		SerialNumber:    serialNumber,
		SatelliteId:     satelliteIdentity.ID,
		UplinkPublicKey: piecePublicKey,
		StorageNodeId:   storageNodeIdentity.ID,
		PieceId:         piece,
		Limit:           100,
		Action:          pb.PieceAction_GET,
		PieceExpiration: expiration,
		OrderExpiration: expiration,
	})
	require.NoError(t, err)

	order, err := signing.SignUplinkOrder(ctx, piecePrivateKey, &pb.Order{
		SerialNumber: serialNumber,
		Amount:       50,
	})
	require.NoError(t, err)

	return &orders.Info{
		Limit: limit,
		Order: order,
	}
}
