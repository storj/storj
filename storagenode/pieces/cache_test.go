// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestDBInit(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		spaceUsedDB := db.PieceSpaceUsedDB()
		total, err := spaceUsedDB.GetTotal(ctx)
		require.NoError(t, err)
		require.Equal(t, total, int64(0))

		// Try to update the total record before we initialize
		err = spaceUsedDB.UpdateTotal(ctx, int64(100))
		require.NoError(t, err)

		// Expect that no total record exists since we haven't
		// initialized yet
		total, err = spaceUsedDB.GetTotal(ctx)
		require.NoError(t, err)
		require.Equal(t, total, int64(0))

		// Now initialize the db to create the total record
		err = spaceUsedDB.Init(ctx)
		require.NoError(t, err)

		// Now that a total record exists, we can update it
		err = spaceUsedDB.UpdateTotal(ctx, int64(100))
		require.NoError(t, err)

		// Confirm the total record is now 100
		total, err = spaceUsedDB.GetTotal(ctx)
		require.NoError(t, err)
		require.Equal(t, total, int64(100))
	})
}
func TestCacheInit(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		spaceUsedDB := db.PieceSpaceUsedDB()
		err := spaceUsedDB.Init(ctx)
		require.NoError(t, err)

		// setup the cache with zero values
		cache := pieces.NewBlobsUsageCacheTest(nil, 0, nil)
		cacheService := pieces.NewService(zap.L(),
			cache,
			pieces.NewStore(zap.L(), cache, nil, nil, spaceUsedDB),
			1*time.Hour,
		)

		// Confirm that when we call init before the cache has been persisted.
		// that the cache gets initialized with zero values
		err = cacheService.Init(ctx)
		require.NoError(t, err)
		actualTotal, err := cache.SpaceUsedForPieces(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(0), actualTotal)
		actualTotalBySA, err := cache.SpaceUsedBySatellite(ctx, storj.NodeID{1})
		require.NoError(t, err)
		require.Equal(t, int64(0), actualTotalBySA)

		// setup: update the cache then sync those cache values
		// to the database
		expectedTotal := int64(150)
		expectedTotalBySA := map[storj.NodeID]int64{{1}: 100, {2}: 50}
		cache = pieces.NewBlobsUsageCacheTest(nil, expectedTotal, expectedTotalBySA)
		cacheService = pieces.NewService(zap.L(),
			cache,
			pieces.NewStore(zap.L(), cache, nil, nil, spaceUsedDB),
			1*time.Hour,
		)
		err = cacheService.PersistCacheTotals(ctx)
		require.NoError(t, err)

		// Confirm that when we call Init after the cache has been persisted
		// that the cache gets initialized with the values from the database
		err = cacheService.Init(ctx)
		require.NoError(t, err)
		actualTotal, err = cache.SpaceUsedForPieces(ctx)
		require.NoError(t, err)
		require.Equal(t, expectedTotal, actualTotal)
		actualTotalBySA, err = cache.SpaceUsedBySatellite(ctx, storj.NodeID{1})
		require.NoError(t, err)
		require.Equal(t, int64(100), actualTotalBySA)
		actualTotalBySA, err = cache.SpaceUsedBySatellite(ctx, storj.NodeID{2})
		require.NoError(t, err)
		require.Equal(t, int64(50), actualTotalBySA)
	})

}

func TestPersistCacheTotals(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		// The database should start out with 0 for all totals
		var expectedTotal int64
		spaceUsedDB := db.PieceSpaceUsedDB()
		err := spaceUsedDB.Init(ctx)
		require.NoError(t, err)
		actualTotal, err := spaceUsedDB.GetTotal(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTotal, actualTotal)

		var expectedTotalBySA = map[storj.NodeID]int64{}
		actualTotalBySA, err := spaceUsedDB.GetTotalsForAllSatellites(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTotalBySA, actualTotalBySA)

		// setup: update the cache then sync those cache values
		// to the database
		// setup the cache with zero values
		expectedTotal = 150
		expectedTotalBySA = map[storj.NodeID]int64{{1}: 100, {2}: 50}
		cache := pieces.NewBlobsUsageCacheTest(nil, expectedTotal, expectedTotalBySA)
		cacheService := pieces.NewService(zap.L(),
			cache,
			pieces.NewStore(zap.L(), cache, nil, nil, spaceUsedDB),
			1*time.Hour,
		)
		err = cacheService.PersistCacheTotals(ctx)
		require.NoError(t, err)

		// Confirm those cache values are now saved persistently in the database
		actualTotal, err = spaceUsedDB.GetTotal(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTotal, actualTotal)

		actualTotalBySA, err = spaceUsedDB.GetTotalsForAllSatellites(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTotalBySA, actualTotalBySA)

		// Delete some piece content
		pieceContentSize := -int64(100)
		cache.Update(ctx, storj.NodeID{1}, pieceContentSize)
		err = cacheService.PersistCacheTotals(ctx)
		require.NoError(t, err)

		// Confirm that the deleted stuff is not in the database anymore
		actualTotal, err = spaceUsedDB.GetTotal(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTotal+pieceContentSize, actualTotal)

		expectedTotalBySA = map[storj.NodeID]int64{{2}: 50}
		actualTotalBySA, err = spaceUsedDB.GetTotalsForAllSatellites(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTotalBySA, actualTotalBySA)

	})
}

