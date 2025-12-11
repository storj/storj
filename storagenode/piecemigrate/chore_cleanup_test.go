// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package piecemigrate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/storagenode/blobstore/filestore"
)

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
}
