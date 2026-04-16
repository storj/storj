// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package piecemigrate_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode/hashstore"
	"storj.io/storj/storagenode/piecemigrate"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/satstore"
	"storj.io/storj/storagenode/storagenodedb"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

// TestMigrationChoreCorrectSpaceAfterRestart verifies the full production
// caching stack across a simulated crash-and-restart:
//
//  1. Write pieces to the old backend.
//  2. Run CacheService so its startup scan populates both the in-memory
//     BlobsUsageCache and the persistent PieceSpaceUsedDB with non-zero totals.
//  3. Stop CacheService without calling PersistCacheTotals again (crash
//     simulation: PieceSpaceUsedDB still holds the pre-migration totals).
//  4. Run the migration chore.  enqueueSatellite calls
//     WalkSatellitePiecesMigration, which marks every UsedSpacePerPrefixDB
//     entry as stale before copying pieces to the new backend and deleting them.
//     Each deletion decrements BlobsUsageCache, driving it to zero — but that
//     zero is never flushed to PieceSpaceUsedDB before the "crash".
//  5. Simulate a node restart by closing and reopening the same on-disk database.
//  6. Call CacheService.Init, which loads the stale non-zero PieceSpaceUsedDB
//     totals back into a fresh BlobsUsageCache.
//  7. Assert that SpaceUsedBySatellite (which reads BlobsUsageCache) reports
//     the stale non-zero value — this is the window before the startup scan.
//  8. Run CacheService.  Its startup scan finds UsedSpacePerPrefixDB stale,
//     re-reads from disk, sees no pieces, and drives BlobsUsageCache to zero.
//  9. Assert that SpaceUsedBySatellite now reports zero.
//
// If WalkSatellitePiecesMigration is removed from the migration chore the
// UsedSpacePerPrefixDB entries remain fresh after step 4.  The startup scan in
// step 8 then trusts those fresh entries, skips the empty directories, and
// reports a non-zero total — causing the final assertion to fail.
func TestMigrationChoreCorrectSpaceAfterRestart(t *testing.T) {
	t.Parallel()

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	defer ctx.Check(log.Sync)

	// storageDir persists between the two "runs" so that the SQLite databases
	// (UsedSpacePerPrefixDB, PieceSpaceUsedDB) and filestore blobs survive the
	// simulated restart.
	storageDir := t.TempDir()

	cfg := storagenodedb.Config{
		Storage:           storageDir,
		Info:              filepath.Join(storageDir, "piecestore.db"),
		Info2:             filepath.Join(storageDir, "info.db"),
		Driver:            "sqlite3+utccheck",
		Pieces:            storageDir,
		TestingDisableWAL: true,
	}

	sat := testrand.NodeID()

	// ── First run ────────────────────────────────────────────────────────────
	// Populate all three caching layers, migrate all pieces, then close without
	// flushing the post-migration zero totals to PieceSpaceUsedDB (crash sim).
	{
		db, err := storagenodedbtest.OpenNew(ctx, log, cfg)
		require.NoError(t, err)
		require.NoError(t, db.MigrateToLatest(ctx))

		// Wrap the raw blobstore in BlobsUsageCache exactly as the production
		// peer does (peer.go: oldBlobStore = peer.StorageOld.BlobsCache).
		blobsCache := pieces.NewBlobsUsageCache(log, db.Pieces())
		fw := pieces.NewFileWalker(log, blobsCache, nil, nil, db.UsedSpacePerPrefix())
		old := pieces.NewStore(log, fw, nil, blobsCache, nil, nil, pieces.DefaultConfig)

		writeTestPieces(ctx, t, old, sat, 20)

		// Run CacheService with a long sync interval so the periodic persist
		// loop does not fire during the test.  The startup scan populates
		// BlobsUsageCache and persists non-zero totals to PieceSpaceUsedDB.
		cacheService := pieces.NewService(log, blobsCache, old, db.PieceSpaceUsedDB(), time.Hour, true)
		var cacheGroup errgroup.Group
		cacheGroup.Go(func() error { return cacheService.Run(ctx) })
		cacheService.InitFence.Wait(ctx)

		total, _, err := old.SpaceUsedBySatellite(ctx, sat)
		require.NoError(t, err)
		require.Greater(t, total, int64(0), "expected non-zero used space after writing pieces")

		// Stop CacheService now.  Loop.Close does not call PersistCacheTotals,
		// so PieceSpaceUsedDB retains the pre-migration non-zero totals.
		require.NoError(t, cacheService.Close())
		require.NoError(t, cacheGroup.Wait())

		newBackend, err := piecestore.NewHashStoreBackend(ctx,
			hashstore.CreateDefaultConfig(hashstore.TableKind_HashTbl, false),
			t.TempDir(), "", nil, nil, log, nil)
		require.NoError(t, err)

		chore := piecemigrate.NewChore(log, piecemigrate.Config{
			Interval:   100 * time.Millisecond,
			Delay:      time.Millisecond,
			BufferSize: 10,
		}, satstore.NewSatelliteStore(t.TempDir(), "migrate_chore"), old, newBackend, nil, "")

		var choreGroup errgroup.Group
		choreGroup.Go(func() error { return chore.Run(ctx) })

		// WalkSatellitePiecesMigration (called inside enqueueSatellite) marks
		// all UsedSpacePerPrefixDB entries stale before migrating each piece.
		// Each subsequent Delete call decrements BlobsUsageCache to zero, but
		// that zero is never written to PieceSpaceUsedDB — simulating the crash.
		chore.SetMigrate(sat, true, true)
		waitForOldStoreEmpty(ctx, t, old, sat)

		require.NoError(t, chore.Close())
		require.NoError(t, choreGroup.Wait())
		require.NoError(t, newBackend.Close())
		require.NoError(t, db.Close())
	}

	// ── Simulated restart ────────────────────────────────────────────────────
	// Reopen the same on-disk database and replay the production startup
	// sequence: CacheService.Init loads PieceSpaceUsedDB → BlobsUsageCache,
	// then CacheService.Run re-scans and corrects the stale totals.
	{
		db, err := storagenodedb.OpenExisting(ctx, log, cfg)
		require.NoError(t, err)
		defer ctx.Check(db.Close)

		blobsCache := pieces.NewBlobsUsageCache(log, db.Pieces())
		fw := pieces.NewFileWalker(log, blobsCache, nil, nil, db.UsedSpacePerPrefix())
		old := pieces.NewStore(log, fw, nil, blobsCache, nil, nil, pieces.DefaultConfig)

		cacheService := pieces.NewService(log, blobsCache, old, db.PieceSpaceUsedDB(), time.Hour, true)

		// Replicate cmd/storagenode/cmd_run.go: Init loads PieceSpaceUsedDB
		// values into BlobsUsageCache before the node starts serving requests.
		require.NoError(t, cacheService.Init(ctx))

		// Before the startup scan completes, SpaceUsedBySatellite reads the
		// stale non-zero value that was loaded from PieceSpaceUsedDB.
		total, _, err := old.SpaceUsedBySatellite(ctx, sat)
		require.NoError(t, err)
		require.Greater(t, total, int64(0),
			"BlobsUsageCache should hold the stale pre-migration total before the startup scan")

		// Run the startup scan.  UsedSpacePerPrefixDB entries are stale, so the
		// filewalker re-reads from disk, finds nothing, and recalculates to zero.
		var cacheGroup errgroup.Group
		cacheGroup.Go(func() error { return cacheService.Run(ctx) })
		cacheService.InitFence.Wait(ctx)

		total, _, err = old.SpaceUsedBySatellite(ctx, sat)
		require.NoError(t, err)
		require.Equal(t, int64(0), total,
			"startup scan after migration and restart must report zero used space")

		require.NoError(t, cacheService.Close())
		require.NoError(t, cacheGroup.Wait())
	}
}

func writeTestPieces(ctx context.Context, t *testing.T, store *pieces.Store, sat storj.NodeID, n int) {
	t.Helper()
	content := testrand.Bytes(memory.KiB)
	for range n {
		func() {
			w, err := store.Writer(ctx, sat, testrand.PieceID(), pb.PieceHashAlgorithm_SHA256)
			require.NoError(t, err)
			defer func() { _ = w.Cancel(ctx) }()
			_, err = sync2.Copy(ctx, w, bytes.NewReader(content))
			require.NoError(t, err)
			require.NoError(t, w.Commit(ctx, &pb.PieceHeader{Hash: w.Hash()}))
		}()
	}
}

func waitForOldStoreEmpty(ctx context.Context, t *testing.T, store *pieces.Store, sat storj.NodeID) {
	t.Helper()
	for {
		var count int
		require.NoError(t, store.WalkSatellitePieces(ctx, sat, func(pieces.StoredPieceAccess) error {
			count++
			return nil
		}))
		if count == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
