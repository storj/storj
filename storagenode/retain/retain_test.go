// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package retain_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/bloomfilter"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestRetainPieces(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		store := pieces.NewStore(zaptest.NewLogger(t), db.Pieces(), db.V0PieceInfo(), db.PieceExpirationDB(), db.PieceSpaceUsedDB())

		const numPieces = 1000
		const numPiecesToKeep = 990
		// pieces from numPiecesToKeep + numOldPieces to numPieces will
		// have a recent timestamp and thus should not be deleted
		const numOldPieces = 5

		// for this test, we set the false positive rate very low, so we can test which pieces should be deleted with precision
		filter := bloomfilter.NewOptimal(numPieces, 0.000000001)

		pieceIDs := generateTestIDs(numPieces)

		satellite0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())
		satellite1 := testidentity.MustPregeneratedSignedIdentity(2, storj.LatestIDVersion())

		uplink := testidentity.MustPregeneratedSignedIdentity(3, storj.LatestIDVersion())

		recentTime := time.Now()
		oldTime := recentTime.Add(-time.Duration(48) * time.Hour)

		// keep pieceIDs[0 : numPiecesToKeep] (old + in filter)
		// delete pieceIDs[numPiecesToKeep : numPiecesToKeep+numOldPieces] (old + not in filter)
		// keep pieceIDs[numPiecesToKeep+numOldPieces : numPieces] (recent + not in filter)
		var pieceCreation time.Time
		// add all pieces to the node pieces info DB - but only count piece ids in filter
		for index, id := range pieceIDs {
			if index < numPiecesToKeep {
				filter.Add(id)
			}

			if index < numPiecesToKeep+numOldPieces {
				pieceCreation = oldTime
			} else {
				pieceCreation = recentTime
			}

			piecehash0, err := signing.SignPieceHash(ctx,
				signing.SignerFromFullIdentity(uplink),
				&pb.PieceHash{
					PieceId: id,
					Hash:    []byte{0, 2, 3, 4, 5},
				})
			require.NoError(t, err)

			piecehash1, err := signing.SignPieceHash(ctx,
				signing.SignerFromFullIdentity(uplink),
				&pb.PieceHash{
					PieceId: id,
					Hash:    []byte{0, 2, 3, 4, 5},
				})
			require.NoError(t, err)

			pieceinfo0 := pieces.Info{
				SatelliteID:     satellite0.ID,
				PieceSize:       4,
				PieceID:         id,
				PieceCreation:   pieceCreation,
				UplinkPieceHash: piecehash0,
				OrderLimit:      &pb.OrderLimit{},
			}
			pieceinfo1 := pieces.Info{
				SatelliteID:     satellite1.ID,
				PieceSize:       4,
				PieceID:         id,
				PieceCreation:   pieceCreation,
				UplinkPieceHash: piecehash1,
				OrderLimit:      &pb.OrderLimit{},
			}

			v0db := store.GetV0PieceInfoDB().(pieces.V0PieceInfoDBForTest)
			err = v0db.Add(ctx, &pieceinfo0)
			require.NoError(t, err)

			err = v0db.Add(ctx, &pieceinfo1)
			require.NoError(t, err)

		}

		retainEnabled := retain.NewService(zaptest.NewLogger(t), store, retain.Config{
			RetainStatus:        retain.Enabled,
			MaxConcurrentRetain: 1,
			RetainTimeBuffer:    0,
		})
		retainDisabled := retain.NewService(zaptest.NewLogger(t), store, retain.Config{
			RetainStatus:        retain.Disabled,
			MaxConcurrentRetain: 1,
			RetainTimeBuffer:    0,
		})
		retainDebug := retain.NewService(zaptest.NewLogger(t), store, retain.Config{
			RetainStatus:        retain.Debug,
			MaxConcurrentRetain: 1,
			RetainTimeBuffer:    0,
		})

		// start the retain services
		var group errgroup.Group
		ctx2, cancel := context.WithCancel(ctx)
		group.Go(func() error {
			return retainEnabled.Run(ctx2)
		})
		group.Go(func() error {
			return retainDisabled.Run(ctx2)
		})
		group.Go(func() error {
			return retainDebug.Run(ctx2)
		})

		// expect that disabled and debug endpoints do not delete any pieces
		req := retain.Request{
			SatelliteID:   satellite0.ID,
			CreatedBefore: recentTime,
			Filter:        filter,
		}
		queued := retainDisabled.Queue(req)
		require.True(t, queued)
		retainDisabled.Wait(ctx2)

		queued = retainDebug.Queue(req)
		require.True(t, queued)
		retainDebug.Wait(ctx2)

		satellite1Pieces, err := getAllPieceIDs(ctx, store, satellite1.ID, recentTime.Add(time.Duration(5)*time.Second))
		require.NoError(t, err)
		require.Equal(t, numPieces, len(satellite1Pieces))

		satellite0Pieces, err := getAllPieceIDs(ctx, store, satellite0.ID, recentTime.Add(time.Duration(5)*time.Second))
		require.NoError(t, err)
		require.Equal(t, numPieces, len(satellite0Pieces))

		// expect that enabled endpoint deletes the correct pieces
		queued = retainEnabled.Queue(req)
		require.True(t, queued)
		retainEnabled.Wait(ctx2)

		// check we have deleted nothing for satellite1
		satellite1Pieces, err = getAllPieceIDs(ctx, store, satellite1.ID, recentTime.Add(time.Duration(5)*time.Second))
		require.NoError(t, err)
		require.Equal(t, numPieces, len(satellite1Pieces))

		// check we did not delete recent pieces or retained pieces for satellite0
		// also check that we deleted the correct pieces for satellite0
		satellite0Pieces, err = getAllPieceIDs(ctx, store, satellite0.ID, recentTime.Add(time.Duration(5)*time.Second))
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
	})
}

func getAllPieceIDs(ctx context.Context, store *pieces.Store, satellite storj.NodeID, createdBefore time.Time) (pieceIDs []storj.PieceID, err error) {
	err = store.WalkSatellitePieces(ctx, satellite, func(pieceAccess pieces.StoredPieceAccess) error {
		mTime, err := pieceAccess.CreationTime(ctx)
		if err != nil {
			return err
		}
		if !mTime.Before(createdBefore) {
			return nil
		}
		pieceIDs = append(pieceIDs, pieceAccess.PieceID())
		return nil
	})
	return pieceIDs, err
}

// generateTestIDs generates n piece ids
func generateTestIDs(n int) []storj.PieceID {
	ids := make([]storj.PieceID, n)
	for i := range ids {
		ids[i] = testrand.PieceID()
	}
	return ids
}
