// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
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
	Interval time.Duration `help:"how frequently checker should audit segments" default:"30s"`
}

// Checker contains the information needed to do checks for missing pieces
type Checker struct {
	metainfo    *metainfo.Service
	lastChecked string
	repairQueue queue.RepairQueue
	overlay     *overlay.Cache
	irrdb       irreparable.DB
	logger      *zap.Logger
	Loop        sync2.Cycle
}

// NewChecker creates a new instance of checker
func NewChecker(metainfo *metainfo.Service, repairQueue queue.RepairQueue, overlay *overlay.Cache, irrdb irreparable.DB, limit int, logger *zap.Logger, interval time.Duration) *Checker {
	// TODO: reorder arguments
	checker := &Checker{
		metainfo:    metainfo,
		lastChecked: "",
		repairQueue: repairQueue,
		overlay:     overlay,
		irrdb:       irrdb,
		logger:      logger,
		Loop:        *sync2.NewCycle(interval),
	}
	return checker
}

// Run the checker loop
func (checker *Checker) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return checker.Loop.Run(ctx, func(ctx context.Context) error {
		err := checker.IdentifyInjuredSegments(ctx)
		if err != nil {
			checker.logger.Error("error with injured segments identification: ", zap.Error(err))
		}
		return nil
	})
}

// Close halts the Checker loop
func (checker *Checker) Close() error {
	checker.Loop.Close()
	return nil
}

// IdentifyInjuredSegments checks for missing pieces off of the metainfo and overlay cache
func (checker *Checker) IdentifyInjuredSegments(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var remoteSegmentsChecked int64
	var remoteSegmentsNeedingRepair int64
	var remoteSegmentsLost int64
	var remoteSegmentInfo []string

	err = checker.metainfo.Iterate("", checker.lastChecked, true, false,
		func(it storage.Iterator) error {
			var item storage.ListItem

			defer func() {
				var nextItem storage.ListItem
				it.Next(&nextItem)
				// start at the next item in the next call
				checker.lastChecked = nextItem.Key.String()
				// if keys are equal, start from the beginning in the next call
				if nextItem.Key.String() == item.Key.String() {
					checker.lastChecked = ""
				}
			}()

			for it.Next(&item) {
				pointer := &pb.Pointer{}

				err = proto.Unmarshal(item.Value, pointer)
				if err != nil {
					return Error.New("error unmarshalling pointer %s", err)
				}

				remote := pointer.GetRemote()
				if remote == nil {
					continue
				}

				pieces := remote.GetRemotePieces()
				if pieces == nil {
					checker.logger.Debug("no pieces on remote segment")
					continue
				}

				missingPieces, err := checker.getMissingPieces(ctx, pieces)
				if err != nil {
					return Error.New("error getting missing pieces %s", err)
				}

				remoteSegmentsChecked++
				numHealthy := len(pieces) - len(missingPieces)
				if (int32(numHealthy) >= pointer.Remote.Redundancy.MinReq) && (int32(numHealthy) <= pointer.Remote.Redundancy.RepairThreshold) {
					remoteSegmentsNeedingRepair++
					err = checker.repairQueue.Insert(ctx, &pb.InjuredSegment{
						Path:       string(item.Key),
						LostPieces: missingPieces,
					})
					if err != nil {
						return Error.New("error adding injured segment to queue %s", err)
					}
				} else if int32(numHealthy) < pointer.Remote.Redundancy.MinReq {
					pathElements := storj.SplitPath(storj.Path(item.Key))
					// check to make sure there are at least *4* path elements. the first three
					// are project, segment, and bucket name, but we want to make sure we're talking
					// about an actual object, and that there's an object name specified
					if len(pathElements) >= 4 {
						project, bucketName, segmentpath := pathElements[0], pathElements[2], pathElements[3]
						lostSegInfo := storj.JoinPaths(project, bucketName, segmentpath)
						if contains(remoteSegmentInfo, lostSegInfo) == false {
							remoteSegmentInfo = append(remoteSegmentInfo, lostSegInfo)
						}
					}

					// TODO: irreparable segment should be using storj.NodeID or something, since at the point of repair
					//       it may have been already repaired once.

					remoteSegmentsLost++
					// make an entry in to the irreparable table
					segmentInfo := &pb.IrreparableSegment{
						Path:               item.Key,
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
			}
			return nil
		},
	)
	if err != nil {
		return err
	}
	mon.IntVal("remote_segments_checked").Observe(remoteSegmentsChecked)
	mon.IntVal("remote_segments_needing_repair").Observe(remoteSegmentsNeedingRepair)
	mon.IntVal("remote_segments_lost").Observe(remoteSegmentsLost)
	mon.IntVal("remote_files_lost").Observe(int64(len(remoteSegmentInfo)))

	return nil
}

func (checker *Checker) getMissingPieces(ctx context.Context, pieces []*pb.RemotePiece) (missingPieces []int32, err error) {
	var nodeIDs storj.NodeIDList
	for _, p := range pieces {
		nodeIDs = append(nodeIDs, p.NodeId)
	}
	badNodeIDs, err := checker.overlay.KnownUnreliableOrOffline(ctx, nodeIDs)
	if err != nil {
		return nil, Error.New("error getting nodes %s", err)
	}

	for _, p := range pieces {
		for _, nodeID := range badNodeIDs {
			if nodeID == p.NodeId {
				missingPieces = append(missingPieces, p.GetPieceNum())
			}
		}
	}
	return missingPieces, nil
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
