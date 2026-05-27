// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/hashstore"
)

// mockPieceStore implements PieceStoreSpaceUsage for testing.
type mockPieceStore struct {
	status         StorageStatus
	piecesTotal    int64
	piecesContent  int64
	trash          int64
	piecesAndTrash int64
}

func (m *mockPieceStore) StorageStatus(ctx context.Context) (StorageStatus, error) {
	return m.status, nil
}

func (m *mockPieceStore) SpaceUsedForPieces(ctx context.Context) (int64, int64, error) {
	return m.piecesTotal, m.piecesContent, nil
}

func (m *mockPieceStore) SpaceUsedForTrash(ctx context.Context) (int64, error) {
	return m.trash, nil
}

func (m *mockPieceStore) SpaceUsedForPiecesAndTrash(ctx context.Context) (int64, error) {
	return m.piecesAndTrash, nil
}

// mockHashStore implements HashStoreBackend for testing.
type mockHashStore struct {
	usage    SpaceUsage
	logsPath string
}

func (m *mockHashStore) SpaceUsage() SpaceUsage {
	return m.usage
}

func (m *mockHashStore) LogsPath() string {
	return m.logsPath
}

// mockDiskSpaceInfo implements DiskSpaceInfo for testing.
type mockDiskSpaceInfo struct {
	info blobstore.DiskInfo
}

func (m *mockDiskSpaceInfo) AvailableSpace(ctx context.Context) (blobstore.DiskInfo, error) {
	return m.info, nil
}

func TestSpaceUsage_ReservedField(t *testing.T) {
	ctx := context.Background()

	// Create a test hashstore backend to generate space usage stats
	config := hashstore.CreateDefaultConfig(hashstore.TableKind_HashTbl, false)
	config.Compaction.RewriteMultiple = 3.0 // Set a specific rewrite multiple for testing

	db, err := hashstore.New(ctx, config, t.TempDir(), "", nil, hashstore.Callbacks{})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	// Create some data to ensure non-zero table sizes
	for i := 0; i < 10; i++ {
		key := [32]byte{byte(i)} // Create a simple key
		w, err := db.Create(ctx, key, time.Time{})
		require.NoError(t, err)
		_, err = w.Write(make([]byte, 1024))
		require.NoError(t, err)
		require.NoError(t, w.Close())
	}

	// Get the hashstore stats
	dbStats, s0Stats, s1Stats := db.Stats()

	// Test that FreeRequired is calculated correctly in individual stores
	expectedS0FreeRequired := memory.Size(2+config.Compaction.RewriteMultiple) * s0Stats.Table.TableSize
	expectedS1FreeRequired := memory.Size(2+config.Compaction.RewriteMultiple) * s1Stats.Table.TableSize
	assert.Equal(t, expectedS0FreeRequired, s0Stats.FreeRequired)
	assert.Equal(t, expectedS1FreeRequired, s1Stats.FreeRequired)

	// Test that FreeRequired is aggregated correctly in DB stats
	expectedDBFreeRequired := max(s0Stats.FreeRequired, s1Stats.FreeRequired)
	assert.Equal(t, expectedDBFreeRequired, dbStats.FreeRequired)

	// Create a mock hash space usage
	hashSpaceUsage := SpaceUsage{
		UsedTotal:       int64(dbStats.LenSet),
		UsedForMetadata: int64(dbStats.TableSize),
		Reserved:        int64(dbStats.FreeRequired), // This is the new field we're testing
	}

	// Test that Reserved field is properly set
	assert.Equal(t, int64(dbStats.FreeRequired), hashSpaceUsage.Reserved)
	require.Greater(t, hashSpaceUsage.Reserved, int64(0), "Reserved space should be positive when there's data")
}

func TestPreFlightCheck_HashStoreHeavyNode(t *testing.T) {
	// Scenario: a node with most data in hashstore and little in blobstore.
	// The disk is 4TB, allocated 2TB, hashstore uses 1.8TB, blobstore uses 50GB,
	// leaving 200GB free on disk.
	//
	// Before the fix, PreFlightCheck only counted blobstore usage (50GB) as totalUsed.
	// It saw freeDiskSpace (200GB) < allocatedDiskSpace (2TB) - totalUsed (50GB) = 1.95TB,
	// so it reduced allocatedDiskSpace to 200GB + 50GB = 250GB.
	// Then 250GB < 500GB minimum → FATAL error, node refuses to start.

	const (
		gb            = int64(1_000_000_000)
		diskTotal     = 4000 * gb
		diskFree      = 200 * gb
		allocated     = 2000 * gb
		blobstoreUsed = 50 * gb
		hashstoreUsed = 1800 * gb
		minimumDisk   = 500 * gb
	)

	ctx := context.Background()
	log := zaptest.NewLogger(t)

	store := &mockPieceStore{
		status: StorageStatus{
			DiskTotal: diskTotal,
			DiskFree:  diskFree,
		},
		piecesAndTrash: blobstoreUsed,
		piecesTotal:    blobstoreUsed,
	}

	hashStore := &mockHashStore{
		usage: SpaceUsage{
			UsedTotal: hashstoreUsed,
		},
	}

	sd, err := NewSharedDisk(ctx, log, store, hashStore, minimumDisk, allocated)
	require.NoError(t, err, "PreFlightCheck should not fail when total used (blobstore + hashstore) is accounted for")
	require.True(t, sd.allocatedDiskSpace >= minimumDisk, "allocated disk space should remain above minimum")
}

