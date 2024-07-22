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
	"storj.io/storj/storagenode/storagenodedb"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestPieceExpirationDB(t *testing.T) {
	// test GetExpired, SetExpiration, DeleteExpiration, DeleteFailed
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		expireDB := db.PieceExpirationDB()

		satelliteID := testrand.NodeID()
		pieceID := testrand.PieceID()

		// GetExpired with no matches
		expired, err := expireDB.GetExpired(ctx, time.Now(), -1)
		require.NoError(t, err)
		require.Len(t, expired, 0)

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
		expired, err = expireDB.GetExpired(ctx, expireAt, -1)
		require.NoError(t, err)
		require.Len(t, expired, 1)

		// DeleteExpiration normal usage
		err = expireDB.DeleteExpirations(ctx, expireAt)
		require.NoError(t, err)

		// Should not be there anymore
		expired, err = expireDB.GetExpired(ctx, expireAt.Add(365*24*time.Hour), -1)
		require.NoError(t, err)
		require.Len(t, expired, 0)
	})
}

func TestPieceExpirationDB_noBuffering(t *testing.T) {
	// test GetExpired, SetExpiration, DeleteExpirations bypassing the buffer
	// so that the database is used directly
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		storagenodedb.MaxPieceExpirationBufferSize = 0
		expireDB := db.PieceExpirationDB()

		satelliteID := testrand.NodeID()
		pieceID := testrand.PieceID()

		// GetExpired with no matches
		expired, err := expireDB.GetExpired(ctx, time.Now(), -1)
		require.NoError(t, err)
		require.Len(t, expired, 0)

		// DeleteExpiration with no matches
		err = expireDB.DeleteExpirations(ctx, time.Time{})
		require.NoError(t, err)

		expireAt := time.Now()

		// SetExpiration
		err = expireDB.SetExpiration(ctx, satelliteID, pieceID, expireAt.Add(-time.Hour))
		require.NoError(t, err)

		// GetExpired normal usage
		expired, err = expireDB.GetExpired(ctx, expireAt, -1)
		require.NoError(t, err)
		require.Len(t, expired, 1)

		// DeleteExpiration normal usage
		err = expireDB.DeleteExpirations(ctx, expireAt)
		require.NoError(t, err)

		// Should not be there anymore
		expired, err = expireDB.GetExpired(ctx, expireAt.Add(365*24*time.Hour), -1)
		require.NoError(t, err)
		require.Len(t, expired, 0)

		// let's add a few more
		var expectedExpired []pieces.ExpiredInfo
		randSetExpiration := func(ctx context.Context, expireAt time.Time) {
			ei := pieces.ExpiredInfo{
				SatelliteID: testrand.NodeID(),
				PieceID:     testrand.PieceID(),
			}
			err = expireDB.SetExpiration(ctx, ei.SatelliteID, ei.PieceID, expireAt.UTC())
			require.NoError(t, err)
			// setting it in the order in which the database will return it
			expectedExpired = append([]pieces.ExpiredInfo{ei}, expectedExpired...)
		}
		num := 0
		for num < 10 {
			num++
			randSetExpiration(ctx, expireAt.Add(-time.Duration(num)*time.Hour))
		}

		// GetExpired batch
		expired, err = expireDB.GetExpired(ctx, expireAt, 1)
		require.NoError(t, err)
		require.Len(t, expired, 1)
		require.Equal(t, expectedExpired[:1], expired)

		expired, err = expireDB.GetExpired(ctx, expireAt, 3)
		require.NoError(t, err)
		require.Len(t, expired, 3)
		require.Equal(t, expectedExpired[:3], expired)

		expired, err = expireDB.GetExpired(ctx, expireAt, 10)
		require.NoError(t, err)
		require.Len(t, expired, 10)
		require.Equal(t, expectedExpired, expired)

		// DeleteExpiration batch
		err = expireDB.DeleteExpirationsBatch(ctx, expireAt, 5)
		require.NoError(t, err)
		// 5 old records should be gone
		expired, err = expireDB.GetExpired(ctx, expireAt, 10)
		require.NoError(t, err)
		require.Len(t, expired, 5)
		require.Equal(t, expectedExpired[5:], expired)
	})
}
