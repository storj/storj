// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
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
		fw := pieces.NewFileWalker(observedLogger, blobs, v0PieceInfo, db.GCFilewalkerProgress(), db.UsedSpacePerPrefix())
		store := pieces.NewStore(observedLogger, fw, nil, blobs, v0PieceInfo, db.PieceExpirationDB(), pieces.DefaultConfig)
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

func TestWalkAndComputeSpaceUsedBySatellite_resume(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		logger := zaptest.NewLogger(t)

		blobs := db.Pieces()
		v0PieceInfo := db.V0PieceInfo()
		fw := pieces.NewFileWalker(logger, blobs, v0PieceInfo, db.GCFilewalkerProgress(), db.UsedSpacePerPrefix())
		store := pieces.NewStore(logger, fw, nil, blobs, v0PieceInfo, db.PieceExpirationDB(), pieces.DefaultConfig)
		testStore := pieces.StoreForTest{Store: store}

		numberOfPieces := 100 // this will be one piece per directory
		const size = 5 * memory.KiB
		expectedTotal := int64(numberOfPieces) * size.Int64()

		satellite := testrand.NodeID()

		// let's create pieces, one for each directory
		for i := 0; i < numberOfPieces; i++ {
			now := time.Now()
			pieceID := numToPieceID(uint16(i))
			w, err := testStore.WriterForFormatVersion(ctx, satellite, pieceID, filestore.FormatV1, pb.PieceHashAlgorithm_SHA256)
			require.NoError(t, err)

			_, err = w.Write(testrand.Bytes(size))
			require.NoError(t, err)

			require.NoError(t, w.Commit(ctx, &pb.PieceHeader{
				CreationTime: now,
			}))
		}

		fwError := errors.New("interrupt")
		numOfPiecesScannedAtFirstWalk := 0
		total, totalContentSize, _, err := fw.WalkAndComputeSpaceUsedBySatelliteWithWalkFunc(ctx, satellite, func(access pieces.StoredPieceAccess) error {
			if numOfPiecesScannedAtFirstWalk >= numberOfPieces/2 {
				// intentionally return an error to end the walk
				return fwError
			}
			numOfPiecesScannedAtFirstWalk++
			return nil
		})
		require.ErrorIs(t, err, fwError)
		require.Equal(t, numberOfPieces/2, numOfPiecesScannedAtFirstWalk)
		require.Equal(t, int64(numOfPiecesScannedAtFirstWalk)*size.Int64(), totalContentSize)
		require.GreaterOrEqual(t, total, totalContentSize)

		numOfPiecesScannedAtSecondWalk := 0
		// let's resume the walk
		total, totalContentSize, piecesCount, err := fw.WalkAndComputeSpaceUsedBySatelliteWithWalkFunc(ctx, satellite, func(access pieces.StoredPieceAccess) error {
			numOfPiecesScannedAtSecondWalk++
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, expectedTotal, totalContentSize)
		require.GreaterOrEqual(t, total, totalContentSize)
		// +1 because the last prefix during the first walk would not be stored due to the error
		// and that will be the starting point for the second walk
		expectedNumOfPiecesScannedAtSecondWalk := (numberOfPieces - numOfPiecesScannedAtFirstWalk) + 1
		require.Equal(t, expectedNumOfPiecesScannedAtSecondWalk, numOfPiecesScannedAtSecondWalk)
		require.Equal(t, int64(numberOfPieces), piecesCount)
	})
}
func numToPieceID(n uint16) storj.PieceID {
	var b [32]byte
	binary.BigEndian.PutUint16(b[:], n<<6)
	return storj.PieceID(b[:])
}
