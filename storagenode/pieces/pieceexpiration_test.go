// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestPieceExpirationDB(t *testing.T) {
	// test GetExpired, SetExpiration, DeleteExpiration, DeleteFailed
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		expireDB := db.PieceExpirationDB()

		satelliteID := testrand.NodeID()
		pieceID := testrand.PieceID()
		expectedExpireInfo := pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID,
			InPieceInfo: false,
		}

		// GetExpired with no matches
		expiredPieceIDs, err := expireDB.GetExpired(ctx, time.Now(), 1000)
		require.NoError(t, err)
		require.Len(t, expiredPieceIDs, 0)

		// DeleteExpiration with no matches
		found, err := expireDB.DeleteExpiration(ctx, satelliteID, pieceID)
		require.NoError(t, err)
		require.False(t, found)

		// DeleteFailed with no matches
		err = expireDB.DeleteFailed(ctx, satelliteID, pieceID, time.Now())
		require.NoError(t, err)

		expireAt := time.Now()

		// SetExpiration normal usage
		err = expireDB.SetExpiration(ctx, satelliteID, pieceID, expireAt)
		require.NoError(t, err)

		// SetExpiration duplicate
		err = expireDB.SetExpiration(ctx, satelliteID, pieceID, expireAt.Add(time.Hour))
		require.Error(t, err)

		// GetExpired normal usage
		expiredPieceIDs, err = expireDB.GetExpired(ctx, expireAt.Add(time.Microsecond), 1000)
		require.NoError(t, err)
		require.Len(t, expiredPieceIDs, 1)
		assert.Equal(t, expiredPieceIDs[0], expectedExpireInfo)

		deleteFailedAt := expireAt.Add(2 * time.Microsecond)

		// DeleteFailed normal usage
		err = expireDB.DeleteFailed(ctx, satelliteID, pieceID, deleteFailedAt)
		require.NoError(t, err)

		// GetExpired filters out rows with deletion_failed_at = t
		expiredPieceIDs, err = expireDB.GetExpired(ctx, deleteFailedAt, 1000)
		require.NoError(t, err)
		require.Len(t, expiredPieceIDs, 0)
		expiredPieceIDs, err = expireDB.GetExpired(ctx, deleteFailedAt.Add(time.Microsecond), 1000)
		require.NoError(t, err)
		require.Len(t, expiredPieceIDs, 1)
		assert.Equal(t, expiredPieceIDs[0], expectedExpireInfo)

		// DeleteExpiration normal usage
		found, err = expireDB.DeleteExpiration(ctx, satelliteID, pieceID)
		require.NoError(t, err)
		require.True(t, found)

		// Should not be there anymore
		expiredPieceIDs, err = expireDB.GetExpired(ctx, expireAt.Add(365*24*time.Hour), 1000)
		require.NoError(t, err)
		require.Len(t, expiredPieceIDs, 0)
	})
}
