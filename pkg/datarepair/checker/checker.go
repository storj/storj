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
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
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
	statdb      statdb.DB
	pointerdb   *pointerdb.Service
	repairQueue queue.RepairQueue
	overlay     *overlay.Service
	irrdb       irreparable.DB
	logger      *zap.Logger
	Loop        sync2.Cycle
}

// NewChecker creates a new instance of checker
func NewChecker(pointerdb *pointerdb.Service, sdb statdb.DB, repairQueue queue.RepairQueue, overlay *overlay.Service, irrdb irreparable.DB, limit int, logger *zap.Logger, interval time.Duration) *Checker {
	// TODO: reorder arguments
	checker := &Checker{
		statdb:      sdb,
		pointerdb:   pointerdb,
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
		return err
	})
}

// Close halts the Checker loop
func (checker *Checker) Close() error {
	checker.Loop.Close()
	return nil
}

// IdentifyInjuredSegments checks for missing pieces off of the pointerdb and overlay cache
func (checker *Checker) IdentifyInjuredSegments(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var remoteSegmentsChecked int64
	var remoteSegmentsNeedingRepair int64
	var remoteSegmentsLost int64
	var remoteSegmentInfo []string

	err = checker.pointerdb.Iterate("", "", true, false,
		func(it storage.Iterator) error {
			var item storage.ListItem
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

				var nodeIDs storj.NodeIDList
				for _, p := range pieces {
					nodeIDs = append(nodeIDs, p.NodeId)
				}

				// Find all offline nodes
				offlineNodes, err := checker.overlay.OfflineNodes(ctx, nodeIDs)
				if err != nil {
					return Error.New("error getting offline nodes %s", err)
				}

				invalidNodes, err := checker.invalidNodes(ctx, nodeIDs)
				if err != nil {
					return Error.New("error getting invalid nodes %s", err)
				}

				missingPieces := combineOfflineWithInvalid(offlineNodes, invalidNodes)

				remoteSegmentsChecked++
				numHealthy := len(nodeIDs) - len(missingPieces)
				if (int32(numHealthy) >= pointer.Remote.Redundancy.MinReq) && (int32(numHealthy) < pointer.Remote.Redundancy.RepairThreshold) {
					remoteSegmentsNeedingRepair++
					err = checker.repairQueue.Enqueue(ctx, &pb.InjuredSegment{
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

					//add the entry if new or update attempt count if already exists
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

// Find invalidNodes by checking the audit results that are place in statdb
func (checker *Checker) invalidNodes(ctx context.Context, nodeIDs storj.NodeIDList) (invalidNodes []int, err error) {
	// filter if nodeIDs have invalid pieces from auditing results
	maxStats := &statdb.NodeStats{
		AuditSuccessRatio: 0, // TODO: update when we have stats added to statdb
		UptimeRatio:       0, // TODO: update when we have stats added to statdb
	}

	invalidIDs, err := checker.statdb.FindInvalidNodes(ctx, nodeIDs, maxStats)
	if err != nil {
		return nil, Error.New("error getting valid nodes from statdb %s", err)
	}

	invalidNodesMap := make(map[storj.NodeID]bool)
	for _, invalidID := range invalidIDs {
		invalidNodesMap[invalidID] = true
	}

	for i, nID := range nodeIDs {
		if invalidNodesMap[nID] {
			invalidNodes = append(invalidNodes, i)
		}
	}

	return invalidNodes, nil
}

// combine the offline nodes with nodes marked invalid by statdb
func combineOfflineWithInvalid(offlineNodes []int, invalidNodes []int) (missingPieces []int32) {
	for _, offline := range offlineNodes {
		missingPieces = append(missingPieces, int32(offline))
	}

	offlineMap := make(map[int]bool)
	for _, i := range offlineNodes {
		offlineMap[i] = true
	}
	for _, i := range invalidNodes {
		if !offlineMap[i] {
			missingPieces = append(missingPieces, int32(i))
		}
	}

	return missingPieces
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
