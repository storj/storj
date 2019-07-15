// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storage"
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

// durabilityStats remote segment information
type durabilityStats struct {
	remoteFilesChecked          int64
	remoteSegmentsChecked       int64
	remoteSegmentsNeedingRepair int64
	remoteSegmentsLost          int64
	remoteSegmentInfo           []string
}

// Checker contains the information needed to do checks for missing pieces
type Checker struct {
	metainfo        *metainfo.Service
	lastChecked     string
	repairQueue     queue.RepairQueue
	nodestate       *ReliabilityCache
	irrdb           irreparable.DB
	logger          *zap.Logger
	Loop            sync2.Cycle
	IrreparableLoop sync2.Cycle
	monStats        durabilityStats
}

// NewChecker creates a new instance of checker
func NewChecker(metainfo *metainfo.Service, repairQueue queue.RepairQueue, overlay *overlay.Cache, irrdb irreparable.DB, limit int, logger *zap.Logger, config Config) *Checker {
	// TODO: reorder arguments
	return &Checker{
		metainfo:        metainfo,
		lastChecked:     "",
		repairQueue:     repairQueue,
		nodestate:       NewReliabilityCache(overlay, config.ReliabilityCacheStaleness),
		irrdb:           irrdb,
		logger:          logger,
		Loop:            *sync2.NewCycle(config.Interval),
		IrreparableLoop: *sync2.NewCycle(config.IrreparableInterval),
		monStats:        durabilityStats{},
	}
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

// RefreshReliabilityCache forces refreshing node online status cache.
func (checker *Checker) RefreshReliabilityCache(ctx context.Context) error {
	return checker.nodestate.Refresh(ctx)
}

// Close halts the Checker loop
func (checker *Checker) Close() error {
	checker.Loop.Close()
	return nil
}

// IdentifyInjuredSegments checks for missing pieces off of the metainfo and overlay cache
func (checker *Checker) IdentifyInjuredSegments(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = checker.metainfo.Iterate(ctx, "", checker.lastChecked, true, false,
		func(ctx context.Context, it storage.Iterator) error {
			var item storage.ListItem

			defer func() {
				var nextItem storage.ListItem
				it.Next(ctx, &nextItem)
				// start at the next item in the next call
				checker.lastChecked = nextItem.Key.String()
				// if we have finished iterating, send and reset durability stats
				if checker.lastChecked == "" {
					// send durability stats
					mon.IntVal("remote_files_checked").Observe(checker.monStats.remoteFilesChecked)
					mon.IntVal("remote_segments_checked").Observe(checker.monStats.remoteSegmentsChecked)
					mon.IntVal("remote_segments_needing_repair").Observe(checker.monStats.remoteSegmentsNeedingRepair)
					mon.IntVal("remote_segments_lost").Observe(checker.monStats.remoteSegmentsLost)
					mon.IntVal("remote_files_lost").Observe(int64(len(checker.monStats.remoteSegmentInfo)))

					// reset durability stats for next iteration
					checker.monStats = durabilityStats{}
				}
			}()

			for it.Next(ctx, &item) {
				pointer := &pb.Pointer{}

				err = proto.Unmarshal(item.Value, pointer)
				if err != nil {
					return Error.New("error unmarshalling pointer %s", err)
				}
				remote := pointer.GetRemote()
				if remote == nil {
					continue
				}

				err = checker.updateSegmentStatus(ctx, pointer, item.Key.String(), &checker.monStats)
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

func (checker *Checker) updateSegmentStatus(ctx context.Context, pointer *pb.Pointer, path string, monStats *durabilityStats) (err error) {
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

	missingPieces, err := checker.nodestate.MissingPieces(ctx, pointer.CreationDate, pieces)
	if err != nil {
		return Error.New("error getting missing pieces %s", err)
	}

	monStats.remoteSegmentsChecked++
	pathElements := storj.SplitPath(path)
	if len(pathElements) >= 2 && pathElements[1] == "l" {
		monStats.remoteFilesChecked++
	}

	numHealthy := int32(len(pieces) - len(missingPieces))
	redundancy := pointer.Remote.Redundancy
	mon.IntVal("checker_segment_total_count").Observe(int64(len(pieces)))
	mon.IntVal("checker_segment_healthy_count").Observe(int64(numHealthy))

	// we repair when the number of healthy pieces is less than or equal to the repair threshold
	// except for the case when the repair and success thresholds are the same (a case usually seen during testing)
	if numHealthy > redundancy.MinReq && numHealthy <= redundancy.RepairThreshold && numHealthy < redundancy.SuccessThreshold {
		if len(missingPieces) == 0 {
			checker.logger.Warn("Missing pieces is zero in checker, but this should be impossible -- bad redundancy scheme.")
			return nil
		}
		monStats.remoteSegmentsNeedingRepair++
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
			checker.logger.Error("error deleting entry from irreparable db: ", zap.Error(err))
		}
		// we need one additional piece for error correction. If only the minimum is remaining the file can't be repaired and is lost.
		// except for the case when minimum and repair thresholds are the same (a case usually seen during testing)
	} else if numHealthy <= redundancy.MinReq && numHealthy < redundancy.RepairThreshold {
		// check to make sure there are at least *4* path elements. the first three
		// are project, segment, and bucket name, but we want to make sure we're talking
		// about an actual object, and that there's an object name specified
		if len(pathElements) >= 4 {
			project, bucketName, segmentpath := pathElements[0], pathElements[2], pathElements[3]
			lostSegInfo := storj.JoinPaths(project, bucketName, segmentpath)
			if contains(monStats.remoteSegmentInfo, lostSegInfo) == false {
				monStats.remoteSegmentInfo = append(monStats.remoteSegmentInfo, lostSegInfo)
			}
		}

		monStats.remoteSegmentsLost++
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

		err = checker.updateSegmentStatus(ctx, seg[0].GetSegmentDetail(), string(seg[0].GetPath()), &durabilityStats{})
		if err != nil {
			checker.logger.Error("irrepair segment checker failed: ", zap.Error(err))
		}
		offset++
	}

	return nil
}
