// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestPieceExpirationDB(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		expireDB := db.PieceExpirationDB()
		pieces.PieceExpirationFunctionalityTest(ctx, t, expireDB)
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
		expiredLists, err := expireDB.GetExpired(ctx, time.Now(), pieces.DefaultExpirationLimits())
		require.NoError(t, err)
		expired := pieces.FlattenExpirationInfoLists(expiredLists)
		require.Len(t, expired, 0)

		// DeleteExpiration with no matches
		err = expireDB.DeleteExpirations(ctx, time.Time{})
		require.NoError(t, err)

		expireAt := time.Now()

		// SetExpiration
		err = expireDB.SetExpiration(ctx, satelliteID, pieceID, expireAt.Add(-time.Hour), 0)
		require.NoError(t, err)

		// GetExpired normal usage
		expiredLists, err = expireDB.GetExpired(ctx, expireAt, pieces.DefaultExpirationLimits())
		require.NoError(t, err)
		expired = pieces.FlattenExpirationInfoLists(expiredLists)
		require.Len(t, expired, 1)

		// DeleteExpiration normal usage
		err = expireDB.DeleteExpirations(ctx, expireAt)
		require.NoError(t, err)

		// Should not be there anymore
		expiredLists, err = expireDB.GetExpired(ctx, expireAt.Add(365*24*time.Hour), pieces.DefaultExpirationLimits())
		require.NoError(t, err)
		expired = pieces.FlattenExpirationInfoLists(expiredLists)
		require.Len(t, expired, 0)

		// let's add a few more
		var expectedExpired []pieces.ExpiredInfo
		randSetExpiration := func(ctx context.Context, expireAt time.Time) {
			ei := pieces.ExpiredInfo{
				SatelliteID: testrand.NodeID(),
				PieceID:     testrand.PieceID(),
			}
			err = expireDB.SetExpiration(ctx, ei.SatelliteID, ei.PieceID, expireAt.UTC(), 0)
			require.NoError(t, err)
			// setting it in the order in which the database will return it
			expectedExpired = append([]pieces.ExpiredInfo{ei}, expectedExpired...)
		}
		num := 0
		for num < 10 {
			num++
			randSetExpiration(ctx, expireAt.Add(-time.Duration(num)*time.Hour))
		}

		limits := pieces.DefaultExpirationLimits()

		// GetExpired batch
		limits.BatchSize = 1
		expiredLists, err = expireDB.GetExpired(ctx, expireAt, limits)
		require.NoError(t, err)
		expired = pieces.FlattenExpirationInfoLists(expiredLists)
		require.Len(t, expired, 1)
		require.Equal(t, expectedExpired[:1], expired)

		limits.BatchSize = 3
		expiredLists, err = expireDB.GetExpired(ctx, expireAt, limits)
		require.NoError(t, err)
		expired = pieces.FlattenExpirationInfoLists(expiredLists)
		require.Len(t, expired, 3)
		require.Equal(t, expectedExpired[:3], expired)

		limits.BatchSize = 10
		expiredLists, err = expireDB.GetExpired(ctx, expireAt, limits)
		require.NoError(t, err)
		expired = pieces.FlattenExpirationInfoLists(expiredLists)
		require.Len(t, expired, 10)
		require.Equal(t, expectedExpired, expired)

		// DeleteExpiration batch
		limits.BatchSize = 5
		err = expireDB.DeleteExpirationsBatch(ctx, expireAt, limits)
		require.NoError(t, err)
		// 5 old records should be gone
		limits.BatchSize = 10
		expiredLists, err = expireDB.GetExpired(ctx, expireAt, limits)
		require.NoError(t, err)
		expired = pieces.FlattenExpirationInfoLists(expiredLists)
		require.Len(t, expired, 5)
		require.Equal(t, expectedExpired[5:], expired)
	})
}

func TestPieceExpirationFlatStore(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		dataDir := ctx.Dir("pieceexpiration")
		store, err := pieces.NewPieceExpirationStore(zaptest.NewLogger(t), pieces.PieceExpirationConfig{
			DataDir:               dataDir,
			ConcurrentFileHandles: 2,
			MaxBufferTime:         time.Second,
		})
		require.NoError(t, err)

		// put values in both databases
		satelliteID := testrand.NodeID()
		pieceID1 := testrand.PieceID()
		pieceID2 := testrand.PieceID()
		pieceID3 := testrand.PieceID()
		now := time.Now()

		err = store.SetExpiration(ctx, satelliteID, pieceID1, now.Add(24*time.Hour), 111)
		require.NoError(t, err)
		err = store.SetExpiration(ctx, satelliteID, pieceID2, now.Add(40*time.Hour), 222)
		require.NoError(t, err)
		err = store.SetExpiration(ctx, satelliteID, pieceID3, now.Add(48*time.Hour), 333)
		require.NoError(t, err)

		expirationLists, err := store.GetExpired(ctx, now.Add(72*time.Hour), pieces.DefaultExpirationLimits())
		require.NoError(t, err)
		expirationInfos := pieces.FlattenExpirationInfoLists(expirationLists)
		require.Contains(t, expirationInfos, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID1,
			PieceSize:   111,
		})
		require.Contains(t, expirationInfos, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID2,
			PieceSize:   222,
		})
		require.Contains(t, expirationInfos, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID3,
			PieceSize:   333,
		})

		// delete up to now+36h
		err = store.DeleteExpirations(ctx, now.Add(36*time.Hour))
		require.NoError(t, err)

		// piece1 should be deleted from the store, and not the others
		expirationLists, err = store.GetExpired(ctx, now.Add(72*time.Hour), pieces.DefaultExpirationLimits())
		require.NoError(t, err)
		expirationInfos = pieces.FlattenExpirationInfoLists(expirationLists)
		require.Len(t, expirationInfos, 2)
		require.Contains(t, expirationInfos, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID2,
			PieceSize:   222,
		})
		require.Contains(t, expirationInfos, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID3,
			PieceSize:   333,
		})
	})
}
