// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
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
		total, err := spaceUsedDB.GetPieceTotal(ctx)
		require.NoError(t, err)
		require.Equal(t, total, int64(0))

		// Expect that no total record exists since we haven't
		// initialized yet
		total, err = spaceUsedDB.GetPieceTotal(ctx)
		require.NoError(t, err)
		require.Equal(t, total, int64(0))

		// Expect no record for trash total
		trashTotal, err := spaceUsedDB.GetTrashTotal(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(0), trashTotal)

		// Now initialize the db to create the total record
		err = spaceUsedDB.Init(ctx)
		require.NoError(t, err)

		// Now that a total record exists, we can update it
		err = spaceUsedDB.UpdatePieceTotal(ctx, int64(100))
		require.NoError(t, err)

		err = spaceUsedDB.UpdateTrashTotal(ctx, int64(150))
		require.NoError(t, err)

		// Confirm the total record has been updated
		total, err = spaceUsedDB.GetPieceTotal(ctx)
		require.NoError(t, err)
		require.Equal(t, total, int64(100))

		// Confirm the trash total record has been updated
		total, err = spaceUsedDB.GetTrashTotal(ctx)
		require.NoError(t, err)
		require.Equal(t, total, int64(150))
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
		cache := pieces.NewBlobsUsageCacheTest(nil, 0, 0, nil)
		cacheService := pieces.NewService(zap.L(),
			cache,
			pieces.NewStore(zap.L(), cache, nil, nil, spaceUsedDB),
			1*time.Hour,
		)

		// Confirm that when we call init before the cache has been persisted.
		// that the cache gets initialized with zero values
		err = cacheService.Init(ctx)
		require.NoError(t, err)
		piecesTotal, err := cache.SpaceUsedForPieces(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(0), piecesTotal)
		actualTotalBySA, err := cache.SpaceUsedBySatellite(ctx, storj.NodeID{1})
		require.NoError(t, err)
		require.Equal(t, int64(0), actualTotalBySA)
		trashTotal, err := cache.SpaceUsedForTrash(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(0), trashTotal)

		// setup: update the cache then sync those cache values
		// to the database
		expectedPieces := int64(150)
		expectedTotalBySA := map[storj.NodeID]int64{{1}: 100, {2}: 50}
		expectedTrash := int64(127)
		cache = pieces.NewBlobsUsageCacheTest(nil, expectedPieces, expectedTrash, expectedTotalBySA)
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
		piecesTotal, err = cache.SpaceUsedForPieces(ctx)
		require.NoError(t, err)
		require.Equal(t, expectedPieces, piecesTotal)
		actualTotalBySA, err = cache.SpaceUsedBySatellite(ctx, storj.NodeID{1})
		require.NoError(t, err)
		require.Equal(t, int64(100), actualTotalBySA)
		actualTotalBySA, err = cache.SpaceUsedBySatellite(ctx, storj.NodeID{2})
		require.NoError(t, err)
		require.Equal(t, int64(50), actualTotalBySA)
		actualTrash, err := cache.SpaceUsedForTrash(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(127), actualTrash)
	})

}

