// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"

	"storj.io/storj/pkg/datarepair/irreparabledb"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/statdb"
	statpb "storj.io/storj/pkg/statdb/proto"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// Checker is the interface for data repair checker
type Checker interface {
	Run(ctx context.Context) error
}

// Checker contains the information needed to do checks for missing pieces
type checker struct {
	statdb      *statdb.StatDB
	pointerdb   *pointerdb.Server
	repairQueue *queue.Queue
	overlay     pb.OverlayServer
	irrdb       *irreparabledb.Database
	limit       int
	logger      *zap.Logger
	ticker      *time.Ticker
}

// newChecker creates a new instance of checker
func newChecker(pointerdb *pointerdb.Server, sdb *statdb.StatDB, repairQueue *queue.Queue, overlay pb.OverlayServer, irrdb *irreparabledb.Database, limit int, logger *zap.Logger, interval time.Duration) *checker {
	return &checker{
		statdb:      sdb,
		pointerdb:   pointerdb,
		repairQueue: repairQueue,
		overlay:     overlay,
		irrdb:       irrdb,
		limit:       limit,
		logger:      logger,
		ticker:      time.NewTicker(interval),
	}
}

// Run the checker loop
func (c *checker) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		err = c.identifyInjuredSegments(ctx)
		if err != nil {
			zap.L().Error("Checker failed", zap.Error(err))
		}

		select {
		case <-c.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the checker is canceled via context
			return ctx.Err()
		}
	}
}

// identifyInjuredSegments checks for missing pieces off of the pointerdb and overlay cache
func (c *checker) identifyInjuredSegments(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = c.pointerdb.Iterate(ctx, &pb.IterateRequest{Recurse: true},
		func(it storage.Iterator) error {
			var item storage.ListItem
			lim := c.limit
			if lim <= 0 || lim > storage.LookupLimit {
				lim = storage.LookupLimit
			}
			for ; lim > 0 && it.Next(&item); lim-- {
				pointer := &pb.Pointer{}
				err = proto.Unmarshal(item.Value, pointer)
				if err != nil {
					return Error.New("error unmarshalling pointer %s", err)
				}
				remote := pointer.GetRemote()
				if remote == nil {
					c.logger.Debug("no remote segment on pointer")
					continue
				}
				pieces := remote.GetRemotePieces()
				if pieces == nil {
					c.logger.Debug("no pieces on remote segment")
					continue
				}
				var nodeIDs storj.NodeIDList
				for _, p := range pieces {
					nodeIDs = append(nodeIDs, p.NodeId)
				}

				// Find all offline nodes
				offlineNodes, err := c.offlineNodes(ctx, nodeIDs)
				if err != nil {
					return Error.New("error getting offline nodes %s", err)
				}

				invalidNodes, err := c.invalidNodes(ctx, nodeIDs)
				if err != nil {
					return Error.New("error getting invalid nodes %s", err)
				}

				missingPieces := combineOfflineWithInvalid(offlineNodes, invalidNodes)

				numHealthy := len(nodeIDs) - len(missingPieces)
				if (int32(numHealthy) >= pointer.Remote.Redundancy.MinReq) && (int32(numHealthy) < pointer.Remote.Redundancy.RepairThreshold) {
					err = c.repairQueue.Enqueue(&pb.InjuredSegment{
						Path:       string(item.Key),
						LostPieces: missingPieces,
					})
					if err != nil {
						return Error.New("error adding injured segment to queue %s", err)
					}
				} else if int32(numHealthy) < pointer.Remote.Redundancy.MinReq {
					// make an entry in to the irreparable table
					segmentInfo := &irreparabledb.RemoteSegmentInfo{
						EncryptedSegmentPath:   item.Key,
						EncryptedSegmentDetail: item.Value,
						LostPiecesCount:        int64(len(missingPieces)),
						RepairUnixSec:          time.Now().Unix(),
						RepairAttemptCount:     int64(1),
					}

					//add the entry if new or update attempt count if already exists
					err := c.irrdb.IncrementRepairAttempts(ctx, segmentInfo)
					if err != nil {
						return Error.New("error handling irreparable segment to queue %s", err)
					}
				}
			}
			return nil
		},
	)
	return err
}

// returns the indices of offline nodes
func (c *checker) offlineNodes(ctx context.Context, nodeIDs storj.NodeIDList) (offline []int32, err error) {
	responses, err := c.overlay.BulkLookup(ctx, pb.NodeIDsToLookupRequests(nodeIDs))
	if err != nil {
		return []int32{}, err
	}
	nodes := pb.LookupResponsesToNodes(responses)
	for i, n := range nodes {
		if n == nil {
			offline = append(offline, int32(i))
		}
	}
	return offline, nil
}

// Find invalidNodes by checking the audit results that are place in statdb
func (c *checker) invalidNodes(ctx context.Context, nodeIDs storj.NodeIDList) (invalidNodes []int32, err error) {
	// filter if nodeIDs have invalid pieces from auditing results
	findInvalidNodesReq := &statpb.FindInvalidNodesRequest{
		NodeIds: nodeIDs,
		MaxStats: &pb.NodeStats{
			AuditSuccessRatio: 0, // TODO: update when we have stats added to statdb
			UptimeRatio:       0, // TODO: update when we have stats added to statdb
		},
	}

	resp, err := c.statdb.FindInvalidNodes(ctx, findInvalidNodesReq)
	if err != nil {
		return nil, Error.New("error getting valid nodes from statdb %s", err)
	}

	invalidNodesMap := make(map[storj.NodeID]bool)
	for _, invalidID := range resp.InvalidIds {
		invalidNodesMap[invalidID] = true
	}

	for i, nID := range nodeIDs {
		if invalidNodesMap[nID] {
			invalidNodes = append(invalidNodes, int32(i))
		}
	}

	return invalidNodes, nil
}

// combine the offline nodes with nodes marked invalid by statdb
func combineOfflineWithInvalid(offlineNodes []int32, invalidNodes []int32) (missingPieces []int32) {
	missingPieces = append(missingPieces, offlineNodes...)

	offlineMap := make(map[int32]bool)
	for _, i := range offlineNodes {
		offlineMap[i] = true
	}
	for _, i := range invalidNodes {
		if !offlineMap[i] {
			missingPieces = append(missingPieces, i)
		}
	}

	return missingPieces
}
