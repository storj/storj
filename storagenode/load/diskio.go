// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package load

import (
	"sync"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"
)

// Stats represents cumulative counts of read/write operations, read/write bytes, and time spent
// doing reads and writes on a particular block device.
type Stats struct {
	ReadCount  uint64
	WriteCount uint64
	ReadBytes  uint64
	WriteBytes uint64
}

var logStatFailure sync.Once

// DiskIO creates a monkit StatSource that can yield disk IO statistics about the device
// underlying the specified device.
func DiskIO(logger *zap.Logger, pid int32) monkit.StatSource {
	return monkit.StatSourceFunc(func(cb func(key monkit.SeriesKey, field string, val float64)) {
		var stats Stats
		err := stats.Get(pid)
		if err != nil {
			logStatFailure.Do(func() {
				logger.Debug("could not get disk i/o stats", zap.Error(err))
			})
			return
		}
		monkit.StatSourceFromStruct(monkit.NewSeriesKey("diskio"), &stats).Stats(cb)
	})
}

// Get collects I/O statistics for the current process (if implemented in gopsutil).
func (s *Stats) Get(pid int32) error {
	p, err := process.NewProcess(pid)
	if err != nil {
		return err
	}
	counters, err := p.IOCounters()
	if err != nil {
		return err
	}
	s.ReadCount = counters.ReadCount
	s.WriteCount = counters.WriteCount
	s.ReadBytes = counters.ReadBytes
	s.WriteBytes = counters.WriteBytes
	return nil
}
