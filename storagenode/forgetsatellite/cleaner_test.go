// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package forgetsatellite_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/internalpb"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/satellites"
)

func TestCleaner(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 3, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		storagenode := planet.StorageNodes[0]
		cleanupSatellite := planet.Satellites[0]

		// pause the chore
		storagenode.ForgetSatellite.Chore.Loop.Pause()

		store := planet.StorageNodes[0].Storage2.BlobsCache
		defer ctx.Check(store.Close)

		blobSize := memory.KB
		blobRef := blobstore.BlobRef{
			Namespace: cleanupSatellite.ID().Bytes(),
			Key:       testrand.PieceID().Bytes(),
		}
		w, err := store.Create(ctx, blobRef)
		require.NoError(t, err)
		_, err = w.Write(testrand.Bytes(blobSize))
		require.NoError(t, err)
		require.NoError(t, w.Commit(ctx))

		// create a new satellite reputation
		timestamp := time.Now().UTC()
		reputationDB := planet.StorageNodes[0].DB.Reputation()

		stats := reputation.Stats{
			SatelliteID: cleanupSatellite.ID(),
			Audit: reputation.Metric{
				TotalCount:   6,
				SuccessCount: 7,
				Alpha:        8,
				Beta:         9,
				Score:        10,
				UnknownAlpha: 11,
				UnknownBeta:  12,
				UnknownScore: 13,
			},
			OnlineScore: 14,
			UpdatedAt:   timestamp,
			JoinedAt:    timestamp,
		}
		err = reputationDB.Store(ctx, stats)
		require.NoError(t, err)
		// test that the reputation was stored correctly
		rstats, err := reputationDB.Get(ctx, cleanupSatellite.ID())
		require.NoError(t, err)
		require.NotNil(t, rstats)
		require.Equal(t, stats, *rstats)

		// cleanup not initiated
		err = storagenode.ForgetSatellite.Cleaner.Run(ctx, cleanupSatellite.ID())
		require.Error(t, err, "forget-satellite not initiated for satellite")

		// initiate cleanup
		resp, err := storagenode.ForgetSatellite.Endpoint.InitForgetSatellite(ctx, &internalpb.InitForgetSatelliteRequest{
			SatelliteId:  cleanupSatellite.ID(),
			ForceCleanup: true,
		})
		require.NoError(t, err)
		require.Equal(t, true, resp.InProgress)
		require.Equal(t, cleanupSatellite.ID(), resp.SatelliteId)

		// run the cleaner
		err = storagenode.ForgetSatellite.Cleaner.Run(ctx, cleanupSatellite.ID())
		require.NoError(t, err)

		// check status
		satellite, err := storagenode.DB.Satellites().GetSatellite(ctx, cleanupSatellite.ID())
		require.NoError(t, err)
		require.Equal(t, cleanupSatellite.ID(), satellite.SatelliteID)
		require.Equal(t, satellites.CleanupSucceeded, satellite.Status)

		// check that the blob was deleted
		blobInfo, err := store.Stat(ctx, blobRef)
		require.Error(t, err)
		require.True(t, errs.Is(err, os.ErrNotExist))
		require.Nil(t, blobInfo)

		// check that the reputation was deleted
		rstats, err = reputationDB.Get(ctx, cleanupSatellite.ID())
		require.NoError(t, err)
		require.Equal(t, &reputation.Stats{SatelliteID: cleanupSatellite.ID()}, rstats)

		// check that satellite is no longer in the trust pool
		require.False(t, storagenode.Storage2.Trust.IsTrusted(ctx, cleanupSatellite.ID()))

		// try to clean up again
		err = storagenode.ForgetSatellite.Cleaner.Run(ctx, cleanupSatellite.ID())
		require.Error(t, err, "cleanup already completed for satellite")
	})
}
