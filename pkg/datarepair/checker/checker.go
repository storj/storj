// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"

	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/storage"
)

// Config contains configurable values for checker
type Config struct {
	// QueueAddress string `help:"data repair queue address" default:"redis://localhost:6379?db=5&password=123"`
	Interval time.Duration `help:"how frequently checker should audit segments" default:"30s"`
}

// Run runs the checker with configured values
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)

	zap.S().Info("Checker is starting up")

	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		for {
			select {
			case <-ticker.C:
				zap.S().Info("Starting segment checker service")
			case <-ctx.Done():
				return
			}
		}
	}()

	return server.Run(ctx)
}

// Checker contains the information needed to do checks for missing pieces
type Checker struct {
	pointerdb   *pointerdb.Server
	repairQueue *queue.Queue
	overlay     pb.OverlayServer
	limit       int
	logger      *zap.Logger
}

// NewChecker creates a new instance of checker
func NewChecker(pointerdb *pointerdb.Server, repairQueue *queue.Queue, overlay pb.OverlayServer, limit int, logger *zap.Logger) *Checker {
	return &Checker{
		pointerdb:   pointerdb,
		repairQueue: repairQueue,
		overlay:     overlay,
		limit:       limit,
		logger:      logger,
	}
}

// IdentifyInjuredSegments checks for missing pieces off of the pointerdb and overlay cache
func (c *Checker) IdentifyInjuredSegments(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	c.logger.Debug("entering pointerdb iterate")

	err = c.pointerdb.Iterate(ctx, &pb.IterateRequest{Recurse: true},
		func(it storage.Iterator) error {
			var item storage.ListItem
			if c.limit <= 0 || c.limit > storage.LookupLimit {
				c.limit = storage.LookupLimit
			}
			for ; c.limit > 0 && it.Next(&item); c.limit-- {
				pointer := &pb.Pointer{}
				err = proto.Unmarshal(item.Value, pointer)
				if err != nil {
					return Error.New("error unmarshalling pointer %s", err)
				}
				pieces := pointer.Remote.RemotePieces
				var nodeIDs []dht.NodeID
				for _, p := range pieces {
					nodeIDs = append(nodeIDs, node.IDFromString(p.NodeId))
				}
				missingPieces, err := c.offlineNodes(ctx, nodeIDs)
				if err != nil {
					return Error.New("error getting missing offline nodes %s", err)
				}
				numHealthy := len(nodeIDs) - len(missingPieces)
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
func (c *Checker) offlineNodes(ctx context.Context, nodeIDs []dht.NodeID) (offline []int32, err error) {
	responses, err := c.overlay.BulkLookup(ctx, nodeIDsToLookupRequests(nodeIDs))
	if err != nil {
		return []int32{}, err
	}
	nodes := lookupResponsesToNodes(responses)
	for i, n := range nodes {
		if n == nil {
			offline = append(offline, int32(i))
		}
	}
	return offline, nil
}

func nodeIDsToLookupRequests(nodeIDs []dht.NodeID) *pb.LookupRequests {
	var rq []*pb.LookupRequest
	for _, v := range nodeIDs {
		r := &pb.LookupRequest{NodeID: v.String()}
		rq = append(rq, r)
	}
	return &pb.LookupRequests{Lookuprequest: rq}
}

func lookupResponsesToNodes(responses *pb.LookupResponses) []*pb.Node {
	var nodes []*pb.Node
	for _, v := range responses.Lookupresponse {
		n := v.Node
		nodes = append(nodes, n)
	}
	return nodes
}
