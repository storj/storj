// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/metainfo"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("checker error")
	mon   = monkit.Package()
)

// Config contains configurable values for checker
type Config struct {
	Interval            time.Duration `help:"how frequently checker should check for bad segments" releaseDefault:"30s" devDefault:"0h0m10s"`
	IrreparableInterval time.Duration `help:"how frequently irrepairable checker should check for lost pieces" releaseDefault:"30m" devDefault:"0h0m5s"`
}

// durabilityStats remote segment information
type durabilityStats struct {
	remoteFilesChecked          int64
	remoteSegmentsChecked       int64
	remoteSegmentsNeedingRepair int64
	remoteSegmentsLost          int64
	remoteSegmentInfo           map[string]struct{}
}

// Checker contains the information needed to do checks for missing pieces
type Checker struct {
	metainfo        *metainfo.Service
	lastChecked     metainfo.Path
	repairQueue     queue.RepairQueue
	overlay         *overlay.Cache
	irrdb           irreparable.DB
	logger          *zap.Logger
	Loop            sync2.Cycle
	IrreparableLoop sync2.Cycle
}

// NewChecker creates a new instance of checker
func NewChecker(metainfoSrv *metainfo.Service, repairQueue queue.RepairQueue, overlay *overlay.Cache, irrdb irreparable.DB, limit int, logger *zap.Logger, repairInterval, irreparableInterval time.Duration) *Checker {
	// TODO: reorder arguments
	checker := &Checker{
		metainfo:        metainfoSrv,
		lastChecked:     metainfo.Path{},
		repairQueue:     repairQueue,
		overlay:         overlay,
		irrdb:           irrdb,
		logger:          logger,
		Loop:            *sync2.NewCycle(repairInterval),
		IrreparableLoop: *sync2.NewCycle(irreparableInterval),
	}
	return checker
}

// Run the checker loop
func (checker *Checker) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return checker.Loop.Run(ctx, checker.IdentifyInjuredSegments)
	})

	group.Go(func() error {
		return checker.IrreparableLoop.Run(ctx, checker.IrreparableProcess)
	})

	return group.Wait()
}

// Close halts the Checker loop
func (checker *Checker) Close() error {
	checker.Loop.Close()
	return nil
}

// IdentifyInjuredSegments checks for missing pieces off of the metainfo and overlay cache
func (checker *Checker) IdentifyInjuredSegments(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var monStats durabilityStats
	monStats.remoteSegmentInfo = make(map[string]struct{})

	defer func() {
		// send durability stats
		mon.IntVal("remote_files_checked").Observe(monStats.remoteFilesChecked)
		mon.IntVal("remote_segments_checked").Observe(monStats.remoteSegmentsChecked)
		mon.IntVal("remote_segments_needing_repair").Observe(monStats.remoteSegmentsNeedingRepair)
		mon.IntVal("remote_segments_lost").Observe(monStats.remoteSegmentsLost)
		mon.IntVal("remote_files_lost").Observe(int64(len(monStats.remoteSegmentInfo)))
	}()

	// TODO: we want to startAfter checker.lastChecked, but metainfo.Iterate doesn't support
	// a startAfter argument. so we have to fake it and skip the first loop
	skipNext := !checker.lastChecked.Equal(metainfo.Path{})

	err = checker.metainfo.Iterate(ctx, metainfo.Path{}, checker.lastChecked, true, false,
		func(ctx context.Context, it metainfo.Iterator) (err error) {
			var item metainfo.ListItem
			for it.Next(ctx, &item) {
				if skipNext {
					if item.Path.Equal(checker.lastChecked) {
						continue
					}
					skipNext = false
				}
				checker.lastChecked = item.Path
				err = checker.updateSegmentStatus(ctx, item.Pointer, item.Path, &monStats)
				if err != nil {
					return err
				}
			}
			return nil
		},
	)
	if err != nil {
		return err
	}

	checker.lastChecked = metainfo.Path{}
	return nil
}

