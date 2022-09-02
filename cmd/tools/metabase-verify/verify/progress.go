// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package verify

import (
	"context"
	"runtime"

	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/storj/satellite/metabase/segmentloop"
)

// ProgressObserver counts and prints progress of metabase loop.
type ProgressObserver struct {
	Log *zap.Logger

	ProgressPrintFrequency int64

	RemoteSegmentCount int64
	InlineSegmentCount int64
}

// Report reports the current progress.
func (progress *ProgressObserver) Report() {
	progress.Log.Debug("progress",
		zap.Int64("remote segments", progress.RemoteSegmentCount),
		zap.Int64("inline segments", progress.InlineSegmentCount),
	)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	progress.Log.Debug("memory",
		zap.String("Alloc", memory.Size(int64(m.Alloc)).String()),
		zap.String("TotalAlloc", memory.Size(int64(m.TotalAlloc)).String()),
		zap.String("Sys", memory.Size(int64(m.Sys)).String()),
		zap.Uint32("NumGC", m.NumGC),
	)
}

// RemoteSegment implements the Observer interface.
func (progress *ProgressObserver) RemoteSegment(context.Context, *segmentloop.Segment) error {
	progress.RemoteSegmentCount++
	if (progress.RemoteSegmentCount+progress.InlineSegmentCount)%progress.ProgressPrintFrequency == 0 {
		progress.Report()
	}
	return nil
}

// InlineSegment implements the Observer interface.
func (progress *ProgressObserver) InlineSegment(context.Context, *segmentloop.Segment) error {
	progress.InlineSegmentCount++
	if (progress.RemoteSegmentCount+progress.InlineSegmentCount)%progress.ProgressPrintFrequency == 0 {
		progress.Report()
	}
	return nil
}

// LoopStarted is called at each start of a loop.
func (progress *ProgressObserver) LoopStarted(ctx context.Context, info segmentloop.LoopInfo) (err error) {
	return nil
}
