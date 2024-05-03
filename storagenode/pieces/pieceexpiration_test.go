// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestPieceExpirationDB(t *testing.T) {
	// test GetExpired, SetExpiration, DeleteExpiration, DeleteFailed
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		expireDB := db.PieceExpirationDB()

		satelliteID := testrand.NodeID()
		pieceID := testrand.PieceID()

		// GetExpired with no matches
		err := expireDB.GetExpired(ctx, time.Now(), func(_ context.Context, ei pieces.ExpiredInfo) bool {
			t.Fatal("should not be called")
			return false
		})
		require.NoError(t, err)

		// DeleteExpiration with no matches
		err = expireDB.DeleteExpirations(ctx, time.Time{})
		require.NoError(t, err)

		expireAt := time.Now()

		// SetExpiration normal usage
		err = expireDB.SetExpiration(ctx, satelliteID, pieceID, expireAt)
		require.NoError(t, err)

		// SetExpiration duplicate
		err = expireDB.SetExpiration(ctx, satelliteID, pieceID, expireAt.Add(-time.Hour))
		require.NoError(t, err)

		// GetExpired normal usage
		var expired []pieces.ExpiredInfo
		err = expireDB.GetExpired(ctx, expireAt, func(_ context.Context, ei pieces.ExpiredInfo) bool {
			expired = append(expired, ei)
			return true
		})
		require.NoError(t, err)
		require.Len(t, expired, 1)

		// DeleteExpiration normal usage
		err = expireDB.DeleteExpirations(ctx, expireAt)
		require.NoError(t, err)

		// Should not be there anymore
		err = expireDB.GetExpired(ctx, expireAt.Add(365*24*time.Hour), func(_ context.Context, ei pieces.ExpiredInfo) bool {
			t.Fatal("should not be called")
			return false
		})
		require.NoError(t, err)
	})
}
