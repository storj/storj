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
	"storj.io/storj/satellite/metainfoloop"
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

	ReliabilityCacheStaleness time.Duration `help:"how stale reliable node cache can be" releaseDefault:"5m" devDefault:"5m"`
}

// Checker contains the information needed to do checks for missing pieces
type Checker struct {
	metainfoloop    *metainfoloop.Service
	lastChecked     string
	repairQueue     queue.RepairQueue
	nodestate       *ReliabilityCache
	irrdb           irreparable.DB
	log             *zap.Logger
	Loop            sync2.Cycle
	IrreparableLoop sync2.Cycle
}

// NewChecker creates a new instance of checker
func NewChecker(metainfoloop *metainfoloop.Service, repairQueue queue.RepairQueue, overlay *overlay.Cache, irrdb irreparable.DB, limit int, log *zap.Logger, config Config) *Checker {
	// TODO: reorder arguments
	return &Checker{
		metainfoloop:    metainfoloop,
		lastChecked:     "",
		repairQueue:     repairQueue,
		nodestate:       NewReliabilityCache(overlay, config.ReliabilityCacheStaleness),
		irrdb:           irrdb,
		log:             log,
		Loop:            *sync2.NewCycle(config.Interval),
		IrreparableLoop: *sync2.NewCycle(config.IrreparableInterval),
	}
}

// Run the checker loop
func (checker *Checker) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return checker.Loop.Run(ctx, func(ctx context.Context) error {
			checkerObserver := checker.NewObserver()

			err := checker.metainfoloop.Join(ctx, checkerObserver)
			if err != nil {
				return Error.Wrap(err)
			}

			mon.IntVal("remote_files_checked").Observe(checkerObserver.monStats.remoteFilesChecked)
			mon.IntVal("remote_segments_checked").Observe(checkerObserver.monStats.remoteSegmentsChecked)
			mon.IntVal("remote_segments_needing_repair").Observe(checkerObserver.monStats.remoteSegmentsNeedingRepair)
			mon.IntVal("remote_segments_lost").Observe(checkerObserver.monStats.remoteSegmentsLost)
			mon.IntVal("remote_files_lost").Observe(int64(len(checkerObserver.monStats.remoteSegmentInfo)))
			return nil
		})
	})

	group.Go(func() error {
		return checker.IrreparableLoop.Run(ctx, checker.IrreparableProcess)
	})

	return group.Wait()
}

// RefreshReliabilityCache forces refreshing node online status cache.
func (checker *Checker) RefreshReliabilityCache(ctx context.Context) error {
	return checker.nodestate.Refresh(ctx)
}

// Close halts the Checker loop
func (checker *Checker) Close() error {
	checker.Loop.Close()
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

// TODO this is now only used for irreperable db. Figure out if it can be reduced or reconcile with duplicate code in observer.RemoteSegment
func (checker *Checker) updateSegmentStatus(ctx context.Context, pointer *pb.Pointer, path string) (err error) {
	defer mon.Task()(&ctx)(&err)
	remote := pointer.GetRemote()
	if remote == nil {
		return nil
	}

	pieces := remote.GetRemotePieces()
	if pieces == nil {
		checker.log.Debug("no pieces on remote segment")
		return nil
	}

	missingPieces, err := checker.nodestate.MissingPieces(ctx, pointer.CreationDate, pieces)
	if err != nil {
		return Error.New("error getting missing pieces %s", err)
	}

	numHealthy := int32(len(pieces) - len(missingPieces))
	redundancy := pointer.Remote.Redundancy

	// we repair when the number of healthy pieces is less than or equal to the repair threshold
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
	if numHealthy > redundancy.MinReq && numHealthy <= redundancy.RepairThreshold && numHealthy < redundancy.SuccessThreshold {
		if len(missingPieces) == 0 {
			checker.log.Warn("Missing pieces is zero in checker, but this should be impossible -- bad redundancy scheme.")
			return nil
		}
		err = checker.repairQueue.Insert(ctx, &pb.InjuredSegment{
			Path:         []byte(path),
			LostPieces:   missingPieces,
			InsertedTime: time.Now().UTC(),
		})
		if err != nil {
			return Error.New("error adding injured segment to queue %s", err)
		}

		// delete always returns nil when something was deleted and also when element didn't exists
		err = checker.irrdb.Delete(ctx, []byte(path))
		if err != nil {
			checker.log.Error("error deleting entry from irreparable db: ", zap.Error(err))
		}
		// we need one additional piece for error correction. If only the minimum is remaining the file can't be repaired and is lost.
		// except for the case when minimum and repair thresholds are the same (a case usually seen during testing)
	} else if numHealthy <= redundancy.MinReq && numHealthy < redundancy.RepairThreshold {
		// make an entry in to the irreparable table
		segmentInfo := &pb.IrreparableSegment{
			Path:               []byte(path),
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

// IrreparableProcess picks items from irreparabledb and add them to the repair
// worker queue if they, now, can be repaired.
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

		err = checker.updateSegmentStatus(ctx, seg[0].GetSegmentDetail(), string(seg[0].GetPath()))
		if err != nil {
			checker.log.Error("irrepair segment checker failed: ", zap.Error(err))
		}
		offset++
	}

	return nil
}
