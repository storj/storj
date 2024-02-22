// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestGCFilewalkerDB_GetInsert(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		gcFilewalkerDB := db.GCFilewalkerProgress()

		progress := pieces.GCFilewalkerProgress{
			SatelliteID:              testrand.NodeID(),
			BloomfilterCreatedBefore: time.Now().UTC(),
			Prefix:                   "yf",
		}

		t.Run("insert", func(t *testing.T) {
			err := gcFilewalkerDB.Store(ctx, progress)
			require.NoError(t, err)
		})

		t.Run("get", func(t *testing.T) {
			result, err := gcFilewalkerDB.Get(ctx, progress.SatelliteID)
			require.NoError(t, err)

			require.Equal(t, result.SatelliteID, progress.SatelliteID)
			require.Equal(t, result.BloomfilterCreatedBefore, progress.BloomfilterCreatedBefore)
			require.Equal(t, result.Prefix, progress.Prefix)
		})

		t.Run("reset", func(t *testing.T) {
			err := gcFilewalkerDB.Reset(ctx, progress.SatelliteID)
			require.NoError(t, err)

			result, err := gcFilewalkerDB.Get(ctx, progress.SatelliteID)
			require.Error(t, err)
			require.Equal(t, pieces.GCFilewalkerProgress{SatelliteID: progress.SatelliteID}, result)
		})
	})
}

func TestUsedSpacePerPrefix_GetInsert(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		usedSpacePerPrefixDB := db.UsedSpacePerPrefix()

		usedSpace := pieces.PrefixUsedSpace{
			SatelliteID: testrand.NodeID(),
			Prefix:      "yf",
			TotalBytes:  1234567890,
			LastUpdated: time.Now().UTC(),
		}

		t.Run("insert", func(t *testing.T) {
			err := usedSpacePerPrefixDB.Store(ctx, usedSpace)
			require.NoError(t, err)
		})

		t.Run("get", func(t *testing.T) {
			results, err := usedSpacePerPrefixDB.Get(ctx, usedSpace.SatelliteID)
			require.NoError(t, err)

			require.Len(t, results, 1)
			require.Equal(t, results[0].SatelliteID, usedSpace.SatelliteID)
			require.Equal(t, results[0].Prefix, usedSpace.Prefix)
			require.Equal(t, results[0].TotalBytes, usedSpace.TotalBytes)
			require.Equal(t, results[0].LastUpdated, usedSpace.LastUpdated)
		})
	})
}