func TestPersistCacheTotals(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		// The database should start out with 0 for all totals
		var expectedPieces int64
		spaceUsedDB := db.PieceSpaceUsedDB()
		err := spaceUsedDB.Init(ctx)
		require.NoError(t, err)
		piecesTotal, err := spaceUsedDB.GetPieceTotal(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedPieces, piecesTotal)

		var expectedTrash int64
		err = spaceUsedDB.Init(ctx)
		require.NoError(t, err)
		trashTotal, err := spaceUsedDB.GetTrashTotal(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTrash, trashTotal)

		var expectedTotalBySA = map[storj.NodeID]int64{}
		actualTotalBySA, err := spaceUsedDB.GetPieceTotalsForAllSatellites(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTotalBySA, actualTotalBySA)

		// setup: update the cache then sync those cache values
		// to the database
		// setup the cache with zero values
		expectedPieces = 150
		expectedTotalBySA = map[storj.NodeID]int64{{1}: 100, {2}: 50}
		expectedTrash = 127
		cache := pieces.NewBlobsUsageCacheTest(nil, expectedPieces, expectedTrash, expectedTotalBySA)
		cacheService := pieces.NewService(zap.L(),
			cache,
			pieces.NewStore(zap.L(), cache, nil, nil, spaceUsedDB),
			1*time.Hour,
		)
		err = cacheService.PersistCacheTotals(ctx)
		require.NoError(t, err)

		// Confirm those cache values are now saved persistently in the database
		piecesTotal, err = spaceUsedDB.GetPieceTotal(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedPieces, piecesTotal)

		actualTotalBySA, err = spaceUsedDB.GetPieceTotalsForAllSatellites(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTotalBySA, actualTotalBySA)

		trashTotal, err = spaceUsedDB.GetTrashTotal(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTrash, trashTotal)

		// Delete some piece content
		pieceContentSize := -int64(100)
		trashDelta := int64(104)
		cache.Update(ctx, storj.NodeID{1}, pieceContentSize, trashDelta)
		err = cacheService.PersistCacheTotals(ctx)
		require.NoError(t, err)

		// Confirm that the deleted stuff is not in the database anymore
		piecesTotal, err = spaceUsedDB.GetPieceTotal(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedPieces+pieceContentSize, piecesTotal)

		trashTotal, err = spaceUsedDB.GetTrashTotal(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTrash+trashDelta, trashTotal)

		expectedTotalBySA = map[storj.NodeID]int64{{2}: 50}
		actualTotalBySA, err = spaceUsedDB.GetPieceTotalsForAllSatellites(ctx)
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

		startTrash    int64
		endTrash      int64
		newTrash      int64
		expectedTrash int64
	}{
		{"1", 0, 0, 0, 0, 0, 0, 0, 0},
		{"2", 0, 100, 0, 50, 100, 110, 50, 55},
		{"3", 0, 100, 90, 140, 0, 100, 50, 100},
		{"4", 0, 100, 110, 160, 0, 100, -10, 40},
		{"5", 0, 100, -10, 40, 0, 0, 0, 0},
		{"6", 0, 100, -200, 0, 0, 0, 0, 0},
		{"7", 100, 0, 0, 0, 0, 0, 0, 0},
		{"8", 100, 0, 90, 40, 0, 0, 0, 0},
		{"9", 100, 0, 30, 0, 0, 0, 0, 0},
		{"10", 100, 0, 110, 60, 0, 0, 0, 0},
		{"11", 100, 0, -10, 0, 0, 0, 0, 0},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			ID1 := storj.NodeID{1, 1}
			cache := pieces.NewBlobsUsageCacheTest(nil,
				tt.end,
				tt.endTrash,
				map[storj.NodeID]int64{ID1: tt.end},
			)

			cache.Recalculate(ctx,
				tt.new,
				tt.start,
				map[storj.NodeID]int64{ID1: tt.new},
				map[storj.NodeID]int64{ID1: tt.start},
				tt.newTrash,
				tt.startTrash,
			)

			// Test: confirm correct cache values
			actualTotalSpaceUsed, err := cache.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actualTotalSpaceUsed)

			actualTotalSpaceUsedBySA, err := cache.SpaceUsedBySatellite(ctx, ID1)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actualTotalSpaceUsedBySA)

			actualTrash, err := cache.SpaceUsedForTrash(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTrash, actualTrash)
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
		int64(100),
		map[storj.NodeID]int64{ID1: int64(100), ID2: int64(50)},
	)

	cache.Recalculate(ctx,
		int64(100),
		int64(0),
		map[storj.NodeID]int64{ID1: int64(100)},
		map[storj.NodeID]int64{ID1: int64(0)},
		200,
		0,
	)

	// Test: confirm correct cache values
	actualTotalSpaceUsed, err := cache.SpaceUsedForPieces(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(175), actualTotalSpaceUsed)

	actualTotalSpaceUsedBySA, err := cache.SpaceUsedBySatellite(ctx, ID2)
	require.NoError(t, err)
	assert.Equal(t, int64(25), actualTotalSpaceUsedBySA)

	actualTrash, err := cache.SpaceUsedForTrash(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(250), actualTrash)
}

