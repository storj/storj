// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"database/sql"
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestGCFilewalkerDB_GetInsert(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		gcFilewalkerDB := db.GCFilewalkerProgress()

		progress := pieces.GCFilewalkerProgress{
			SatelliteID:              testrand.NodeID(),
			BloomfilterCreatedBefore: time.Now().UTC(),
		}
		t.Run("insert", func(t *testing.T) {
			// inserting multiple prefixes to ensure flushing to trigger the insert
			for i := uint16(0); i < 32*2; i++ {
				var b [2]byte
				binary.BigEndian.PutUint16(b[:], i<<6)
				keyPrefix := filestore.PathEncoding.EncodeToString(b[:])[:2]
				progress.Prefix = keyPrefix
				err := gcFilewalkerDB.Store(ctx, progress)
				require.NoError(t, err)
			}
		})

		t.Run("get", func(t *testing.T) {
			result, err := gcFilewalkerDB.Get(ctx, progress.SatelliteID)
			require.NoError(t, err)

			require.Equal(t, result.SatelliteID, progress.SatelliteID)
			require.Equal(t, result.BloomfilterCreatedBefore, progress.BloomfilterCreatedBefore)
			require.Equal(t, result.Prefix, progress.Prefix)

			// find progress for an unknown satellite; also ensures a cache miss
			satellite2 := testrand.NodeID()
			result, err = gcFilewalkerDB.Get(ctx, satellite2)
			require.ErrorIs(t, err, sql.ErrNoRows)
			require.Equal(t, pieces.GCFilewalkerProgress{SatelliteID: satellite2}, result)
		})

		t.Run("reset", func(t *testing.T) {
			err := gcFilewalkerDB.Reset(ctx, progress.SatelliteID)
			require.NoError(t, err)

			result, err := gcFilewalkerDB.Get(ctx, progress.SatelliteID)
			require.ErrorIs(t, err, sql.ErrNoRows)
			require.Equal(t, pieces.GCFilewalkerProgress{SatelliteID: progress.SatelliteID}, result)
		})
	})
}

func TestUsedSpacePerPrefix_GetInsert(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		usedSpacePerPrefixDB := db.UsedSpacePerPrefix()

		usedSpace := pieces.PrefixUsedSpace{
			SatelliteID:      testrand.NodeID(),
			Prefix:           "yf",
			TotalBytes:       1234567890,
			TotalContentSize: 123456789,
			PieceCounts:      123,
			LastUpdated:      time.Now().UTC(),
		}

		var expectedPrefixes = make(map[string]pieces.PrefixUsedSpace)

		// Store
		err := usedSpacePerPrefixDB.Store(ctx, usedSpace)
		require.NoError(t, err)
		expectedPrefixes[usedSpace.Prefix] = usedSpace

		// storeBatch
		usedSpace2 := usedSpace
		usedSpace2.Prefix = "1a"
		usedSpace2.LastUpdated = time.Now().Add(-time.Hour * 1).UTC()

		usedSpace3 := usedSpace
		usedSpace3.Prefix = "aa"

		prefixes := []pieces.PrefixUsedSpace{usedSpace2, usedSpace3}
		err = usedSpacePerPrefixDB.StoreBatch(ctx, prefixes)
		require.NoError(t, err)
		expectedPrefixes[usedSpace2.Prefix] = usedSpace2
		expectedPrefixes[usedSpace3.Prefix] = usedSpace3

		// Get
		results, err := usedSpacePerPrefixDB.Get(ctx, usedSpace.SatelliteID, nil)
		require.NoError(t, err)

		require.Len(t, results, len(expectedPrefixes))
		for _, result := range results {
			require.Equal(t, expectedPrefixes[result.Prefix], result)
		}

		// insert a new prefix with older lastUpdated
		usedSpace4 := usedSpace
		usedSpace4.Prefix = "zz"
		usedSpace4.LastUpdated = time.Now().Add(-time.Hour * 48).UTC()
		err = usedSpacePerPrefixDB.Store(ctx, usedSpace4)
		require.NoError(t, err)
		expectedPrefixes[usedSpace4.Prefix] = usedSpace4

		// Get with lastUpdated
		lastUpdated := time.Now().Add(-time.Hour * 24)
		results, err = usedSpacePerPrefixDB.Get(ctx, usedSpace.SatelliteID, &lastUpdated)
		require.NoError(t, err)
		require.Len(t, results, 3)
		for _, result := range results {
			require.Equal(t, expectedPrefixes[result.Prefix], result)
		}

		// GetSatelliteUsedSpace
		var expectedTotal int64
		var expectedContentSize int64

		for _, prefix := range expectedPrefixes {
			expectedTotal += prefix.TotalBytes
			expectedContentSize += prefix.TotalContentSize
		}

		piecesTotal, piecesContentSize, _, err := usedSpacePerPrefixDB.GetSatelliteUsedSpace(ctx, usedSpace.SatelliteID)
		require.NoError(t, err)
		require.Equal(t, expectedTotal, piecesTotal)
		require.Equal(t, expectedContentSize, piecesContentSize)
	})
}
