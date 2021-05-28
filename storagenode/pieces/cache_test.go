// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storage"
	"storj.io/storj/storage/filestore"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestDBInit(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		spaceUsedDB := db.PieceSpaceUsedDB()

		// Expect that no total record exists since we haven't
		// initialized yet
		piecesTotal, piecesContentSize, err := spaceUsedDB.GetPieceTotals(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(0), piecesTotal)
		require.Equal(t, int64(0), piecesContentSize)

		// Expect no record for trash total
		trashTotal, err := spaceUsedDB.GetTrashTotal(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(0), trashTotal)

		// Now initialize the db to create the total record
		err = spaceUsedDB.Init(ctx)
		require.NoError(t, err)

		// Now that a total record exists, we can update it
		err = spaceUsedDB.UpdatePieceTotals(ctx, int64(100), int64(101))
		require.NoError(t, err)

		err = spaceUsedDB.UpdateTrashTotal(ctx, int64(150))
		require.NoError(t, err)

		// Confirm the total record has been updated
		piecesTotal, piecesContentSize, err = spaceUsedDB.GetPieceTotals(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(100), piecesTotal)
		require.Equal(t, int64(101), piecesContentSize)

		// Confirm the trash total record has been updated
		trashTotal, err = spaceUsedDB.GetTrashTotal(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(150), trashTotal)
	})
}

func TestUpdate(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	sat1 := testrand.NodeID()
	startSats := map[storj.NodeID]pieces.SatelliteUsage{
		sat1: {
			Total:       -20,
			ContentSize: -21,
		},
	}
	cache := pieces.NewBlobsUsageCacheTest(zaptest.NewLogger(t), nil, -10, -11, -12, startSats)

	// Sanity check that the values are negative to start
	piecesTotal, piecesContentSize, err := cache.SpaceUsedForPieces(ctx)
	require.NoError(t, err)

	trashTotal, err := cache.SpaceUsedForTrash(ctx)
	require.NoError(t, err)

	require.Equal(t, int64(-10), piecesTotal)
	require.Equal(t, int64(-11), piecesContentSize)
	require.Equal(t, int64(-12), trashTotal)

	cache.Update(ctx, sat1, -1, -2, -3)

	piecesTotal, piecesContentSize, err = cache.SpaceUsedForPieces(ctx)
	require.NoError(t, err)

	trashTotal, err = cache.SpaceUsedForTrash(ctx)
	require.NoError(t, err)

	require.Equal(t, int64(0), piecesTotal)
	require.Equal(t, int64(0), piecesContentSize)
	require.Equal(t, int64(0), trashTotal)
}

func TestCacheInit(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		spaceUsedDB := db.PieceSpaceUsedDB()
		err := spaceUsedDB.Init(ctx)
		require.NoError(t, err)

		log := zaptest.NewLogger(t)
		// setup the cache with zero values
		cache := pieces.NewBlobsUsageCacheTest(log, nil, 0, 0, 0, nil)
		cacheService := pieces.NewService(log,
			cache,
			pieces.NewStore(log, cache, nil, nil, spaceUsedDB, pieces.DefaultConfig),
			1*time.Hour,
		)

		// Confirm that when we call init before the cache has been persisted.
		// that the cache gets initialized with zero values
		err = cacheService.Init(ctx)
		require.NoError(t, err)
		piecesTotal, piecesContentSize, err := cache.SpaceUsedForPieces(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(0), piecesTotal)
		require.Equal(t, int64(0), piecesContentSize)
		satPiecesTotal, satPiecesContentSize, err := cache.SpaceUsedBySatellite(ctx, storj.NodeID{1})
		require.NoError(t, err)
		require.Equal(t, int64(0), satPiecesTotal)
		require.Equal(t, int64(0), satPiecesContentSize)
		trashTotal, err := cache.SpaceUsedForTrash(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(0), trashTotal)

		// setup: update the cache then sync those cache values
		// to the database
		expectedPiecesTotal := int64(150)
		expectedPiecesContentSize := int64(151)
		expectedTotalBySA := map[storj.NodeID]pieces.SatelliteUsage{
			{1}: {
				Total:       100,
				ContentSize: 101,
			},
			{2}: {
				Total:       50,
				ContentSize: 51,
			},
		}
		expectedTrash := int64(127)
		cache = pieces.NewBlobsUsageCacheTest(log, nil, expectedPiecesTotal, expectedPiecesContentSize, expectedTrash, expectedTotalBySA)
		cacheService = pieces.NewService(log,
			cache,
			pieces.NewStore(log, cache, nil, nil, spaceUsedDB, pieces.DefaultConfig),
			1*time.Hour,
		)
		err = cacheService.PersistCacheTotals(ctx)
		require.NoError(t, err)

		// Now create an empty cache. Values will be read later by the db.
		cache = pieces.NewBlobsUsageCacheTest(log, nil, 0, 0, 0, nil)
		cacheService = pieces.NewService(log,
			cache,
			pieces.NewStore(log, cache, nil, nil, spaceUsedDB, pieces.DefaultConfig),
			1*time.Hour,
		)
		// Confirm that when we call Init after the cache has been persisted
		// that the cache gets initialized with the values from the database
		err = cacheService.Init(ctx)
		require.NoError(t, err)
		piecesTotal, piecesContentSize, err = cache.SpaceUsedForPieces(ctx)
		require.NoError(t, err)
		require.Equal(t, expectedPiecesTotal, piecesTotal)
		require.Equal(t, expectedPiecesContentSize, piecesContentSize)
		sat1PiecesTotal, sat1PiecesContentSize, err := cache.SpaceUsedBySatellite(ctx, storj.NodeID{1})
		require.NoError(t, err)
		require.Equal(t, int64(100), sat1PiecesTotal)
		require.Equal(t, int64(101), sat1PiecesContentSize)
		sat2PiecesTotal, sat2PiecesContentSize, err := cache.SpaceUsedBySatellite(ctx, storj.NodeID{2})
		require.NoError(t, err)
		require.Equal(t, int64(50), sat2PiecesTotal)
		require.Equal(t, int64(51), sat2PiecesContentSize)
		actualTrash, err := cache.SpaceUsedForTrash(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(127), actualTrash)
	})

}

