// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package datarepair

import (
	"context"
	"go.uber.org/zap"
	"github.com/golang/protobuf/proto"
	"github.com/zeebo/errs"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/overlay"
)

type checker struct {
	params *pb.IdentifyRequest //move this proto
	pointerdb storage.KeyValueStore
	repairQueue *Queue
	overlay overlay.Overlay
	logger *zap.Logger
}


var (
	mon          = monkit.Package()
	checkerError = errs.Class("data repair checker error")
	
)

func newChecker() *checker{
	return &checker{}
}

func (c *checker) identifyInjuredSegments(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	c.logger.Debug("entering pointerdb iterate")

	err = c.pointerdb.Iterate(storage.IterateOptions{Prefix: storage.Key(c.params.Prefix), First: storage.Key(c.params.First), Recurse: c.params.Recurse, Reverse: c.params.Reverse},
		func(it storage.Iterator) error {
			var item storage.ListItem
			for ; c.params.Limit > 0 && it.Next(&item); c.params.Limit-- {
				pointer := &pb.Pointer{}
				err = proto.Unmarshal(item.Value, pointer)
				if err != nil {
					return checkerError.New("error unmarshalling pointer %s", err)
				}
				pieces := pointer.Remote.RemotePieces
				var nodeIDs []dht.NodeID
				for _, p := range pieces {
					nodeIDs = append(nodeIDs, kademlia.StringToNodeID(p.NodeId))
				}
				missingPieces, err := c.offlineNodes(ctx, nodeIDs)
				if err != nil {
					return checkerError.New("error getting missing offline nodes %s", err)
				}
				if int32(len(missingPieces)) >= pointer.Remote.Redundancy.RepairThreshold {
					err = c.repairQueue.Add(&pb.InjuredSegment{
						Path: string(item.Key), 
						LostPieces: missingPieces,
					})
					if err != nil {
						return checkerError.New("error adding injured segment to queue %s", err)
					}
				}
			}
			return nil
		},
	)
	return err
}

//returns the indices of offline nodes
func (c *checker) offlineNodes(ctx context.Context, nodeIDs []dht.NodeID) (indices []int32, err error) {
	nodes, err := c.overlay.BulkLookup(ctx, nodeIDs)
	if err != nil {
		return []int32{}, err
	}
	for i, n := range nodes {
		if n == nil {
			indices = append(indices, int32(i))
		}
	}
	return indices, nil
}