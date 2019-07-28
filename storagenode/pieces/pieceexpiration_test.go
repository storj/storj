// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/storagenode"
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

		// SetExpiration normal usage
		err = expireDB.SetExpiration(ctx, satelliteID, pieceID, time.Now())
		require.NoError(t, err)

		// SetExpiration duplicate
		err = expireDB.SetExpiration(ctx, satelliteID, pieceID, time.Now())
		require.Error(t, err)

		// GetExpired normal usage
		expiredPieceIDs, err = expireDB.GetExpired(ctx, time.Now(), 1000)
		require.NoError(t, err)
		require.Len(t, expiredPieceIDs, 1)
		assert.Equal(t, expiredPieceIDs[0].SatelliteID, satelliteID)
		assert.Equal(t, expiredPieceIDs[0].PieceID, pieceID)
		assert.False(t, expiredPieceIDs[0].InPieceInfo)
	})
}