func TestRecalculateCache(t *testing.T) {
	testCases := []struct {
		name     string
		start    int64
		end      int64
		new      int64
		expected int64
	}{
		{"1", 0, 0, 0, 0},
		{"2", 0, 100, 0, 50},
		{"3", 0, 100, 90, 140},
		{"4", 0, 100, 110, 160},
		{"5", 0, 100, -10, 40},
		{"6", 0, 100, -200, 0},
		{"7", 100, 0, 0, 0},
		{"8", 100, 0, 90, 40},
		{"9", 100, 0, 30, 0},
		{"10", 100, 0, 110, 60},
		{"11", 100, 0, -10, 0},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx := testcontext.New(t)
			defer ctx.Cleanup()
			ID1 := storj.NodeID{1, 1}
			cache := pieces.NewBlobsUsageCacheTest(nil,
				tt.end,
				map[storj.NodeID]int64{ID1: tt.end},
			)

			err := cache.Recalculate(ctx,
				tt.new,
				tt.start,
				map[storj.NodeID]int64{ID1: tt.new},
				map[storj.NodeID]int64{ID1: tt.start},
			)
			require.NoError(t, err)

			// Test: confirm correct cache values
			actualTotalSpaceUsed, err := cache.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actualTotalSpaceUsed)

			actualTotalSpaceUsedBySA, err := cache.SpaceUsedBySatellite(ctx, ID1)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actualTotalSpaceUsedBySA)
		})
	}
}

func TestRecalculateCacheMissed(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	ID1 := storj.NodeID{1}
	ID2 := storj.NodeID{2}

	// setup: once we are done recalculating the pieces on disk,
	// there are items in the cache that are not in the
	// new recalculated values
	cache := pieces.NewBlobsUsageCacheTest(nil,
		int64(150),
		map[storj.NodeID]int64{ID1: int64(100), ID2: int64(50)},
	)

	err := cache.Recalculate(ctx,
		int64(100),
		int64(0),
		map[storj.NodeID]int64{ID1: int64(100)},
		map[storj.NodeID]int64{ID1: int64(0)},
	)
	require.NoError(t, err)

	// Test: confirm correct cache values
	actualTotalSpaceUsed, err := cache.SpaceUsedForPieces(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(175), actualTotalSpaceUsed)

	actualTotalSpaceUsedBySA, err := cache.SpaceUsedBySatellite(ctx, ID2)
	require.NoError(t, err)
	assert.Equal(t, int64(25), actualTotalSpaceUsedBySA)
}

func TestCacheCreateDelete(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		cache := pieces.NewBlobsUsageCache(db.Pieces())
		satelliteID := testrand.Bytes(32)
		ref := storage.BlobRef{
			Namespace: satelliteID,
			Key:       testrand.Bytes(32),
		}
		blob, err := cache.Create(ctx, ref, int64(4096))
		require.NoError(t, err)
		saID := storj.NodeID{}
		copy(saID[:], satelliteID)
		blobWriter, err := pieces.NewWriter(blob, cache, saID)
		require.NoError(t, err)
		pieceContent := []byte("stuff")
		_, err = blobWriter.Write(pieceContent)
		require.NoError(t, err)
		header := pb.PieceHeader{}
		err = blobWriter.Commit(ctx, &header)
		require.NoError(t, err)

		// Expect that the cache has those bytes written for the piece
		actualTotal, err := cache.SpaceUsedForPieces(ctx)
		require.NoError(t, err)
		require.Equal(t, len(pieceContent), int(actualTotal))
		actualTotalBySA, err := cache.SpaceUsedBySatellite(ctx, saID)
		require.NoError(t, err)
		require.Equal(t, len(pieceContent), int(actualTotalBySA))

		// Delete that piece and confirm the cache is updated
		err = cache.Delete(ctx, ref)
		require.NoError(t, err)

		actualTotal, err = cache.SpaceUsedForPieces(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, int(actualTotal))
		actualTotalBySA, err = cache.SpaceUsedBySatellite(ctx, saID)
		require.NoError(t, err)
		require.Equal(t, 0, int(actualTotalBySA))
	})
}

func TestCacheCreateMultipleSatellites(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite1 := planet.Satellites[0]
		satellite2 := planet.Satellites[1]
		uplink := planet.Uplinks[0]
		// Setup: create data for the uplink to upload
		expectedData := testrand.Bytes(5 * memory.KiB)
		err := uplink.Upload(ctx, satellite1, "testbucket", "test/path", expectedData)
		require.NoError(t, err)
		err = uplink.Upload(ctx, satellite2, "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		var total, total1, total2 int64
		for _, sn := range planet.StorageNodes {
			totalP, err := sn.Storage2.BlobsCache.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			total += totalP
			totalBySA1, err := sn.Storage2.BlobsCache.SpaceUsedBySatellite(ctx, satellite1.Identity.ID)
			require.NoError(t, err)
			total1 += totalBySA1
			totalBySA2, err := sn.Storage2.BlobsCache.SpaceUsedBySatellite(ctx, satellite2.Identity.ID)
			require.NoError(t, err)
			total2 += totalBySA2
		}
		require.Equal(t, int64(47104), total)
		require.Equal(t, int64(23552), total1)
		require.Equal(t, int64(23552), total2)
	})

}
