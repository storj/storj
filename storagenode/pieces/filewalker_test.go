// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestFilewalker_Basic(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		observedZapCore, observedLogs := observer.New(zap.DebugLevel)
		observedLogger := zap.New(observedZapCore).Named("filewalker")

		blobs := db.Pieces()
		v0PieceInfo := db.V0PieceInfo()
		fw := pieces.NewFileWalker(observedLogger, blobs, v0PieceInfo, db.GCFilewalkerProgress())
		store := pieces.NewStore(observedLogger, fw, nil, blobs, v0PieceInfo, db.PieceExpirationDB(), db.PieceSpaceUsedDB(), pieces.DefaultConfig)
		testStore := pieces.StoreForTest{Store: store}

		numberOfPieces := 100

		const size = 5 * memory.KiB

		satellite := testrand.NodeID()

		for i := 0; i < numberOfPieces; i++ {
			now := time.Now()
			pieceID := testrand.PieceID()
			w, err := testStore.WriterForFormatVersion(ctx, satellite, pieceID, filestore.FormatV1, pb.PieceHashAlgorithm_SHA256)
			require.NoError(t, err)

			_, err = w.Write(testrand.Bytes(size))
			require.NoError(t, err)

			require.NoError(t, w.Commit(ctx, &pb.PieceHeader{
				CreationTime: now,
			}))
		}

		filter := bloomfilter.NewOptimal(int64(numberOfPieces), 0.000000001)

		// WalkAndComputeSpaceUsedBySatellite
		total, totalContentSize, _, err := fw.WalkAndComputeSpaceUsedBySatellite(ctx, satellite)
		require.NoError(t, err)
		require.Equal(t, int64(numberOfPieces)*size.Int64(), totalContentSize)
		require.GreaterOrEqual(t, total, int64(numberOfPieces)*size.Int64())

		// WalkSatellitePieces
		count := 0
		numOfTrashPieces := 0
		err = store.WalkSatellitePieces(ctx, satellite, func(pieceAccess pieces.StoredPieceAccess) error {
			count++
			if count%2 == 0 {
				filter.Add(pieceAccess.PieceID())
				numOfTrashPieces++
			}
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, numberOfPieces, count)

		// WalkSatellitePiecesToTrash
		trashPieceCount := 0
		piecesCount, _, err := fw.WalkSatellitePiecesToTrash(ctx, satellite, time.Now(), filter, func(pieceID storj.PieceID) error {
			trashPieceCount++
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, int64(numberOfPieces), piecesCount)
		require.Equal(t, numOfTrashPieces, trashPieceCount)

		// check for the logs
		require.Equal(t, 0, observedLogs.FilterMessage("failed to get progress from database").Len())
		require.Equal(t, 0, observedLogs.FilterMessage("failed to store progress to database").Len())
		require.Equal(t, 0, observedLogs.FilterMessage("bloomfilter createdBefore time does not match the one used in the last scan").Len())
		require.Equal(t, 0, observedLogs.FilterMessage("failed to reset progress in database").Len())
		require.Equal(t, 1, observedLogs.FilterMessage("resetting progress in database").Len())
		require.Equal(t, 0, observedLogs.FilterMessage("failed to determine mtime of blob").Len())
	})
}
