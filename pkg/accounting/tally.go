// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
)

// Tally is the service for adding up storage node data usage
type Tally interface {
	Run(ctx context.Context) error
}

type tally struct {
	pointerdb *pointerdb.Server
	overlay   pb.OverlayServer
	kademlia  *kademlia.Kademlia
	limit     int
	logger    *zap.Logger
	ticker    *time.Ticker
}

func newTally(pointerdb *pointerdb.Server, overlay pb.OverlayServer, kademlia *kademlia.Kademlia, limit int, logger *zap.Logger, interval time.Duration) *tally {
	return &tally{
		pointerdb: pointerdb,
		overlay:   overlay,
		kademlia:  kademlia,
		limit:     limit,
		logger:    logger,
	}
}

// Run the collector loop
func (t *tally) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		err = t.identifyActiveNodes(ctx)
		if err != nil {
			zap.L().Error("Collector failed", zap.Error(err))
		}

		select {
		case <-t.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the collector is canceled via context
			return ctx.Err()
		}
	}
}

// identifyActiveNodes iterates through pointerdb and identifies nodes that have storage on them
func (t *tally) identifyActiveNodes(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	rt, err := t.kademlia.GetRoutingTable(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	self := rt.Local()
	identity := &provider.FullIdentity{}
	client, err := node.NewNodeClient(identity, self, t.kademlia)
	if err != nil {
		return Error.Wrap(err)
	}

	t.logger.Debug("entering pointerdb iterate")
	err = t.pointerdb.Iterate(ctx, &pb.IterateRequest{Recurse: true},
		func(it storage.Iterator) error {
			var item storage.ListItem
			lim := t.limit
			if lim <= 0 || lim > storage.LookupLimit {
				lim = storage.LookupLimit
			}
			for ; lim > 0 && it.Next(&item); lim-- {
				pointer := &pb.Pointer{}
				err = proto.Unmarshal(item.Value, pointer)
				if err != nil {
					return Error.Wrap(err)
				}
				pieces := pointer.Remote.RemotePieces
				var nodeIDs []dht.NodeID
				for _, p := range pieces {
					nodeIDs = append(nodeIDs, node.IDFromString(p.NodeId))
				}
				online, err := t.onlineNodes(ctx, nodeIDs)
				if err != nil {
					return Error.Wrap(err)
				}
				go t.tallyAtRestStorage(ctx, pointer, online, client)
			}
			return nil
		},
	)
	return err
}

func (t *tally) onlineNodes(ctx context.Context, nodeIDs []dht.NodeID) (online []*pb.Node, err error) {
	responses, err := t.overlay.BulkLookup(ctx, utils.NodeIDsToLookupRequests(nodeIDs))
	if err != nil {
		return []*pb.Node{}, err
	}
	nodes := utils.LookupResponsesToNodes(responses)
	for _, n := range nodes {
		if n != nil {
			online = append(online, n)
		}
	}
	return online, nil
}

func (t *tally) tallyAtRestStorage(ctx context.Context, pointer *pb.Pointer, nodes []*pb.Node, client node.Client) {
	segmentSize := pointer.GetSize()
	minReq := pointer.Remote.Redundancy.GetMinReq()
	if minReq <= 0 {
		zap.L().Error("minReq must be an int greater than 0")
		return
	}
	pieceSize := segmentSize / int64(minReq)
	for _, n := range nodes {
		nodeAvail := true
		var err error
		ok := t.needToContact(n.Id)
		if ok {
			nodeAvail, err = client.Ping(ctx, *n)
			if err != nil {
				zap.L().Error("ping failed")
				continue
			}
		}
		if nodeAvail {
			err := t.updateGranularTable(n.Id, pieceSize)
			if err != nil {
				zap.L().Error("update failed")
			}
		}
	}
}

func (t *tally) needToContact(nodeID string) bool {
	//TODO
	//check db if node was updated within the last time period
	return true
}

func (t *tally) updateGranularTable(nodeID string, pieceSize int64) error {
	//TODO
	return nil
}
