// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

// TestGetPieceIDs does the following:
// * Create 90 pieces
// * Request 50 pieces starting from the beginning. Expect 50 pieces.
// * Request 50 pieces starting from the end of the previous request. Expect 40 pieces.
// * Request 50 pieces starting from the end of the previous request. Expect 0 pieces.
func TestGetPieceIDs(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		pieceInfos := db.PieceInfo()

		satellite := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())
		uplink := testidentity.MustPregeneratedSignedIdentity(3, storj.LatestIDVersion())
		totalPieces := 90
		for i := 0; i < totalPieces; i++ {
			newID := testrand.PieceID()

			pieceHash, err := signing.SignPieceHash(ctx,
				signing.SignerFromFullIdentity(uplink),
				&pb.PieceHash{
					PieceId: newID,
					Hash:    []byte{0, 2, 3, 4, 5},
				})
			require.NoError(t, err)

			err = pieceInfos.Add(ctx, &pieces.Info{
				SatelliteID:     satellite.ID,
				PieceSize:       4,
				PieceID:         newID,
				PieceCreation:   time.Now().Add(-time.Minute),
				UplinkPieceHash: pieceHash,
				OrderLimit:      &pb.OrderLimit{},
			})
			require.NoError(t, err)
		}

		seen := make(map[storj.PieceID]bool)

		requestSize := 50
		cursor := storj.PieceID{}

		pieceIDs, err := pieceInfos.GetPieceIDs(ctx, satellite.ID, time.Now(), requestSize, cursor)
		require.NoError(t, err)
		require.Len(t, pieceIDs, 50)
		for _, id := range pieceIDs {
			require.False(t, seen[id])
			seen[id] = true
			cursor = id
		}

		pieceIDs, err = pieceInfos.GetPieceIDs(ctx, satellite.ID, time.Now(), requestSize, cursor)
		require.NoError(t, err)
		require.Len(t, pieceIDs, 40)
		for _, id := range pieceIDs {
			require.False(t, seen[id])
			seen[id] = true
			cursor = id
		}

		pieceIDs, err = pieceInfos.GetPieceIDs(ctx, satellite.ID, time.Now(), requestSize, cursor)
		require.NoError(t, err)
		require.Len(t, pieceIDs, 0)
	})
}
