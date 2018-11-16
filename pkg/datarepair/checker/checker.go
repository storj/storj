// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
	"storj.io/storj/pkg/storj"

	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/storage"
)

// Checker is the interface for the data repair queue
type Checker interface {
	IdentifyInjuredSegments(ctx context.Context) (err error)
	Run(ctx context.Context) error
}

// Checker contains the information needed to do checks for missing pieces
type checker struct {
	pointerdb   *pointerdb.Server
	repairQueue *queue.Queue
	overlay     pb.OverlayServer
	limit       int
	logger      *zap.Logger
	ticker      *time.Ticker
}

// NewChecker creates a new instance of checker
func newChecker(pointerdb *pointerdb.Server, repairQueue *queue.Queue, overlay pb.OverlayServer, limit int, logger *zap.Logger, interval time.Duration) *checker {
	return &checker{
		pointerdb:   pointerdb,
		repairQueue: repairQueue,
		overlay:     overlay,
		limit:       limit,
		logger:      logger,
		ticker:      time.NewTicker(interval),
	}
}

// Run the checker loop
func (c *checker) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		err = c.IdentifyInjuredSegments(ctx)
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

// IdentifyInjuredSegments checks for missing pieces off of the pointerdb and overlay cache
func (c *checker) IdentifyInjuredSegments(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	c.logger.Debug("entering pointerdb iterate")

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
				pieces := pointer.Remote.RemotePieces
				var nodeIDsBytes [][]byte
				for _, p := range pieces {
					nodeIDsBytes = append(nodeIDsBytes, p.NodeId)
				}

				nodeIDs, err := storj.NodeIDsFromBytes(nodeIDsBytes)
				if err != nil {
					return Error.Wrap(err)
				}
				missingPieces, err := c.offlineNodes(ctx, nodeIDs)
				if err != nil {
					return Error.New("error getting missing offline nodes %s", err)
				}
				numHealthy := len(nodeIDsBytes) - len(missingPieces)
				if int32(numHealthy) < pointer.Remote.Redundancy.RepairThreshold {
					err = c.repairQueue.Enqueue(&pb.InjuredSegment{
						Path:       string(item.Key),
						LostPieces: missingPieces,
					})
					if err != nil {
						return Error.New("error adding injured segment to queue %s", err)
					}
				}
			}
			return nil
		},
	)
	return err
}

// returns the indices of offline and online nodes
func (c *checker) offlineNodes(ctx context.Context, nodeIDs storj.NodeIDList) (offline []int32, err error) {
	responses, err := c.overlay.BulkLookup(ctx, nodeIDsToLookupRequests(nodeIDs))
	if err != nil {
		return []int32{}, err
	}
	nodes, err := lookupResponsesToNodes(responses)
	if err != nil {
		return []int32{}, err
	}
	for i, n := range nodes {
		if n.Id == nil {
			offline = append(offline, int32(i))
		}
	}
	return offline, nil
}

func nodeIDsToLookupRequests(nodeIDs storj.NodeIDList) *pb.LookupRequests {
	var rq []*pb.LookupRequest
	for _, v := range nodeIDs {
		r := &pb.LookupRequest{NodeId: v.Bytes()}
		rq = append(rq, r)
	}
	return &pb.LookupRequests{Lookuprequest: rq}
}

func lookupResponsesToNodes(responses *pb.LookupResponses) ([]storj.Node, error) {
	var pbNodes []*pb.Node
	for _, v := range responses.Lookupresponse {
		n := v.Node
		pbNodes = append(pbNodes, n)
	}
	return storj.NewNodes(pbNodes)
}
