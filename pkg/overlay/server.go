// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/dht"

	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
	"storj.io/storj/storage"
)

const (
	maxNodes = 40
)

// Server implements our overlay RPC service
type Server struct {
	dht     dht.DHT
	cache   *Cache
	logger  *zap.Logger
	metrics *monkit.Registry
}

// Lookup finds the address of a node in our overlay network
func (o *Server) Lookup(ctx context.Context, req *proto.LookupRequest) (*proto.LookupResponse, error) {
	na, err := o.cache.Get(ctx, req.NodeID)

	if err != nil {
		o.logger.Error("Error looking up node", zap.Error(err), zap.String("nodeID", req.NodeID))
		return nil, err
	}

	return &proto.LookupResponse{
		Node: &proto.Node{
			Id:      req.GetNodeID(),
			Address: na,
		},
	}, nil
}

// FindStorageNodes searches the overlay network for nodes that meet the provided requirements
func (o *Server) FindStorageNodes(ctx context.Context, req *proto.FindStorageNodesRequest) (*proto.FindStorageNodesResponse, error) {
	// NB:  call FilterNodeReputation from node_reputation package to find nodes for storage
	keys, err := o.cache.DB.List(nil, storage.Limit(10))
	if err != nil {
		o.logger.Error("Error listing nodes", zap.Error(err))
		return nil, err
	}

	if len(keys) > maxNodes {
		// TODO(coyle): determine if this is a set value or they user of the api will specify
		keys = keys[:maxNodes]
	}

	nodes := o.getNodes(ctx, keys)

	return &proto.FindStorageNodesResponse{
		Nodes: nodes,
	}, nil
}

func (o *Server) getNodes(ctx context.Context, keys storage.Keys) []*proto.Node {
	wg := &sync.WaitGroup{}
	ch := make(chan *proto.Node, len(keys))

	wg.Add(len(keys))
	for _, v := range keys {
		go func(ch chan *proto.Node, id string) {

			defer wg.Done()
			na, err := o.cache.Get(ctx, id)
			if err != nil {
				o.logger.Error("failed to get key from cache", zap.Error(err))
				return
			}

			ch <- &proto.Node{Id: id, Address: na}
		}(ch, v.String())
	}

	wg.Wait()
	close(ch)
	nodes := []*proto.Node{}
	for node := range ch {
		nodes = append(nodes, node)
	}

	return nodes
}
