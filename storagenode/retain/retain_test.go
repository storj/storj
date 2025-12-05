// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package retain_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/identity/testidentity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/cmd/storagenode/internalcmd"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestRetainPieces(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		log := zaptest.NewLogger(t)
		blobs := pieces.NewBlobsUsageCache(log, db.Pieces())
		v0PieceInfo := db.V0PieceInfo()
		fw := pieces.NewFileWalker(log, blobs, v0PieceInfo, db.GCFilewalkerProgress(), db.UsedSpacePerPrefix())
		store := pieces.NewStore(log, fw, nil, blobs, v0PieceInfo, db.PieceExpirationDB(), pieces.DefaultConfig)
		testStore := pieces.StoreForTest{Store: store}

		const numPieces = 100
		const numPiecesToKeep = 95
		// pieces from numPiecesToKeep + numOldPieces to numPieces will
		// have a recent timestamp and thus should not be deleted
		const numOldPieces = 5

		// for this test, we set the false positive rate very low, so we can test which pieces should be deleted with precision
		filter := bloomfilter.NewOptimal(numPieces, 0.000000001)

		pieceIDs := generateTestIDs(numPieces)

		satellite0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())
		satellite1 := testidentity.MustPregeneratedSignedIdentity(2, storj.LatestIDVersion())

		uplink := testidentity.MustPregeneratedSignedIdentity(3, storj.LatestIDVersion())

		// keep pieceIDs[0 : numPiecesToKeep] (old + in filter)
		// delete pieceIDs[numPiecesToKeep : numPiecesToKeep+numOldPieces] (old + not in filter)
		// keep pieceIDs[numPiecesToKeep+numOldPieces : numPieces] (recent + not in filter)
		// add all pieces to the node pieces info DB - but only count piece ids in filter
		for index, id := range pieceIDs {
			var formatVer blobstore.FormatVersion
			if index%2 == 0 {
				formatVer = filestore.FormatV0
			} else {
				formatVer = filestore.FormatV1
			}

			if index < numPiecesToKeep {
				filter.Add(id)
			}

			const size = 100 * memory.B

			// Write file for all satellites
			for _, satelliteID := range []storj.NodeID{satellite0.ID, satellite1.ID} {
				now := time.Now()
				w, err := testStore.WriterForFormatVersion(ctx, satelliteID, id, formatVer, pb.PieceHashAlgorithm_SHA256)
				require.NoError(t, err)

				_, err = w.Write(testrand.Bytes(size))
				require.NoError(t, err)

				require.NoError(t, w.Commit(ctx, &pb.PieceHeader{
					CreationTime: now,
				}))

				piecehash, err := signing.SignPieceHash(ctx,
					signing.SignerFromFullIdentity(uplink),
					&pb.PieceHash{
						PieceId: id,
						Hash:    []byte{0, 2, 3, 4, 5},
					})
				require.NoError(t, err)

				if formatVer == filestore.FormatV0 {
					v0db := testStore.GetV0PieceInfoDBForTest()
					err = v0db.Add(ctx, &pieces.Info{
						SatelliteID:     satelliteID,
						PieceSize:       4,
						PieceID:         id,
						PieceCreation:   now,
						UplinkPieceHash: piecehash,
						OrderLimit:      &pb.OrderLimit{},
					})
					require.NoError(t, err)
				}
			}
		}

		usedForTrash, err := blobs.SpaceUsedForTrash(ctx)
		require.NoError(t, err)
		require.Zero(t, usedForTrash)

		retainCachePath := ctx.Dir("retain")
		retainStoreCachePath := ctx.Dir("retain-store")

		retainEnabled := retain.NewService(zaptest.NewLogger(t), store, retain.Config{
			Status:      retain.Enabled,
			Concurrency: 1,
			MaxTimeSkew: 0,
			CachePath:   retainCachePath,
		})

		retainDisabled := retain.NewService(zaptest.NewLogger(t), store, retain.Config{
			Status:      retain.Disabled,
			Concurrency: 1,
			MaxTimeSkew: 0,
			CachePath:   retainCachePath,
		})

		retainDebug := retain.NewService(zaptest.NewLogger(t), store, retain.Config{
			Status:      retain.Debug,
			Concurrency: 1,
			MaxTimeSkew: 0,
			CachePath:   retainCachePath,
		})

		retainStore := retain.NewService(zaptest.NewLogger(t), store, retain.Config{
			Status:      retain.Store,
			Concurrency: 1,
			MaxTimeSkew: 0,
			CachePath:   retainStoreCachePath,
		})

		// start the retain services
		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		var group errgroup.Group
		group.Go(func() error {
			return retainEnabled.Run(runCtx)
		})
		group.Go(func() error {
			return retainDisabled.Run(runCtx)
		})
		group.Go(func() error {
			return retainDebug.Run(runCtx)
		})
		group.Go(func() error {
			return retainStore.Run(runCtx)
		})

		// expect that disabled and debug endpoints do not delete any pieces
		req := &pb.RetainRequest{
			CreationDate: time.Now(),
			Filter:       filter.Bytes(),
		}

		satellite1Pieces, err := getAllPieceIDs(ctx, store, satellite1.ID)
		require.NoError(t, err)
		require.Equal(t, numPieces, len(satellite1Pieces))

		satellite0Pieces, err := getAllPieceIDs(ctx, store, satellite0.ID)
		require.NoError(t, err)
		require.Equal(t, numPieces, len(satellite0Pieces))

		{
			err = retainDisabled.Queue(ctx, satellite0.ID, req)
			require.NoError(t, err)
			retainDisabled.TestWaitUntilEmpty()

			// check we have deleted nothing for satellite0
			piecesAfter, err := getAllPieceIDs(ctx, store, satellite0.ID)
			require.NoError(t, err)
			require.Equal(t, numPieces, len(piecesAfter))
		}

		{
			err = retainStore.Queue(ctx, satellite0.ID, req)
			require.NoError(t, err)

			// check we have deleted nothing for satellite0
			piecesAfter, err := getAllPieceIDs(ctx, store, satellite0.ID)
			require.NoError(t, err)
			require.Equal(t, numPieces, len(piecesAfter))

			entries, err := os.ReadDir(retainStoreCachePath)
			require.NoError(t, err)
			require.Len(t, entries, 1)
		}

		err = retainDebug.Queue(ctx, satellite0.ID, req)
		require.NoError(t, err)
		retainDebug.TestWaitUntilEmpty()

		// expect that enabled endpoint deletes the correct pieces
		err = retainEnabled.Queue(ctx, satellite0.ID, req)
		require.NoError(t, err)
		retainEnabled.TestWaitUntilEmpty()

		// check we have deleted nothing for satellite1
		satellite1Pieces, err = getAllPieceIDs(ctx, store, satellite1.ID)
		require.NoError(t, err)
		require.Equal(t, numPieces, len(satellite1Pieces))

		// check we did not delete recent pieces or retained pieces for satellite0
		// also check that we deleted the correct pieces for satellite0
		satellite0Pieces, err = getAllPieceIDs(ctx, store, satellite0.ID)
		require.NoError(t, err)
		require.Equal(t, numPieces-numOldPieces, len(satellite0Pieces))

		for _, id := range pieceIDs[:numPiecesToKeep] {
			require.Contains(t, satellite0Pieces, id, "piece should not have been deleted (not in bloom filter)")
		}

		for _, id := range pieceIDs[numPiecesToKeep+numOldPieces:] {
			require.Contains(t, satellite0Pieces, id, "piece should not have been deleted (recent piece)")
		}

		for _, id := range pieceIDs[numPiecesToKeep : numPiecesToKeep+numOldPieces] {
			require.NotContains(t, satellite0Pieces, id, "piece should have been deleted")
		}

		// shut down retain services
		cancel()
		err = group.Wait()
		require.True(t, errs2.IsCanceled(err))

		usedForTrash, err = blobs.SpaceUsedForTrash(ctx)
		require.NoError(t, err)
		require.NotZero(t, usedForTrash)
	})
}