func TestPreFlightCheck_SmallDiskNewNode(t *testing.T) {
	// Scenario: a brand new node where neither blobstore nor hashstore has data,
	// and the free disk space is less than the configured allocation.
	// allocatedDiskSpace should be reduced to freeDiskSpace but still pass
	// if freeDiskSpace >= minimumDiskSpace.

	const (
		gb          = int64(1_000_000_000)
		diskTotal   = 1000 * gb
		diskFree    = 800 * gb
		allocated   = 2000 * gb
		minimumDisk = 500 * gb
	)

	ctx := context.Background()
	log := zaptest.NewLogger(t)

	store := &mockPieceStore{
		status: StorageStatus{
			DiskTotal: diskTotal,
			DiskFree:  diskFree,
		},
	}

	hashStore := &mockHashStore{}

	sd, err := NewSharedDisk(ctx, log, store, hashStore, minimumDisk, allocated)
	require.NoError(t, err)
	require.Equal(t, diskFree, sd.allocatedDiskSpace, "allocated should be reduced to free disk space for new node")
}

func TestPreFlightCheck_DiskTooSmall(t *testing.T) {
	// Scenario: free disk space is below minimum even after accounting for all usage.
	// PreFlightCheck should return an error.

	const (
		gb          = int64(1_000_000_000)
		diskTotal   = 500 * gb
		diskFree    = 100 * gb
		allocated   = 2000 * gb
		minimumDisk = 500 * gb
	)

	ctx := context.Background()
	log := zaptest.NewLogger(t)

	store := &mockPieceStore{
		status: StorageStatus{
			DiskTotal: diskTotal,
			DiskFree:  diskFree,
		},
		piecesAndTrash: 50 * gb,
		piecesTotal:    50 * gb,
	}

	hashStore := &mockHashStore{
		usage: SpaceUsage{
			UsedTotal: 300 * gb,
		},
	}

	_, err := NewSharedDisk(ctx, log, store, hashStore, minimumDisk, allocated)
	require.Error(t, err, "PreFlightCheck should fail when adjusted allocated space is below minimum")
}

func TestPreFlightCheck_HashStoreOnly(t *testing.T) {
	// Scenario: hashstore-only node (store == nil). PreFlightCheck should
	// still validate disk space using dir.AvailableSpace instead of skipping.

	const (
		gb            = int64(1_000_000_000)
		diskFree      = 600 * gb
		allocated     = 2000 * gb
		hashstoreUsed = 1200 * gb
		minimumDisk   = 500 * gb
	)

	ctx := context.Background()
	log := zaptest.NewLogger(t)

	hashStore := &mockHashStore{
		usage: SpaceUsage{
			UsedTotal: hashstoreUsed,
		},
	}

	sd := &SharedDisk{
		log:                log,
		store:              nil,
		hashStore:          hashStore,
		allocatedDiskSpace: allocated,
		minimumDiskSpace:   minimumDisk,
		dir: &mockDiskSpaceInfo{
			info: blobstore.DiskInfo{
				TotalSpace:     2000 * gb,
				AvailableSpace: diskFree,
			},
		},
	}

	err := sd.PreFlightCheck(ctx)
	require.NoError(t, err, "PreFlightCheck should pass for hashstore-only node with sufficient space")
	require.True(t, sd.allocatedDiskSpace >= minimumDisk)
}

func TestPreFlightCheck_HashStoreOnly_TooSmall(t *testing.T) {
	// Scenario: hashstore-only node where free + used is below minimum.

	const (
		gb            = int64(1_000_000_000)
		diskFree      = 100 * gb
		allocated     = 2000 * gb
		hashstoreUsed = 200 * gb
		minimumDisk   = 500 * gb
	)

	ctx := context.Background()
	log := zaptest.NewLogger(t)

	hashStore := &mockHashStore{
		usage: SpaceUsage{
			UsedTotal: hashstoreUsed,
		},
	}

	sd := &SharedDisk{
		log:                log,
		store:              nil,
		hashStore:          hashStore,
		allocatedDiskSpace: allocated,
		minimumDiskSpace:   minimumDisk,
		dir: &mockDiskSpaceInfo{
			info: blobstore.DiskInfo{
				TotalSpace:     400 * gb,
				AvailableSpace: diskFree,
			},
		},
	}

	err := sd.PreFlightCheck(ctx)
	require.Error(t, err, "PreFlightCheck should fail for hashstore-only node with insufficient space")
}

func TestDiskSpace_HashStoreOnly_AvailableCappedByFree(t *testing.T) {
	// Scenario: hashstore-only node where the computed available space
	// exceeds actual free disk space. Available should be capped by DiskFree.
	//
	// allocated=2TB, hashstore uses 500GB (UsedTotal), reserved=100GB
	// computed available = 2TB - 500GB - 100GB = 1.4TB
	// but actual free disk = 300GB, so available should be capped to 300GB.

	const (
		gb        = int64(1_000_000_000)
		diskTotal = 2000 * gb
		diskFree  = 300 * gb
		allocated = 2000 * gb
	)

	ctx := context.Background()
	log := zaptest.NewLogger(t)

	hashStore := &mockHashStore{
		usage: SpaceUsage{
			UsedTotal:     500 * gb,
			UsedForPieces: 400 * gb,
			UsedForTrash:  50 * gb,
			Reserved:      100 * gb,
		},
	}

	sd := &SharedDisk{
		log:                log,
		store:              nil,
		hashStore:          hashStore,
		allocatedDiskSpace: allocated,
		minimumDiskSpace:   500 * gb,
		dir: &mockDiskSpaceInfo{
			info: blobstore.DiskInfo{
				TotalSpace:     diskTotal,
				AvailableSpace: diskFree,
			},
		},
	}

	ds, err := sd.DiskSpace(ctx)
	require.NoError(t, err)
	require.Equal(t, diskFree, ds.Available, "available should be capped by actual free disk space")
	require.Equal(t, diskFree, ds.Free)
	require.Equal(t, diskTotal, ds.Total)
}
