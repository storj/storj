// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package rolluparchive_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/shared/dbutil"
)

func TestRollupArchiveChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// ensure that orders (and rollups) aren't marked as expired and removed
				config.Orders.Expiration = time.Hour * 24 * 7
			},
		},
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			for _, sn := range planet.StorageNodes {
				sn.Contact.Chore.TriggerWait(ctx)
			}

			// The purpose of this test is to ensure that the archive chore deletes
			// entries in the storagenode_bandwidth_rollups and bucket_bandwidth_rollups tables
			// and inserts those entries into new archive tables.

			satellite := planet.Satellites[0]
			satellite.Accounting.Rollup.Loop.Pause()

			days := 6

			currentTime := time.Now().UTC()
			// Set timestamp back by the number of days we want to save
			timestamp := currentTime.AddDate(0, 0, -days).Truncate(time.Millisecond)

			projectID := testrand.UUID()

			for i := 0; i < days; i++ {
				nodeID := testrand.NodeID()
				var bucketName string
				bwAmount := int64(1000)

				// When the bucket name and intervalStart is different, a new record is created
				bucketName = fmt.Sprintf("%s%d", "testbucket", i)

				err := satellite.DB.Orders().UpdateBucketBandwidthSettle(ctx,
					projectID, []byte(bucketName), pb.PieceAction_GET, bwAmount, 0, timestamp,
				)
				require.NoError(t, err)

				err = satellite.DB.Orders().UpdateStoragenodeBandwidthSettle(ctx,
					nodeID, pb.PieceAction_GET, bwAmount, timestamp)
				require.NoError(t, err)

				// Advance time by 24 hours
				timestamp = timestamp.Add(time.Hour * 24)
			}

			lastWeek := currentTime.AddDate(0, 0, -7).Truncate(time.Millisecond)
			nodeRollups, err := satellite.DB.StoragenodeAccounting().GetRollupsSince(ctx, lastWeek)
			require.NoError(t, err)
			require.Len(t, nodeRollups, days)

			bucketRollups, err := satellite.DB.ProjectAccounting().GetRollupsSince(ctx, lastWeek)
			require.NoError(t, err)
			require.Len(t, bucketRollups, days)

			// We take off a millisecond so the before isn't exactly the same as one of the interval starts.
			before := currentTime.AddDate(0, 0, -days/2).Add(-time.Millisecond)
			batchSize := 1000
			err = satellite.Accounting.RollupArchive.ArchiveRollups(ctx, before, batchSize)
			require.NoError(t, err)

			nodeRollups, err = satellite.DB.StoragenodeAccounting().GetRollupsSince(ctx, lastWeek)
			require.NoError(t, err)
			require.Len(t, nodeRollups, days/2)

			bucketRollups, err = satellite.DB.ProjectAccounting().GetRollupsSince(ctx, lastWeek)
			require.NoError(t, err)
			require.Len(t, bucketRollups, days/2)

			nodeRollups, err = satellite.DB.StoragenodeAccounting().GetArchivedRollupsSince(ctx, lastWeek)
			require.NoError(t, err)
			require.Len(t, nodeRollups, days/2)

			bucketRollups, err = satellite.DB.ProjectAccounting().GetArchivedRollupsSince(ctx, lastWeek)
			require.NoError(t, err)
			require.Len(t, bucketRollups, days/2)
		})
}

func TestRollupArchiveChore_AlreadyArchived(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]
			if satellite.DB.Testing().Implementation()[0] != dbutil.Postgres {
				t.Skip("skipping archive conflict test for non-Postgres implementations")
			}
			satellite.Accounting.Rollup.Loop.Pause()

			// The purpose of this test is to ensure that rows which are already
			// archived don't block archiving the rest of the rollups, and that
			// their usage isn't counted twice.

			rows := 5
			timestamp := time.Now().UTC().AddDate(0, 0, -10).Truncate(time.Hour)
			projectID := testrand.UUID()

			for i := 0; i < rows; i++ {
				err := satellite.DB.Orders().UpdateBucketBandwidthSettle(ctx,
					projectID, []byte(fmt.Sprintf("testbucket%d", i)), pb.PieceAction_GET, 1000, 0, timestamp,
				)
				require.NoError(t, err)

				err = satellite.DB.Orders().UpdateStoragenodeBandwidthSettle(ctx,
					testrand.NodeID(), pb.PieceAction_GET, 1000, timestamp)
				require.NoError(t, err)
			}

			// Copy one row of each table into the archives, so archiving them conflicts.
			rawDB := satellite.DB.Testing().RawDB()
			_, err := rawDB.ExecContext(ctx, `
				INSERT INTO storagenode_bandwidth_rollup_archives
				SELECT * FROM storagenode_bandwidth_rollups LIMIT 1
			`)
			require.NoError(t, err)
			_, err = rawDB.ExecContext(ctx, `
				INSERT INTO bucket_bandwidth_rollup_archives(bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
				SELECT bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled FROM bucket_bandwidth_rollups LIMIT 1
			`)
			require.NoError(t, err)

			// Use a small batch size to archive in multiple batches.
			err = satellite.Accounting.RollupArchive.ArchiveRollups(ctx, time.Now(), 2)
			require.NoError(t, err)

			since := timestamp.Add(-time.Hour)
			nodeRollups, err := satellite.DB.StoragenodeAccounting().GetRollupsSince(ctx, since)
			require.NoError(t, err)
			require.Empty(t, nodeRollups)

			bucketRollups, err := satellite.DB.ProjectAccounting().GetRollupsSince(ctx, since)
			require.NoError(t, err)
			require.Empty(t, bucketRollups)

			nodeRollups, err = satellite.DB.StoragenodeAccounting().GetArchivedRollupsSince(ctx, since)
			require.NoError(t, err)
			require.Len(t, nodeRollups, rows)
			var nodeSettled uint64
			for _, rollup := range nodeRollups {
				nodeSettled += rollup.Settled
			}
			require.EqualValues(t, rows*1000, nodeSettled)

			bucketRollups, err = satellite.DB.ProjectAccounting().GetArchivedRollupsSince(ctx, since)
			require.NoError(t, err)
			require.Len(t, bucketRollups, rows)
			var bucketSettled int64
			for _, rollup := range bucketRollups {
				bucketSettled += rollup.Settled
			}
			require.EqualValues(t, rows*1000, bucketSettled)
		})
}