func TestCacheCreateDeleteAndTrash(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		cache := pieces.NewBlobsUsageCache(db.Pieces())
		pieceContent := []byte("stuff")
		satelliteID := testrand.NodeID()
		refs := []storage.BlobRef{
			{
				Namespace: satelliteID.Bytes(),
				Key:       testrand.Bytes(32),
			},
			{
				Namespace: satelliteID.Bytes(),
				Key:       testrand.Bytes(32),
			},
		}
		for _, ref := range refs {
			blob, err := cache.Create(ctx, ref, int64(4096))
			require.NoError(t, err)
			blobWriter, err := pieces.NewWriter(blob, cache, satelliteID)
			require.NoError(t, err)
			_, err = blobWriter.Write(pieceContent)
			require.NoError(t, err)
			header := pb.PieceHeader{}
			err = blobWriter.Commit(ctx, &header)
			require.NoError(t, err)
		}

		assertValues := func(msg string, satID storj.NodeID, expPiece, expTrash int) {
			piecesTotal, err := cache.SpaceUsedForPieces(ctx)
			require.NoError(t, err, msg)
			assert.Equal(t, expPiece, int(piecesTotal), msg)
			actualTotalBySA, err := cache.SpaceUsedBySatellite(ctx, satelliteID)
			require.NoError(t, err, msg)
			assert.Equal(t, expPiece, int(actualTotalBySA), msg)
			trashTotal, err := cache.SpaceUsedForTrash(ctx)
			require.NoError(t, err, msg)
			assert.Equal(t, expTrash, int(trashTotal), msg)
		}

		assertValues("first write", satelliteID, len(pieceContent)*2, 0)

		// Trash one piece
		blobInfo, err := cache.Stat(ctx, refs[0])
		require.NoError(t, err)
		fileInfo, err := blobInfo.Stat(ctx)
		require.NoError(t, err)
		ref0Size := fileInfo.Size()
		err = cache.Trash(ctx, refs[0])
		require.NoError(t, err)
		assertValues("trashed refs[0]", satelliteID, len(pieceContent), int(ref0Size))

		// Restore one piece
		_, err = cache.RestoreTrash(ctx, satelliteID.Bytes())
		require.NoError(t, err)
		assertValues("restore trash for satellite", satelliteID, len(pieceContent)*2, 0)

		// Trash piece again
		err = cache.Trash(ctx, refs[0])
		require.NoError(t, err)
		assertValues("trashed refs[0]", satelliteID, len(pieceContent), int(ref0Size))

		// Empty trash
		_, _, err = cache.EmptyTrash(ctx, satelliteID.Bytes(), time.Now().Add(24*time.Hour))
		require.NoError(t, err)
		assertValues("trashed refs[0]", satelliteID, len(pieceContent), 0)

		// Delete that piece and confirm the cache is updated
		err = cache.Delete(ctx, refs[1])
		require.NoError(t, err)

		assertValues("delete refs[0]", satelliteID, 0, 0)
	})
}

func TestCacheCreateMultipleSatellites(t *testing.T) {
	t.Skip("flaky: V3-2416")
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

func TestConcurrency(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		satellite := planet.Satellites[0]

		var group errgroup.Group
		group.Go(func() error {
			node.Storage2.BlobsCache.Update(ctx, satellite.ID(), 1000, 0)
			return nil
		})
		err := node.Storage2.CacheService.PersistCacheTotals(ctx)
		require.NoError(t, err)
		require.NoError(t, group.Wait())
	})
}