func TestCachServiceRun(t *testing.T) {
	log := zaptest.NewLogger(t)
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		spaceUsedDB := db.PieceSpaceUsedDB()

		blobstore, err := filestore.NewAt(log, ctx.Dir(), filestore.DefaultConfig)
		require.NoError(t, err)

		// Prior to initializing the cache service (which should walk the files),
		// write a single file so something exists to be counted
		expBlobSize := memory.KB
		w, err := blobstore.Create(ctx, storage.BlobRef{
			Namespace: testrand.NodeID().Bytes(),
			Key:       testrand.PieceID().Bytes(),
		}, -1)
		require.NoError(t, err)
		_, err = w.Write(testrand.Bytes(expBlobSize))
		require.NoError(t, err)
		require.NoError(t, w.Commit(ctx))

		// Now write a piece that we are going to trash
		expTrashSize := 2 * memory.KB
		trashRef := storage.BlobRef{
			Namespace: testrand.NodeID().Bytes(),
			Key:       testrand.PieceID().Bytes(),
		}
		w, err = blobstore.Create(ctx, trashRef, -1)
		require.NoError(t, err)
		_, err = w.Write(testrand.Bytes(expTrashSize))
		require.NoError(t, err)
		require.NoError(t, w.Commit(ctx))
		require.NoError(t, blobstore.Trash(ctx, trashRef)) // trash it

		// Now instantiate the cache
		cache := pieces.NewBlobsUsageCache(log, blobstore)
		cacheService := pieces.NewService(log,
			cache,
			pieces.NewStore(log, cache, nil, nil, spaceUsedDB, pieces.DefaultConfig),
			1*time.Hour,
		)

		// Init the cache service, to read the values from the db (should all be 0)
		require.NoError(t, cacheService.Init(ctx))
		piecesTotal, piecesContentSize, err := cache.SpaceUsedForPieces(ctx)
		require.NoError(t, err)
		trashTotal, err := cache.SpaceUsedForTrash(ctx)
		require.NoError(t, err)

		// Assert that all the values start as 0, since we have not walked the files
		assert.Equal(t, int64(0), piecesTotal)
		assert.Equal(t, int64(0), piecesContentSize)
		assert.Equal(t, int64(0), trashTotal)

		// Run the cache service, which will walk all the pieces
		var eg errgroup.Group
		eg.Go(func() error {
			return cacheService.Run(ctx)
		})

		// Wait for the cache service init to finish
		cacheService.InitFence.Wait(ctx)

		// Check and verify that the reported sizes match expected values
		piecesTotal, piecesContentSize, err = cache.SpaceUsedForPieces(ctx)
		require.NoError(t, err)
		trashTotal, err = cache.SpaceUsedForTrash(ctx)
		require.NoError(t, err)

		assert.Equal(t, int64(expBlobSize), piecesTotal)
		assert.Equal(t, int64(expBlobSize-pieces.V1PieceHeaderReservedArea), piecesContentSize)
		assert.True(t, trashTotal >= int64(expTrashSize))

		require.NoError(t, cacheService.Close())
		require.NoError(t, eg.Wait())
	})
}