// checks for a string in slice
func contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

func (checker *Checker) updateSegmentStatus(ctx context.Context, pointer *pb.Pointer, path metainfo.Path, monStats *durabilityStats) (err error) {
	defer mon.Task()(&ctx)(&err)
	remote := pointer.GetRemote()
	if remote == nil {
		return nil
	}

	pieces := remote.GetRemotePieces()
	if pieces == nil {
		checker.logger.Debug("no pieces on remote segment")
		return nil
	}

	missingPieces, err := checker.overlay.GetMissingPieces(ctx, pieces)
	if err != nil {
		return Error.New("error getting missing pieces %s", err)
	}

	monStats.remoteSegmentsChecked++
	if path.SegmentIndex() == -1 {
		monStats.remoteFilesChecked++
	}

	numHealthy := int32(len(pieces) - len(missingPieces))
	redundancy := pointer.Remote.Redundancy
	// we repair when the number of healthy files is less than or equal to the repair threshold
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
	if numHealthy > redundancy.MinReq && numHealthy <= redundancy.RepairThreshold && numHealthy < redundancy.SuccessThreshold {
		if len(missingPieces) == 0 {
			checker.logger.Warn("Missing pieces is zero in checker, but this should be impossible -- bad redundancy scheme.")
			return nil
		}
		monStats.remoteSegmentsNeedingRepair++
		err = checker.repairQueue.Insert(ctx, &pb.InjuredSegment{
			Path:       path.String(),
			LostPieces: missingPieces,
		})
		if err != nil {
			return Error.Wrap(err)
		}

		// delete always returns nil when something was deleted and also when element didn't exists
		err = checker.irrdb.Delete(ctx, path.Raw())
		if err != nil {
			checker.logger.Error("error deleting entry from irreparable db: ", zap.Error(err))
		}
		// we need one additional piece for error correction. If only the minimum is remaining the file can't be repaired and is lost.
		// except for the case when minimum and repair thresholds are the same (a case usually seen during testing)
	} else if numHealthy <= redundancy.MinReq && numHealthy < redundancy.RepairThreshold {
		// check to make sure there's an object path and add it to the remote segment info if necessary
		if _, ok := path.Bucket(); ok && path.EncryptedPath().Valid() {
			objectPath := path.ObjectString()
			if _, ok := monStats.remoteSegmentInfo[objectPath]; !ok {
				monStats.remoteSegmentInfo[objectPath] = struct{}{}
			}
		}

		monStats.remoteSegmentsLost++
		// make an entry in to the irreparable table
		segmentInfo := &pb.IrreparableSegment{
			Path:               path.Raw(),
			SegmentDetail:      pointer,
			LostPieces:         int32(len(missingPieces)),
			LastRepairAttempt:  time.Now().Unix(),
			RepairAttemptCount: int64(1),
		}

		// add the entry if new or update attempt count if already exists
		err := checker.irrdb.IncrementRepairAttempts(ctx, segmentInfo)
		if err != nil {
			return Error.New("error handling irreparable segment to queue %s", err)
		}
	}
	return nil
}

// IrreparableProcess picks items from irrepairabledb and spawns a repair worker
func (checker *Checker) IrreparableProcess(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	limit := 1
	var offset int64

	for {
		seg, err := checker.irrdb.GetLimited(ctx, limit, offset)
		if err != nil {
			return Error.New("error reading segment from the queue %s", err)
		}

		// zero segments returned with nil err
		if len(seg) == 0 {
			break
		}

		path, err := metainfo.ParsePath(seg[0].GetPath())
		if err != nil {
			return err
		}

		err = checker.updateSegmentStatus(ctx, seg[0].GetSegmentDetail(), path, &durabilityStats{})
		if err != nil {
			checker.logger.Error("irrepair segment checker failed: ", zap.Error(err))
		}
		offset++
	}

	return nil
}
