// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package load

import (
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"
)

// DiskStatsReport holds computed iostat-like metrics for a block device.
type DiskStatsReport struct {
	// ReadsPerSec is the number of read operations completed per second.
	ReadsPerSec float64
	// WritesPerSec is the number of write operations completed per second.
	WritesPerSec float64
	// ReadBytesPerSec is the number of bytes read per second (derived from sectors, 512 bytes each).
	ReadBytesPerSec float64
	// WriteBytesPerSec is the number of bytes written per second.
	WriteBytesPerSec float64
	// ReadAwaitMs is the average time (in milliseconds) for read requests to be served.
	ReadAwaitMs float64
	// WriteAwaitMs is the average time (in milliseconds) for write requests to be served.
	WriteAwaitMs float64
	// AvgQueueSize is the average I/O queue length (weighted time doing I/O / elapsed time).
	AvgQueueSize float64
	// IOsInProgress is the number of I/Os currently in flight (instantaneous snapshot).
	IOsInProgress float64
	// UtilizationPct is the percentage of time the device was busy (0-100).
	UtilizationPct float64
}

const sectorSize = 512

// DiskStats creates a monkit StatSource that yields iostat-like disk I/O statistics
// for the block device underlying the given directory path.
// It samples /proc/diskstats on each Stats() call and computes deltas from the previous sample.
func DiskStats(logger *zap.Logger, dir string) *DiskStatsCollector {
	ds := &DiskStatsCollector{
		logger: logger,
		dir:    dir,
	}
	return ds
}

// DiskStatsCollector reads /proc/diskstats and reports per-device I/O metrics via monkit.
type DiskStatsCollector struct {
	logger     *zap.Logger
	dir        string
	initOnce   sync.Once
	deviceName string
	initErr    error

	mu       sync.Mutex
	lastTime time.Time
	lastRaw  rawDiskStats
	report   DiskStatsReport
}

func (ds *DiskStatsCollector) init() {
	ds.deviceName, ds.initErr = deviceNameFromPath(ds.dir)
	if ds.initErr != nil {
		ds.logger.Warn("could not resolve block device for disk stats",
			zap.String("dir", ds.dir),
			zap.Error(ds.initErr))
		return
	}
	ds.logger.Info("disk stats monitoring initialized",
		zap.String("dir", ds.dir),
		zap.String("device", ds.deviceName))
}

func (ds *DiskStatsCollector) sample() {
	ds.initOnce.Do(ds.init)
	if ds.initErr != nil {
		return
	}

	raw, err := readDiskStats(ds.deviceName)
	if err != nil {
		ds.logger.Warn("could not read disk stats",
			zap.String("device", ds.deviceName),
			zap.Error(err))
		return
	}

	ds.mu.Lock()
	defer ds.mu.Unlock()

	now := time.Now()
	if ds.lastTime.IsZero() {
		// First sample, just store it.
		ds.lastTime = now
		ds.lastRaw = raw
		return
	}

	elapsed := now.Sub(ds.lastTime).Seconds()
	if elapsed <= 0 {
		return
	}

	prev := ds.lastRaw

	deltaReads := raw.ReadsCompleted - prev.ReadsCompleted
	deltaWrites := raw.WritesCompleted - prev.WritesCompleted
	deltaSectorsRead := raw.SectorsRead - prev.SectorsRead
	deltaSectorsWritten := raw.SectorsWritten - prev.SectorsWritten
	deltaMsReading := raw.MsReading - prev.MsReading
	deltaMsWriting := raw.MsWriting - prev.MsWriting
	deltaMsDoingIO := raw.MsDoingIO - prev.MsDoingIO
	deltaWeightedMs := raw.WeightedMsIO - prev.WeightedMsIO

	ds.report.ReadsPerSec = float64(deltaReads) / elapsed
	ds.report.WritesPerSec = float64(deltaWrites) / elapsed
	ds.report.ReadBytesPerSec = float64(deltaSectorsRead) * sectorSize / elapsed
	ds.report.WriteBytesPerSec = float64(deltaSectorsWritten) * sectorSize / elapsed

	if deltaReads > 0 {
		ds.report.ReadAwaitMs = float64(deltaMsReading) / float64(deltaReads)
	} else {
		ds.report.ReadAwaitMs = 0
	}
	if deltaWrites > 0 {
		ds.report.WriteAwaitMs = float64(deltaMsWriting) / float64(deltaWrites)
	} else {
		ds.report.WriteAwaitMs = 0
	}

	// Average queue size: weighted time in ms / elapsed time in ms.
	elapsedMs := elapsed * 1000
	ds.report.AvgQueueSize = float64(deltaWeightedMs) / elapsedMs

	// Current I/Os in progress (instantaneous, not a delta).
	ds.report.IOsInProgress = float64(raw.IOsInProgress)

	// Utilization: fraction of time the device had I/O in progress.
	ds.report.UtilizationPct = float64(deltaMsDoingIO) / elapsedMs * 100
	if ds.report.UtilizationPct > 100 {
		ds.report.UtilizationPct = 100
	}

	ds.lastTime = now
	ds.lastRaw = raw
}

// Stats implements monkit.StatSource.
func (ds *DiskStatsCollector) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	ds.sample()

	ds.initOnce.Do(ds.init)
	if ds.initErr != nil {
		return
	}

	ds.mu.Lock()
	r := ds.report
	ds.mu.Unlock()

	k := monkit.NewSeriesKey("disk_stats").WithTag("device", ds.deviceName)
	cb(k, "reads_per_sec", r.ReadsPerSec)
	cb(k, "writes_per_sec", r.WritesPerSec)
	cb(k, "read_bytes_per_sec", r.ReadBytesPerSec)
	cb(k, "write_bytes_per_sec", r.WriteBytesPerSec)
	cb(k, "read_await_ms", r.ReadAwaitMs)
	cb(k, "write_await_ms", r.WriteAwaitMs)
	cb(k, "avg_queue_size", r.AvgQueueSize)
	cb(k, "ios_in_progress", r.IOsInProgress)
	cb(k, "utilization_pct", r.UtilizationPct)
}
