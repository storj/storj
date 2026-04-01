// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package piecemigrate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/storagenode/blobstore/filestore"
)

// staticWriteStateChecker is a test helper that always returns a fixed value for IsWritingToNew.
type staticWriteStateChecker struct{ writingToNew bool }

func (s *staticWriteStateChecker) IsWritingToNew(storj.NodeID) bool { return s.writingToNew }

func TestCleanupEmptyDirectories(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	satelliteID := storj.NodeID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	namespaceStr := filestore.PathEncoding.EncodeToString(satelliteID.Bytes())

	newChore := func(t *testing.T) (*Chore, string) {
		tempDir := t.TempDir()
		blobsPath := filepath.Join(tempDir, "blobs")
		chore := &Chore{
			log:               log,
			config:            Config{CleanupEmptyDirs: true},
			oldBlobsPath:      blobsPath,
			migratingActive:   make(map[storj.NodeID]bool),
			migratingProgress: make(map[storj.NodeID]*migrationProgress),
		}
		chore.migratingProgress[satelliteID] = &migrationProgress{}
		return chore, filepath.Join(blobsPath, namespaceStr)
	}

	t.Run("removes empty prefix directories", func(t *testing.T) {
		chore, satelliteDir := newChore(t)

		prefixes := []string{"aa", "bb", "cc", "dd", "ee"}
		for _, prefix := range prefixes {
			require.NoError(t, os.MkdirAll(filepath.Join(satelliteDir, prefix), 0755))
		}

		chore.cleanupEmptyDirectories(ctx, satelliteID)

		for _, prefix := range prefixes {
			require.NoDirExists(t, filepath.Join(satelliteDir, prefix))
		}
		// Satellite directory itself should also be removed.
		require.NoDirExists(t, satelliteDir)

		chore.mu.Lock()
		require.Equal(t, int64(0), chore.migratingProgress[satelliteID].remainingDirectories)
		chore.mu.Unlock()
	})

	t.Run("keeps non-empty prefix directories", func(t *testing.T) {
		chore, satelliteDir := newChore(t)

		// Create some empty and some non-empty prefix dirs.
		emptyPrefixes := []string{"aa", "bb"}
		for _, prefix := range emptyPrefixes {
			require.NoError(t, os.MkdirAll(filepath.Join(satelliteDir, prefix), 0755))
		}

		activePath := filepath.Join(satelliteDir, "cc")
		require.NoError(t, os.MkdirAll(activePath, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(activePath, "piece.sj1"), []byte("data"), 0644))

		chore.cleanupEmptyDirectories(ctx, satelliteID)

		for _, prefix := range emptyPrefixes {
			require.NoDirExists(t, filepath.Join(satelliteDir, prefix))
		}
		require.DirExists(t, activePath)
		require.DirExists(t, satelliteDir)

		chore.mu.Lock()
		require.Equal(t, int64(1), chore.migratingProgress[satelliteID].remainingDirectories)
		chore.mu.Unlock()
	})

	t.Run("removes satellite directory when all prefixes gone", func(t *testing.T) {
		chore, satelliteDir := newChore(t)

		// One prefix with a file, one empty.
		require.NoError(t, os.MkdirAll(filepath.Join(satelliteDir, "aa"), 0755))
		activePath := filepath.Join(satelliteDir, "bb")
		require.NoError(t, os.MkdirAll(activePath, 0755))
		activeFile := filepath.Join(activePath, "piece.sj1")
		require.NoError(t, os.WriteFile(activeFile, []byte("data"), 0644))

		// First cleanup: removes "aa", keeps "bb".
		chore.cleanupEmptyDirectories(ctx, satelliteID)
		require.NoDirExists(t, filepath.Join(satelliteDir, "aa"))
		require.DirExists(t, activePath)

		// Simulate migration completing: remove the file.
		require.NoError(t, os.Remove(activeFile))

		// Second cleanup: removes "bb" and the satellite dir.
		chore.cleanupEmptyDirectories(ctx, satelliteID)
		require.NoDirExists(t, activePath)
		require.NoDirExists(t, satelliteDir)

		chore.mu.Lock()
		require.Equal(t, int64(0), chore.migratingProgress[satelliteID].remainingDirectories)
		chore.mu.Unlock()
	})

	t.Run("handles missing satellite directory", func(t *testing.T) {
		chore, _ := newChore(t)

		// Don't create any directories. Should not panic.
		chore.cleanupEmptyDirectories(ctx, satelliteID)

		chore.mu.Lock()
		require.Equal(t, int64(0), chore.migratingProgress[satelliteID].remainingDirectories)
		chore.mu.Unlock()
	})

	t.Run("ignores non-prefix entries", func(t *testing.T) {
		chore, satelliteDir := newChore(t)

		// Create a valid empty prefix dir and a non-prefix entry (wrong name length).
		require.NoError(t, os.MkdirAll(filepath.Join(satelliteDir, "aa"), 0755))
		require.NoError(t, os.MkdirAll(filepath.Join(satelliteDir, "trash"), 0755))

		chore.cleanupEmptyDirectories(ctx, satelliteID)

		require.NoDirExists(t, filepath.Join(satelliteDir, "aa"))
		// "trash" is not a 2-char prefix, so it's ignored (not removed).
		require.DirExists(t, filepath.Join(satelliteDir, "trash"))
	})

	t.Run("skips cleanup when still writing to old backend", func(t *testing.T) {
		chore, satelliteDir := newChore(t)
		chore.writeChecker = &staticWriteStateChecker{writingToNew: false}

		prefixes := []string{"aa", "bb"}
		for _, prefix := range prefixes {
			require.NoError(t, os.MkdirAll(filepath.Join(satelliteDir, prefix), 0755))
		}

		chore.cleanupEmptyDirectories(ctx, satelliteID)

		// Directories must not be removed while writes still go to the old backend.
		for _, prefix := range prefixes {
			require.DirExists(t, filepath.Join(satelliteDir, prefix))
		}
	})

	t.Run("runs cleanup when writing to new backend", func(t *testing.T) {
		chore, satelliteDir := newChore(t)
		chore.writeChecker = &staticWriteStateChecker{writingToNew: true}

		prefixes := []string{"aa", "bb"}
		for _, prefix := range prefixes {
			require.NoError(t, os.MkdirAll(filepath.Join(satelliteDir, prefix), 0755))
		}

		chore.cleanupEmptyDirectories(ctx, satelliteID)

		for _, prefix := range prefixes {
			require.NoDirExists(t, filepath.Join(satelliteDir, prefix))
		}
	})

	t.Run("handles empty oldBlobsPath", func(t *testing.T) {
		chore := &Chore{
			log:               log,
			config:            Config{CleanupEmptyDirs: true},
			oldBlobsPath:      "",
			migratingActive:   make(map[storj.NodeID]bool),
			migratingProgress: make(map[storj.NodeID]*migrationProgress),
		}

		// Should not panic with empty path.
		chore.cleanupEmptyDirectories(ctx, satelliteID)
	})

	t.Run("cleans old zero-sized file debris", func(t *testing.T) {
		chore, satelliteDir := newChore(t)

		prefixPath := filepath.Join(satelliteDir, "aa")
		require.NoError(t, os.MkdirAll(prefixPath, 0755))

		// Create zero-sized files and make them old enough to delete.
		oldEnough := time.Now().Add(-2 * minZeroFileAge)
		for _, name := range []string{"zero1.sj1", "zero2.sj1"} {
			f := filepath.Join(prefixPath, name)
			require.NoError(t, os.WriteFile(f, []byte{}, 0644))
			require.NoError(t, os.Chtimes(f, oldEnough, oldEnough))
		}

		chore.cleanupEmptyDirectories(ctx, satelliteID)

		require.NoDirExists(t, prefixPath, "prefix with only old zero-sized files should be removed")
		require.NoDirExists(t, satelliteDir, "satellite dir should be removed when empty")

		chore.mu.Lock()
		require.Equal(t, int64(0), chore.migratingProgress[satelliteID].remainingDirectories)
		chore.mu.Unlock()
	})

	t.Run("preserves recent zero-sized files", func(t *testing.T) {
		chore, satelliteDir := newChore(t)

		prefixPath := filepath.Join(satelliteDir, "aa")
		require.NoError(t, os.MkdirAll(prefixPath, 0755))

		// Create a zero-sized file that was just created (too recent to delete).
		recentFile := filepath.Join(prefixPath, "recent.sj1")
		require.NoError(t, os.WriteFile(recentFile, []byte{}, 0644))

		chore.cleanupEmptyDirectories(ctx, satelliteID)

		require.FileExists(t, recentFile, "recent zero-sized file should be preserved")
		require.DirExists(t, prefixPath, "prefix with recent zero-sized file should remain")

		chore.mu.Lock()
		require.Equal(t, int64(1), chore.migratingProgress[satelliteID].remainingDirectories)
		chore.mu.Unlock()
	})

	t.Run("does not remove dir with mixed zero-sized and real files", func(t *testing.T) {
		chore, satelliteDir := newChore(t)

		prefixPath := filepath.Join(satelliteDir, "aa")
		require.NoError(t, os.MkdirAll(prefixPath, 0755))

		// Create an old zero-sized file and a real piece file.
		oldEnough := time.Now().Add(-2 * minZeroFileAge)
		zeroFile := filepath.Join(prefixPath, "zero.sj1")
		require.NoError(t, os.WriteFile(zeroFile, []byte{}, 0644))
		require.NoError(t, os.Chtimes(zeroFile, oldEnough, oldEnough))

		realFile := filepath.Join(prefixPath, "piece.sj1")
		require.NoError(t, os.WriteFile(realFile, []byte("data"), 0644))

		chore.cleanupEmptyDirectories(ctx, satelliteID)

		// Real file should remain; directory should not be removed.
		require.FileExists(t, realFile, "real piece file should remain")
		require.DirExists(t, prefixPath, "prefix with real files should remain")

		chore.mu.Lock()
		require.Equal(t, int64(1), chore.migratingProgress[satelliteID].remainingDirectories)
		chore.mu.Unlock()
	})
}