func TestRetainPieces_lazyFilewalker(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		log := zaptest.NewLogger(t)
		blobs := pieces.NewBlobsUsageCache(log, db.Pieces())
		v0PieceInfo := db.V0PieceInfo()
		fw := pieces.NewFileWalker(log, blobs, v0PieceInfo, db.GCFilewalkerProgress(), db.UsedSpacePerPrefix())
		cfg := pieces.DefaultConfig
		cfg.EnableLazyFilewalker = true

		lazyFwCfg := db.Config().LazyFilewalkerConfig()
		lazyFw := lazyfilewalker.NewSupervisor(log, lazyFwCfg, "")
		cmd := internalcmd.NewGCFilewalkerCmd()
		cmd.Logger = log.Named("gc-filewalker")
		cmd.Ctx = ctx
		lazyFw.TestingSetGCCmd(cmd)
		store := pieces.NewStore(log, fw, lazyFw, blobs, v0PieceInfo, db.PieceExpirationDB(), cfg)
		testStore := pieces.StoreForTest{Store: store}

		const numPieces = 100
		const numPiecesToKeep = 95
		// pieces from numPiecesToKeep + numOldPieces to numPieces will
		// have a recent timestamp and thus should not be deleted
		const numOldPieces = 5

		// for this test, we set the false positive rate very low, so we can test which pieces should be deleted with precision
		filter := bloomfilter.NewOptimal(numPieces, 0.000000001)

		pieceIDs := generateTestIDs(numPieces)

		satellite0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())
		satellite1 := testidentity.MustPregeneratedSignedIdentity(2, storj.LatestIDVersion())

		uplink := testidentity.MustPregeneratedSignedIdentity(3, storj.LatestIDVersion())

		// keep pieceIDs[0 : numPiecesToKeep] (old + in filter)
		// delete pieceIDs[numPiecesToKeep : numPiecesToKeep+numOldPieces] (old + not in filter)
		// keep pieceIDs[numPiecesToKeep+numOldPieces : numPieces] (recent + not in filter)
		// add all pieces to the node pieces info DB - but only count piece ids in filter
		for index, id := range pieceIDs {
			var formatVer blobstore.FormatVersion
			if index%2 == 0 {
				formatVer = filestore.FormatV0
			} else {
				formatVer = filestore.FormatV1
			}

			if index < numPiecesToKeep {
				filter.Add(id)
			}

			const size = 100 * memory.B

			// Write file for all satellites
			for _, satelliteID := range []storj.NodeID{satellite0.ID, satellite1.ID} {
				now := time.Now()
				w, err := testStore.WriterForFormatVersion(ctx, satelliteID, id, formatVer, pb.PieceHashAlgorithm_SHA256)
				require.NoError(t, err)

				_, err = w.Write(testrand.Bytes(size))
				require.NoError(t, err)

				require.NoError(t, w.Commit(ctx, &pb.PieceHeader{
					CreationTime: now,
				}))

				piecehash, err := signing.SignPieceHash(ctx,
					signing.SignerFromFullIdentity(uplink),
					&pb.PieceHash{
						PieceId: id,
						Hash:    []byte{0, 2, 3, 4, 5},
					})
				require.NoError(t, err)

				if formatVer == filestore.FormatV0 {
					v0db := testStore.GetV0PieceInfoDBForTest()
					err = v0db.Add(ctx, &pieces.Info{
						SatelliteID:     satelliteID,
						PieceSize:       4,
						PieceID:         id,
						PieceCreation:   now,
						UplinkPieceHash: piecehash,
						OrderLimit:      &pb.OrderLimit{},
					})
					require.NoError(t, err)
				}
			}
		}

		usedForTrash, err := blobs.SpaceUsedForTrash(ctx)
		require.NoError(t, err)
		require.Zero(t, usedForTrash)

		retainEnabled := retain.NewService(zaptest.NewLogger(t), store, retain.Config{
			Status:      retain.Enabled,
			Concurrency: 1,
			MaxTimeSkew: 0,
			CachePath:   ctx.Dir("retain"),
		})

		// start the retain services
		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		var group errgroup.Group
		group.Go(func() error {
			return retainEnabled.Run(runCtx)
		})

		// expect that disabled and debug endpoints do not delete any pieces
		req := &pb.RetainRequest{
			CreationDate: time.Now(),
			Filter:       filter.Bytes(),
		}

		// expect that enabled endpoint deletes the correct pieces
		err = retainEnabled.Queue(ctx, satellite0.ID, req)
		require.NoError(t, err)
		retainEnabled.TestWaitUntilEmpty()

		// check we have deleted nothing for satellite1
		satellite1Pieces, err := getAllPieceIDs(ctx, store, satellite1.ID)
		require.NoError(t, err)
		require.Equal(t, numPieces, len(satellite1Pieces))

		// check we did not delete recent pieces or retained pieces for satellite0
		// also check that we deleted the correct pieces for satellite0
		satellite0Pieces, err := getAllPieceIDs(ctx, store, satellite0.ID)
		require.NoError(t, err)
		require.Equal(t, numPieces-numOldPieces, len(satellite0Pieces))

		for _, id := range pieceIDs[:numPiecesToKeep] {
			require.Contains(t, satellite0Pieces, id, "piece should not have been deleted (not in bloom filter)")
		}

		for _, id := range pieceIDs[numPiecesToKeep+numOldPieces:] {
			require.Contains(t, satellite0Pieces, id, "piece should not have been deleted (recent piece)")
		}

		for _, id := range pieceIDs[numPiecesToKeep : numPiecesToKeep+numOldPieces] {
			require.NotContains(t, satellite0Pieces, id, "piece should have been deleted")
		}

		// shut down retain services
		cancel()
		err = group.Wait()
		require.True(t, errs2.IsCanceled(err))

		usedForTrash, err = blobs.SpaceUsedForTrash(ctx)
		require.NoError(t, err)
		require.NotZero(t, usedForTrash)
	})
}