func TestPersistCacheTotals(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		log := zaptest.NewLogger(t)

		// The database should start out with 0 for all totals
		var expectedPiecesTotal int64
		var expectedPiecesContentSize int64
		spaceUsedDB := db.PieceSpaceUsedDB()
		err := spaceUsedDB.Init(ctx)
		require.NoError(t, err)
		piecesTotal, piecesContentSize, err := spaceUsedDB.GetPieceTotals(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedPiecesTotal, piecesTotal)
		assert.Equal(t, expectedPiecesContentSize, piecesContentSize)

		var expectedTrash int64
		err = spaceUsedDB.Init(ctx)
		require.NoError(t, err)
		trashTotal, err := spaceUsedDB.GetTrashTotal(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTrash, trashTotal)

		var expectedTotalsBySA = map[storj.NodeID]pieces.SatelliteUsage{}
		totalsBySA, err := spaceUsedDB.GetPieceTotalsForAllSatellites(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTotalsBySA, totalsBySA)

		// setup: update the cache then sync those cache values
		// to the database
		// setup the cache with zero values
		expectedPiecesTotal = 150
		expectedPiecesContentSize = 151
		expectedTotalsBySA = map[storj.NodeID]pieces.SatelliteUsage{
			{1}: {
				Total:       100,
				ContentSize: 101,
			},
			{2}: {
				Total:       50,
				ContentSize: 51,
			},
		}
		expectedTrash = 127
		cache := pieces.NewBlobsUsageCacheTest(log, nil, expectedPiecesTotal, expectedPiecesContentSize, expectedTrash, expectedTotalsBySA)
		cacheService := pieces.NewService(log,
			cache,
			pieces.NewStore(log, cache, nil, nil, spaceUsedDB, pieces.DefaultConfig),
			1*time.Hour,
		)
		err = cacheService.PersistCacheTotals(ctx)
		require.NoError(t, err)

		// Confirm those cache values are now saved persistently in the database
		piecesTotal, piecesContentSize, err = spaceUsedDB.GetPieceTotals(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedPiecesTotal, piecesTotal)
		assert.Equal(t, expectedPiecesContentSize, piecesContentSize)

		totalsBySA, err = spaceUsedDB.GetPieceTotalsForAllSatellites(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTotalsBySA, totalsBySA)

		trashTotal, err = spaceUsedDB.GetTrashTotal(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTrash, trashTotal)

		// Change piece sizes
		piecesTotalDelta := int64(35)
		piecesContentSizeDelta := int64(30)
		trashDelta := int64(35)
		cache.Update(ctx, storj.NodeID{1}, -piecesTotalDelta, -piecesContentSizeDelta, trashDelta)
		err = cacheService.PersistCacheTotals(ctx)
		require.NoError(t, err)

		// Confirm that the deleted stuff is not in the database anymore
		piecesTotal, piecesContentSize, err = spaceUsedDB.GetPieceTotals(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedPiecesTotal-piecesTotalDelta, piecesTotal)
		assert.Equal(t, expectedPiecesContentSize-piecesContentSizeDelta, piecesContentSize)

		trashTotal, err = spaceUsedDB.GetTrashTotal(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTrash+trashDelta, trashTotal)

		expectedTotalsBySA = map[storj.NodeID]pieces.SatelliteUsage{
			{1}: {
				Total:       65,
				ContentSize: 71,
			},
			{2}: {
				Total:       50,
				ContentSize: 51,
			},
		}
		totalsBySA, err = spaceUsedDB.GetPieceTotalsForAllSatellites(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedTotalsBySA, totalsBySA)

	})
}

