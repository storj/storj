// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"errors"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	flag "github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/memory"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metaloop"
)

var mon = monkit.Package()

// Error is the default error class for the package.
var Error = errs.Class("metaloop-benchmark")

// Bench benchmarks metainfo loop performance.
type Bench struct {
	CPUProfile string
	MetabaseDB string

	IgnoreVersionMismatch bool

	ProgressPrintFrequency int64

	Loop metaloop.Config
}

// BindFlags adds bench flags to the the flagset.
func (bench *Bench) BindFlags(flag *flag.FlagSet) {
	flag.StringVar(&bench.CPUProfile, "cpuprofile", "", "write cpu profile to file")
	flag.StringVar(&bench.MetabaseDB, "metabasedb", "", "connection URL for MetabaseDB")

	flag.BoolVar(&bench.IgnoreVersionMismatch, "ignore-version-mismatch", false, "ignore version mismatch")

	flag.Int64Var(&bench.ProgressPrintFrequency, "progress.frequency", 1000000, "how often should we print progress (every object)")

	flag.DurationVar(&bench.Loop.CoalesceDuration, "loop.coalesce-duration", 5*time.Second, "how long to wait for new observers before starting iteration")
	flag.Float64Var(&bench.Loop.RateLimit, "loop.rate-limit", 0, "rate limit (default is 0 which is unlimited segments per second)")
	flag.IntVar(&bench.Loop.ListLimit, "loop.list-limit", 2500, "how many items to query in a batch")
}

// VerifyFlags verifies whether the values provided are valid.
func (bench *Bench) VerifyFlags() error {
	var errlist errs.Group
	if bench.MetabaseDB == "" {
		errlist.Add(errors.New("flag '--metabasedb' is not set"))
	}
	return errlist.Err()
}

// Run runs the benchmark.
func (bench *Bench) Run(ctx context.Context, log *zap.Logger) (err error) {
	defer mon.Task()(&ctx)(&err)

	// setup profiling

	if bench.CPUProfile != "" {
		f, err := os.Create(bench.CPUProfile)
		if err != nil {
			return err
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}

	// setup databases

	mdb, err := metabase.Open(ctx, log.Named("mdb"), bench.MetabaseDB)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { _ = mdb.Close() }()

	checkMetabase := mdb.CheckVersion(ctx)

	if checkMetabase != nil {
		log.Error("versions skewed", zap.Any("metabase version", checkMetabase))
		if !bench.IgnoreVersionMismatch {
			return checkMetabase
		}
	}

	// setup metainfo loop

	var group errs2.Group

	loop := metaloop.New(bench.Loop, mdb)

	group.Go(func() error {
		progress := &ProgressObserver{
			Log:                    log.Named("progress"),
			ProgressPrintFrequency: bench.ProgressPrintFrequency,
		}
		err := loop.Join(ctx, progress)
		progress.Report()
		return Error.Wrap(err)
	})

	group.Go(func() error {
		err := loop.RunOnce(ctx)
		return Error.Wrap(err)
	})

	// wait for loop to finish
	if allErrors := group.Wait(); len(allErrors) > 0 {
		return Error.Wrap(errs.Combine(allErrors...))
	}

	return nil
}

// ProgressObserver counts and prints progress of metainfo loop.
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
