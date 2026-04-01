// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package piecemigrate_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
)

// TestMigrationChoreSpaceReportingAfterMigration verifies the full production
// caching stack (BlobsUsageCache → UsedSpacePerPrefixDB) using a real planet.
//
// Specifically it checks that WalkAndComputeSpaceUsedBySatellite returns zero
// after migration completes.  This requires WalkSatellitePiecesMigration to
// have marked the prefix-cache entries as stale; if it did not, the filewalker
// would trust the cached totals and return a non-zero value for the now-empty
// old backend.
func TestMigrationChoreSpaceReportingAfterMigration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		storageNode := planet.StorageNodes[0]
		satID := planet.Satellites[0].ID()

		// Keep the migration chore from running automatically while we set up.
		storageNode.Storage2.MigrationChore.Loop.Pause()

		// Write pieces to the old (filestore) backend instead of the new
		// (hashstore) backend so that we have data to migrate.
		storageNode.Storage2.MigratingBackend.UpdateState(ctx, satID, func(s *piecestore.MigrationState) {
			s.WriteToNew = false
			s.TTLToNew = false
		})

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", testrand.Bytes(100*memory.KiB))
		require.NoError(t, err)

		// Populate UsedSpacePerPrefixDB with fresh, non-zero entries.
		// Without WalkSatellitePiecesMigration these would remain "fresh"
		// after migration and cause subsequent scans to return stale totals.
		total, _, err := storageNode.StorageOld.Store.WalkAndComputeSpaceUsedBySatellite(ctx, satID, false)
		require.NoError(t, err)
		require.Greater(t, total, int64(0), "expected non-zero used space after uploading")

		// Enable active migration and drive the chore until the old backend is
		// drained.  TriggerWait runs one chore cycle; multiple may be needed if
		// the upload produced several pieces.
		storageNode.Storage2.MigrationChore.SetMigrate(satID, true, true)
		for {
			storageNode.Storage2.MigrationChore.Loop.TriggerWait()
			var count int
			require.NoError(t, storageNode.StorageOld.Store.WalkSatellitePieces(ctx, satID, func(pieces.StoredPieceAccess) error {
				count++
				return nil
			}))
			if count == 0 {
				break
			}
		}

		// The post-migration scan must return zero.  WalkSatellitePiecesMigration
		// marked all prefix-cache entries stale, so the filewalker re-reads from
		// disk, finds no pieces, and reports zero.
		total, _, err = storageNode.StorageOld.Store.WalkAndComputeSpaceUsedBySatellite(ctx, satID, false)
		require.NoError(t, err)
		require.Equal(t, int64(0), total, "post-migration scan must report zero used space for old backend")
	})
}