func TestRecalculateCache(t *testing.T) {
	type testItem struct {
		start, end, new, expected int64
	}
	type testCase struct {
		name              string
		piecesTotal       testItem
		piecesContentSize testItem
		trash             testItem
	}
	testCases := []testCase{
		{
			name: "1",
			piecesTotal: testItem{
				start:    0,
				end:      0,
				new:      0,
				expected: 0,
			},
			piecesContentSize: testItem{
				start:    0,
				end:      0,
				new:      0,
				expected: 0,
			},
			trash: testItem{
				start:    0,
				end:      0,
				new:      0,
				expected: 0,
			},
		},
		{
			name: "2",
			piecesTotal: testItem{
				start:    0,
				end:      100,
				new:      0,
				expected: 50,
			},
			piecesContentSize: testItem{
				start:    0,
				end:      100,
				new:      0,
				expected: 50,
			},
			trash: testItem{
				start:    100,
				end:      110,
				new:      50,
				expected: 55,
			},
		},
		{
			name: "3",
			piecesTotal: testItem{
				start:    0,
				end:      100,
				new:      90,
				expected: 140,
			},
			piecesContentSize: testItem{
				start:    0,
				end:      50,
				new:      100,
				expected: 125,
			},
			trash: testItem{
				start:    0,
				end:      100,
				new:      50,
				expected: 100,
			},
		},
		{
			name: "4",
			piecesTotal: testItem{
				start:    0,
				end:      100,
				new:      -25,
				expected: 25,
			},
			piecesContentSize: testItem{
				start:    0,
				end:      100,
				new:      -50,
				expected: 0,
			},
			trash: testItem{
				start:    0,
				end:      10,
				new:      -3,
				expected: 2,
			},
		},
		{
			name: "5",
			piecesTotal: testItem{
				start:    100,
				end:      0,
				new:      -90,
				expected: 0,
			},
			piecesContentSize: testItem{
				start:    100,
				end:      0,
				new:      50,
				expected: 0,
			},
			trash: testItem{
				start:    100,
				end:      0,
				new:      -25,
				expected: 0,
			},
		},
		{
			name: "6",
			piecesTotal: testItem{
				start:    50,
				end:      -50,
				new:      -50,
				expected: 0,
			},
		},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx := testcontext.New(t)
			defer ctx.Cleanup()
			log := zaptest.NewLogger(t)

			ID1 := storj.NodeID{1, 1}
			cache := pieces.NewBlobsUsageCacheTest(log, nil,
				tt.piecesTotal.end,
				tt.piecesContentSize.end,
				tt.trash.end,
				map[storj.NodeID]pieces.SatelliteUsage{ID1: {Total: tt.piecesTotal.end, ContentSize: tt.piecesContentSize.end}},
			)

			cache.Recalculate(
				tt.piecesTotal.new,
				tt.piecesTotal.start,
				tt.piecesContentSize.new,
				tt.piecesContentSize.start,
				tt.trash.new,
				tt.trash.start,
				map[storj.NodeID]pieces.SatelliteUsage{ID1: {Total: tt.piecesTotal.new, ContentSize: tt.piecesContentSize.new}},
				map[storj.NodeID]pieces.SatelliteUsage{ID1: {Total: tt.piecesTotal.start, ContentSize: tt.piecesContentSize.start}},
			)

			// Test: confirm correct cache values
			piecesTotal, piecesContentSize, err := cache.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.piecesTotal.expected, piecesTotal)
			assert.Equal(t, tt.piecesContentSize.expected, piecesContentSize)

			sat1PiecesTotal, sat1PiecesContentSize, err := cache.SpaceUsedBySatellite(ctx, ID1)
			require.NoError(t, err)
			assert.Equal(t, tt.piecesTotal.expected, sat1PiecesTotal)
			assert.Equal(t, tt.piecesContentSize.expected, sat1PiecesContentSize)

			trashTotal, err := cache.SpaceUsedForTrash(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.trash.expected, trashTotal)
		})
	}
}

func TestRecalculateCacheMissed(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	log := zaptest.NewLogger(t)

	ID1 := storj.NodeID{1}
	ID2 := storj.NodeID{2}

	// setup: once we are done recalculating the pieces on disk,
	// there are items in the cache that are not in the
	// new recalculated values
	cache := pieces.NewBlobsUsageCacheTest(log, nil,
		150,
		200,
		100,
		map[storj.NodeID]pieces.SatelliteUsage{ID1: {Total: 100, ContentSize: 50}, ID2: {Total: 100, ContentSize: 50}},
	)

	cache.Recalculate(
		100,
		0,
		50,
		25,
		200,
		0,
		map[storj.NodeID]pieces.SatelliteUsage{ID1: {Total: 100, ContentSize: 50}},
		map[storj.NodeID]pieces.SatelliteUsage{ID1: {Total: 0, ContentSize: 0}},
	)

	// Test: confirm correct cache values
	piecesTotal, piecesContentSize, err := cache.SpaceUsedForPieces(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(175), piecesTotal)
	assert.Equal(t, int64(137), piecesContentSize)

	piecesTotal, piecesContentSize, err = cache.SpaceUsedBySatellite(ctx, ID2)
	require.NoError(t, err)
	assert.Equal(t, int64(50), piecesTotal)
	assert.Equal(t, int64(25), piecesContentSize)

	trashTotal, err := cache.SpaceUsedForTrash(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(250), trashTotal)
}

