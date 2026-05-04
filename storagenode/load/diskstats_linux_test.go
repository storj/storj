// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package load

import (
	"os"
	"testing"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// realDiskDir returns a directory that is on a real block device (not tmpfs).
// Skips the test if no suitable block device can be found (e.g. in containers).
func realDiskDir(t *testing.T) string {
	t.Helper()
	for _, dir := range []string{"/home", "/"} {
		name, err := deviceNameFromPath(dir)
		if err == nil && name != "" {
			return dir
		}
	}
	cwd, err := os.Getwd()
	require.NoError(t, err)
	name, err := deviceNameFromPath(cwd)
	if err == nil && name != "" {
		return cwd
	}
	t.Skip("no block device with /proc/diskstats entry found (container/overlay environment)")
	return ""
}

func TestDeviceNameFromPath(t *testing.T) {
	dir := realDiskDir(t)
	name, err := deviceNameFromPath(dir)
	require.NoError(t, err)
	require.NotEmpty(t, name)
	t.Logf("device for %s: %s", dir, name)
}

func TestReadDiskStats(t *testing.T) {
	dir := realDiskDir(t)
	name, err := deviceNameFromPath(dir)
	require.NoError(t, err)

	stats, err := readDiskStats(name)
	require.NoError(t, err)
	require.NotZero(t, stats.ReadsCompleted, "expected non-zero reads on the device")
	t.Logf("raw stats for %s: %+v", name, stats)
}

func TestDiskStatsSource(t *testing.T) {
	logger := zaptest.NewLogger(t)
	dir := realDiskDir(t)
	source := DiskStats(logger, dir)

	// First call establishes baseline only; values are all zero.
	collected := map[string]float64{}
	source.Stats(func(key monkit.SeriesKey, field string, val float64) {
		collected[field] = val
	})
	require.Len(t, collected, 9, "first call should emit zero-value fields")

	// Second call computes deltas from the baseline.
	collected = map[string]float64{}
	source.Stats(func(key monkit.SeriesKey, field string, val float64) {
		collected[field] = val
	})

	expectedFields := []string{
		"reads_per_sec",
		"writes_per_sec",
		"read_bytes_per_sec",
		"write_bytes_per_sec",
		"read_await_ms",
		"write_await_ms",
		"avg_queue_size",
		"ios_in_progress",
		"utilization_pct",
	}
	for _, f := range expectedFields {
		_, ok := collected[f]
		require.True(t, ok, "missing field %q", f)
	}
	t.Logf("disk stats: %+v", collected)
}

func TestDiskStatsUnsupportedPath(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// /proc is not backed by a block device. Stats should silently produce nothing.
	source := DiskStats(logger, "/proc")

	collected := map[string]float64{}
	source.Stats(func(key monkit.SeriesKey, field string, val float64) {
		collected[field] = val
	})
	require.Empty(t, collected, "unsupported path should not emit stats")
}
