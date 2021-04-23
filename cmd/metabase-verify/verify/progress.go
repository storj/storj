// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package verify

import (
	"context"
	"runtime"

	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/storj/satellite/metabase/metaloop"
)

// ProgressObserver counts and prints progress of metabase loop.
type ProgressObserver struct {
	Log *zap.Logger

	ProgressPrintFrequency int64

	ObjectCount        int64
	RemoteSegmentCount int64
	InlineSegmentCount int64
}

// Report reports the current progress.
func (progress *ProgressObserver) Report() {
	progress.Log.Debug("progress",
		zap.Int64("objects", progress.ObjectCount),
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

// Object implements the Observer interface.
func (progress *ProgressObserver) Object(context.Context, *metaloop.Object) error {
	progress.ObjectCount++
	if progress.ObjectCount%progress.ProgressPrintFrequency == 0 {
		progress.Report()
	}
	return nil
}

// RemoteSegment implements the Observer interface.
func (progress *ProgressObserver) RemoteSegment(context.Context, *metaloop.Segment) error {
	progress.RemoteSegmentCount++
	return nil
}

// InlineSegment implements the Observer interface.
func (progress *ProgressObserver) InlineSegment(context.Context, *metaloop.Segment) error {
	progress.InlineSegmentCount++
	return nil
}

// LoopStarted is called at each start of a loop.
func (progress *ProgressObserver) LoopStarted(ctx context.Context, info metaloop.LoopInfo) (err error) {
	return nil
}