func TestCacheCreateDeleteAndTrash(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		cache := pieces.NewBlobsUsageCache(zaptest.NewLogger(t), db.Pieces())
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
			blobWriter, err := pieces.NewWriter(zaptest.NewLogger(t), blob, cache, satelliteID)
			require.NoError(t, err)
			_, err = blobWriter.Write(pieceContent)
			require.NoError(t, err)
			header := pb.PieceHeader{}
			err = blobWriter.Commit(ctx, &header)
			require.NoError(t, err)
		}

		assertValues := func(msg string, satID storj.NodeID, expPiecesTotal, expPiecesContentSize, expTrash int) {
			piecesTotal, piecesContentSize, err := cache.SpaceUsedForPieces(ctx)
			require.NoError(t, err, msg)
			assert.Equal(t, expPiecesTotal, int(piecesTotal), msg)
			assert.Equal(t, expPiecesContentSize, int(piecesContentSize), msg)
			piecesTotal, piecesContentSize, err = cache.SpaceUsedBySatellite(ctx, satelliteID)
			require.NoError(t, err, msg)
			assert.Equal(t, expPiecesTotal, int(piecesTotal), msg)
			assert.Equal(t, expPiecesContentSize, int(piecesContentSize), msg)
			trashTotal, err := cache.SpaceUsedForTrash(ctx)
			require.NoError(t, err, msg)
			assert.Equal(t, expTrash, int(trashTotal), msg)
		}

		expPieceSize := len(pieceContent) + pieces.V1PieceHeaderReservedArea

		assertValues("first write", satelliteID, expPieceSize*len(refs), len(pieceContent)*len(refs), 0)

		// Trash one piece
		blobInfo, err := cache.Stat(ctx, refs[0])
		require.NoError(t, err)
		fileInfo, err := blobInfo.Stat(ctx)
		require.NoError(t, err)
		ref0Size := fileInfo.Size()
		err = cache.Trash(ctx, refs[0])
		require.NoError(t, err)
		assertValues("trashed refs[0]", satelliteID, expPieceSize, len(pieceContent), int(ref0Size))

		// Restore one piece
		_, err = cache.RestoreTrash(ctx, satelliteID.Bytes())
		require.NoError(t, err)
		assertValues("restore trash for satellite", satelliteID, expPieceSize*len(refs), len(pieceContent)*len(refs), 0)

		// Trash piece again
		err = cache.Trash(ctx, refs[0])
		require.NoError(t, err)
		assertValues("trashed again", satelliteID, expPieceSize, len(pieceContent), int(ref0Size))

		// Empty trash
		_, _, err = cache.EmptyTrash(ctx, satelliteID.Bytes(), time.Now().Add(24*time.Hour))
		require.NoError(t, err)
		assertValues("emptied trash", satelliteID, expPieceSize, len(pieceContent), 0)

		// Delete that piece and confirm the cache is updated
		err = cache.Delete(ctx, refs[1])
		require.NoError(t, err)

		assertValues("delete item", satelliteID, 0, 0, 0)
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
			piecesTotal, _, err := sn.Storage2.BlobsCache.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			total += piecesTotal
			totalBySA1, _, err := sn.Storage2.BlobsCache.SpaceUsedBySatellite(ctx, satellite1.Identity.ID)
			require.NoError(t, err)
			total1 += totalBySA1
			totalBySA2, _, err := sn.Storage2.BlobsCache.SpaceUsedBySatellite(ctx, satellite2.Identity.ID)
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
			node.Storage2.BlobsCache.Update(ctx, satellite.ID(), 2000, 1000, 0)
			return nil
		})
		err := node.Storage2.CacheService.PersistCacheTotals(ctx)
		require.NoError(t, err)
		require.NoError(t, group.Wait())
	})
}
