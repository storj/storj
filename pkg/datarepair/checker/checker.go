// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"

	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// Checker is the interface for data repair checker
type Checker interface {
	Run(ctx context.Context) error
}

// Checker contains the information needed to do checks for missing pieces
type checker struct {
	pointerdb     *pointerdb.Server
	repairQueue   *queue.Queue
	overlay       pb.OverlayServer
	irreparabledb irrdbclient.Client
	limit         int
	logger        *zap.Logger
	ticker        *time.Ticker
}

// NewChecker creates a new instance of checker
func newChecker(ctx context.Context, pointerdb *pointerdb.Server, repairQueue *queue.Queue, overlay pb.OverlayServer, limit int, logger *zap.Logger, interval time.Duration) (*checker, error) {
	ca, err := provider.NewTestCA(ctx)
	if err != nil {
		return nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}

	dummyirrDBPort := "127.0.0.1:9999"
	dummyApiKey := []byte("AdummyAPIKEY")
	irrclient, err := irrdbclient.NewClient(identity, dummyirrDBPort, []byte(dummyApiKey))
	if err != nil {
		return nil, err
	}
	return &checker{
		pointerdb:     pointerdb,
		repairQueue:   repairQueue,
		overlay:       overlay,
		irreparabledb: irrclient,
		limit:         limit,
		logger:        logger,
		ticker:        time.NewTicker(interval),
	}, nil
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
				missingPieces, err := c.offlineNodes(ctx, nodeIDs)
				if err != nil {
					return Error.New("error getting offline nodes %s", err)
				}
				numHealthy := len(nodeIDs) - len(missingPieces)
				if (int32(numHealthy) >= pointer.Remote.Redundancy.MinReq) && (int32(numHealthy) < pointer.Remote.Redundancy.RepairThreshold) {
					err = c.repairQueue.Enqueue(&pb.InjuredSegment{
						Path:       string(item.Key),
						LostPieces: missingPieces,
					})
					if err != nil {
						return Error.New("error adding injured segment to queue %s", err)
					} else if int32(numHealthy) < pointer.Remote.Redundancy.MinReq {
						// make an entry in to the postgres irreparable table
						err = c.irreparabledb.Create(ctx, item.Key, item.Value)
						if err != nil {
							return Error.New("couldn't make an entry into irreparable db: %s", err)
						}

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
