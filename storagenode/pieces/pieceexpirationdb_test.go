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
		expired, err := expireDB.GetExpired(ctx, time.Now(), -1)
		require.NoError(t, err)
		require.Len(t, expired, 0)

		// DeleteExpiration with no matches
		err = expireDB.DeleteExpirations(ctx, time.Time{})
		require.NoError(t, err)

		expireAt := time.Now()

		// SetExpiration
		err = expireDB.SetExpiration(ctx, satelliteID, pieceID, expireAt.Add(-time.Hour), 0)
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

func TestPieceExpirationBothDBs(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		sqliteDB := db.PieceExpirationDB()

		dataDir := ctx.Dir("pieceexpiration")
		store, err := pieces.NewPieceExpirationStore(zaptest.NewLogger(t), sqliteDB, pieces.PieceExpirationConfig{
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

		// put pieceID1 in both backends. We don't expect this to be a normal
		// situation, but the code should be able to handle it.
		err = store.SetExpiration(ctx, satelliteID, pieceID1, now.Add(24*time.Hour), 111)
		require.NoError(t, err)
		err = sqliteDB.SetExpiration(ctx, satelliteID, pieceID1, now.Add(24*time.Hour), 111)
		require.NoError(t, err)

		err = store.SetExpiration(ctx, satelliteID, pieceID2, now.Add(40*time.Hour), 222)
		require.NoError(t, err)
		err = sqliteDB.SetExpiration(ctx, satelliteID, pieceID3, now.Add(48*time.Hour), 333)
		require.NoError(t, err)

		// check to see that values are in both backends
		expirationInfos, err := store.GetExpired(ctx, now.Add(72*time.Hour), -1)
		require.NoError(t, err)
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
		// (we don't expect PieceSize here; it is not stored in the sqlite db)
		require.Contains(t, expirationInfos, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID3,
		})

		// delete up to now+36h
		err = store.DeleteExpirations(ctx, now.Add(36*time.Hour))
		require.NoError(t, err)

		// piece1 should be deleted from both databases, and not the others
		expirationInfos, err = store.GetExpired(ctx, now.Add(72*time.Hour), -1)
		require.NoError(t, err)
		require.Len(t, expirationInfos, 2)
		require.Contains(t, expirationInfos, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID2,
			PieceSize:   222,
		})
		require.Contains(t, expirationInfos, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID3,
		})

		// querying sqlite3 db only
		expirationInfos, err = sqliteDB.GetExpired(ctx, now.Add(72*time.Hour), -1)
		require.NoError(t, err)
		require.Len(t, expirationInfos, 1)
		require.Equal(t, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID3,
		}, expirationInfos[0])
	})
}

func TestPieceExpirationBatchBothDBs(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		sqliteDB := db.PieceExpirationDB()

		dataDir := ctx.Dir("pieceexpiration")
		store, err := pieces.NewPieceExpirationStore(zaptest.NewLogger(t), sqliteDB, pieces.PieceExpirationConfig{
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

		// put pieceID1 in both backends. We don't expect this to be a normal
		// situation, but the code should be able to handle it.
		err = store.SetExpiration(ctx, satelliteID, pieceID1, now.Add(24*time.Hour), 111)
		require.NoError(t, err)
		err = sqliteDB.SetExpiration(ctx, satelliteID, pieceID1, now.Add(24*time.Hour), 111)
		require.NoError(t, err)

		err = store.SetExpiration(ctx, satelliteID, pieceID2, now.Add(40*time.Hour), 222)
		require.NoError(t, err)
		err = sqliteDB.SetExpiration(ctx, satelliteID, pieceID3, now.Add(48*time.Hour), 333)
		require.NoError(t, err)

		// check to see that values are in both backends
		expirationInfos, err := store.GetExpired(ctx, now.Add(72*time.Hour), -1)
		require.NoError(t, err)
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
		// (we don't expect PieceSize here; it is not stored in the sqlite db)
		require.Contains(t, expirationInfos, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID3,
		})

		// delete up to now+36h
		err = store.DeleteExpirationsBatch(ctx, now.Add(36*time.Hour), 10)
		require.NoError(t, err)

		// piece1 should be deleted from both databases, and not the others
		expirationInfos, err = store.GetExpired(ctx, now.Add(72*time.Hour), -1)
		require.NoError(t, err)
		require.Len(t, expirationInfos, 2)
		require.Contains(t, expirationInfos, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID2,
			PieceSize:   222,
		})
		require.Contains(t, expirationInfos, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID3,
		})

		// querying sqlite3 db only
		expirationInfos, err = sqliteDB.GetExpired(ctx, now.Add(72*time.Hour), -1)
		require.NoError(t, err)
		require.Len(t, expirationInfos, 1)
		require.Equal(t, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID3,
		}, expirationInfos[0])
	})
}
