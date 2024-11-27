// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestPieceExpirationCombinedStore(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		sqliteDB := db.PieceExpirationDB()

		dataDir := ctx.Dir("pieceexpiration")
		store, err := pieces.NewPieceExpirationStore(zaptest.NewLogger(t), pieces.PieceExpirationConfig{
			DataDir:               dataDir,
			ConcurrentFileHandles: 2,
			MaxBufferTime:         time.Second,
		})
		require.NoError(t, err)

		combinedStore := pieces.NewCombinedExpirationStore(zaptest.NewLogger(t), sqliteDB, store)
		// put values in both databases
		satelliteID := testrand.NodeID()
		pieceID1 := testrand.PieceID()
		pieceID2 := testrand.PieceID()
		pieceID3 := testrand.PieceID()
		now := time.Now()

		// put pieceID1 in both backends. We don't expect this to be a normal
		// situation, but the code should be able to handle it.
		err = combinedStore.SetExpiration(ctx, satelliteID, pieceID1, now.Add(24*time.Hour), 111)
		require.NoError(t, err)
		err = sqliteDB.SetExpiration(ctx, satelliteID, pieceID1, now.Add(24*time.Hour), 111)
		require.NoError(t, err)

		err = combinedStore.SetExpiration(ctx, satelliteID, pieceID2, now.Add(40*time.Hour), 222)
		require.NoError(t, err)
		err = sqliteDB.SetExpiration(ctx, satelliteID, pieceID3, now.Add(48*time.Hour), 333)
		require.NoError(t, err)

		// check to see that values are in both backends
		expirationLists, err := combinedStore.GetExpired(ctx, now.Add(72*time.Hour), pieces.DefaultExpirationOptions())
		require.NoError(t, err)
		expirationInfos := pieces.FlattenExpirationInfoLists(expirationLists)
		require.Len(t, expirationInfos, 4)
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
		opts := pieces.DefaultExpirationOptions()
		opts.Limits.BatchSize = 10
		err = combinedStore.DeleteExpirationsBatch(ctx, now.Add(36*time.Hour), opts)
		require.NoError(t, err)

		// piece1 should be deleted from both databases, and not the others
		expirationLists, err = combinedStore.GetExpired(ctx, now.Add(72*time.Hour), pieces.DefaultExpirationOptions())
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
		})

		// querying sqlite3 db only
		expirationLists, err = sqliteDB.GetExpired(ctx, now.Add(72*time.Hour), pieces.DefaultExpirationOptions())
		require.NoError(t, err)
		expirationInfos = pieces.FlattenExpirationInfoLists(expirationLists)
		require.Len(t, expirationInfos, 1)
		require.Equal(t, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID3,
		}, expirationInfos[0])
	})
}