func TestRetainPieces_fromStore(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		log := zaptest.NewLogger(t)
		blobs := pieces.NewBlobsUsageCache(log, db.Pieces())
		v0PieceInfo := db.V0PieceInfo()
		fw := pieces.NewFileWalker(log, blobs, v0PieceInfo, db.GCFilewalkerProgress(), db.UsedSpacePerPrefix())
		cfg := pieces.DefaultConfig
		cfg.EnableLazyFilewalker = true

		lazyFwCfg := db.Config().LazyFilewalkerConfig()
		lazyFw := lazyfilewalker.NewSupervisor(log, lazyFwCfg, "")
		cmd := internalcmd.NewGCFilewalkerCmd()
		cmd.Logger = log.Named("gc-filewalker")
		cmd.Ctx = ctx
		lazyFw.TestingSetGCCmd(cmd)
		store := pieces.NewStore(log, fw, lazyFw, blobs, v0PieceInfo, db.PieceExpirationDB(), cfg)
		testStore := pieces.StoreForTest{Store: store}

		const numPieces = 100
		const numPiecesToKeep = 95
		// pieces from numPiecesToKeep + numOldPieces to numPieces will
		// have a recent timestamp and thus should not be deleted
		const numOldPieces = 5

		// for this test, we set the false positive rate very low, so we can test which pieces should be deleted with precision
		filter := bloomfilter.NewOptimal(numPieces, 0.000000001)

		pieceIDs := generateTestIDs(numPieces)

		satellite0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())
		satellite1 := testidentity.MustPregeneratedSignedIdentity(2, storj.LatestIDVersion())

		uplink := testidentity.MustPregeneratedSignedIdentity(3, storj.LatestIDVersion())

		// keep pieceIDs[0 : numPiecesToKeep] (old + in filter)
		// delete pieceIDs[numPiecesToKeep : numPiecesToKeep+numOldPieces] (old + not in filter)
		// keep pieceIDs[numPiecesToKeep+numOldPieces : numPieces] (recent + not in filter)
		// add all pieces to the node pieces info DB - but only count piece ids in filter
		for index, id := range pieceIDs {
			var formatVer blobstore.FormatVersion
			if index%2 == 0 {
				formatVer = filestore.FormatV0
			} else {
				formatVer = filestore.FormatV1
			}

			if index < numPiecesToKeep {
				filter.Add(id)
			}

			const size = 100 * memory.B

			// Write file for all satellites
			for _, satelliteID := range []storj.NodeID{satellite0.ID, satellite1.ID} {
				now := time.Now()
				w, err := testStore.WriterForFormatVersion(ctx, satelliteID, id, formatVer, pb.PieceHashAlgorithm_SHA256)
				require.NoError(t, err)

				_, err = w.Write(testrand.Bytes(size))
				require.NoError(t, err)

				require.NoError(t, w.Commit(ctx, &pb.PieceHeader{
					CreationTime: now,
				}))

				piecehash, err := signing.SignPieceHash(ctx,
					signing.SignerFromFullIdentity(uplink),
					&pb.PieceHash{
						PieceId: id,
						Hash:    []byte{0, 2, 3, 4, 5},
					})
				require.NoError(t, err)

				if formatVer == filestore.FormatV0 {
					v0db := testStore.GetV0PieceInfoDBForTest()
					err = v0db.Add(ctx, &pieces.Info{
						SatelliteID:     satelliteID,
						PieceSize:       4,
						PieceID:         id,
						PieceCreation:   now,
						UplinkPieceHash: piecehash,
						OrderLimit:      &pb.OrderLimit{},
					})
					require.NoError(t, err)
				}
			}
		}

		usedForTrash, err := blobs.SpaceUsedForTrash(ctx)
		require.NoError(t, err)
		require.Zero(t, usedForTrash)

		retainDir := ctx.Dir("retain")
		req := retain.Request{
			SatelliteID:   satellite0.ID,
			CreatedBefore: time.Now(),
			Filter:        filter,
		}

		// save the request to the store
		err = retain.SaveRequest(retainDir, req.GetFilename(), &pb.RetainRequest{
			CreationDate: req.CreatedBefore,
			Filter:       req.Filter.Bytes(),
		})
		require.NoError(t, err)

		retainEnabled := retain.NewService(zaptest.NewLogger(t), store, retain.Config{
			Status:      retain.Enabled,
			Concurrency: 1,
			MaxTimeSkew: 0,
			CachePath:   ctx.Dir("retain"),
		})

		// start the retain services
		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		var group errgroup.Group
		group.Go(func() error {
			return retainEnabled.Run(runCtx)
		})

		// wait till queue is empty
		retainEnabled.TestWaitUntilEmpty()

		// check we have deleted nothing for satellite1
		satellite1Pieces, err := getAllPieceIDs(ctx, store, satellite1.ID)
		require.NoError(t, err)
		require.Equal(t, numPieces, len(satellite1Pieces))

		// check we did not delete recent pieces or retained pieces for satellite0
		// also check that we deleted the correct pieces for satellite0
		satellite0Pieces, err := getAllPieceIDs(ctx, store, satellite0.ID)
		require.NoError(t, err)
		require.Equal(t, numPieces-numOldPieces, len(satellite0Pieces))

		for _, id := range pieceIDs[:numPiecesToKeep] {
			require.Contains(t, satellite0Pieces, id, "piece should not have been deleted (not in bloom filter)")
		}

		for _, id := range pieceIDs[numPiecesToKeep+numOldPieces:] {
			require.Contains(t, satellite0Pieces, id, "piece should not have been deleted (recent piece)")
		}

		for _, id := range pieceIDs[numPiecesToKeep : numPiecesToKeep+numOldPieces] {
			require.NotContains(t, satellite0Pieces, id, "piece should have been deleted")
		}

		// shut down retain services
		cancel()
		err = group.Wait()
		require.True(t, errs2.IsCanceled(err))

		usedForTrash, err = blobs.SpaceUsedForTrash(ctx)
		require.NoError(t, err)
		require.NotZero(t, usedForTrash)
	})
}

func getAllPieceIDs(ctx context.Context, store *pieces.Store, satellite storj.NodeID) (pieceIDs []storj.PieceID, err error) {
	err = store.WalkSatellitePieces(ctx, satellite, func(pieceAccess pieces.StoredPieceAccess) error {
		pieceIDs = append(pieceIDs, pieceAccess.PieceID())
		return nil
	})
	return pieceIDs, err
}

// generateTestIDs generates n piece ids.
func generateTestIDs(n int) []storj.PieceID {
	ids := make([]storj.PieceID, n)
	for i := range ids {
		ids[i] = testrand.PieceID()
	}
	return ids
}
