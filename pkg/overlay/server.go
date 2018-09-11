// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"
	
	protob "github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/dht"

	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
	"storj.io/storj/storage"
)

// ServerError creates class of errors for stack traces
var ServerError = errs.Class("Server Error")

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
		Node: na,
	}, nil
}

//BulkLookup finds the addresses of nodes in our overlay network
func (o *Server) BulkLookup(ctx context.Context, reqs *proto.LookupRequests) (*proto.LookupResponses, error) {
	ns, err := o.cache.GetAll(ctx, lookupRequestsToNodeIDs(reqs))

	if err != nil {
		return nil, ServerError.New("could not get nodes requested %s\n", err)
	}
	return nodesToLookupResponses(ns), nil
}

// FindStorageNodes searches the overlay network for nodes that meet the provided requirements
func (o *Server) FindStorageNodes(ctx context.Context, req *proto.FindStorageNodesRequest) (resp *proto.FindStorageNodesResponse, err error) {
	opts := req.GetOpts()
	maxNodes := opts.GetAmount()
	restrictions := opts.GetRestrictions()
	restrictedBandwidth := restrictions.GetFreeBandwidth()
	restrictedSpace := restrictions.GetFreeDisk()

	var start storage.Key
	result := []*proto.Node{}
	for {
		var nodes []*proto.Node
		nodes, start, err = o.populate(ctx, start, maxNodes, restrictedBandwidth, restrictedSpace)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		if len(nodes) <= 0 {
			break
		}

		result = append(result, nodes...)

		if len(result) >= int(maxNodes) || start == nil {
			break
		}

	}

	if len(result) < int(maxNodes) {
		return nil, status.Errorf(codes.ResourceExhausted, fmt.Sprintf("requested %d nodes, only %d nodes matched the criteria requested", maxNodes, len(result)))
	}

	if len(result) > int(maxNodes) {
		result = result[:maxNodes]
	}

	return &proto.FindStorageNodesResponse{
		Nodes: result,
	}, nil
}

func (o *Server) getNodes(ctx context.Context, keys storage.Keys) ([]*proto.Node, error) {
	values, err := o.cache.DB.GetAll(keys)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	nodes := []*proto.Node{}
	for _, v := range values {
		n := &proto.Node{}
		if err := protob.Unmarshal(v, n); err != nil {
			return nil, Error.Wrap(err)
		}

		nodes = append(nodes, n)
	}

	return nodes, nil

}

func (o *Server) populate(ctx context.Context, starting storage.Key, maxNodes, restrictedBandwidth, restrictedSpace int64) ([]*proto.Node, storage.Key, error) {
	limit := int(maxNodes * 2)
	keys, err := o.cache.DB.List(starting, limit)
	if err != nil {
		o.logger.Error("Error listing nodes", zap.Error(err))
		return nil, nil, Error.Wrap(err)
	}

	if len(keys) <= 0 {
		o.logger.Info("No Keys returned from List operation")
		return []*proto.Node{}, starting, nil
	}

	result := []*proto.Node{}
	nodes, err := o.getNodes(ctx, keys)
	if err != nil {
		o.logger.Error("Error getting nodes", zap.Error(err))
		return nil, nil, Error.Wrap(err)
	}

	for _, v := range nodes {
		rest := v.GetRestrictions()
		if rest.GetFreeBandwidth() < restrictedBandwidth || rest.GetFreeDisk() < restrictedSpace {
			continue
		}

		result = append(result, v)
	}

	nextStart := keys[len(keys)-1]
	if len(keys) < limit {
		nextStart = nil
	}

	return result, nextStart, nil
}

//lookupRequestsToNodeIDs returns the nodeIDs from the LookupRequests
func lookupRequestsToNodeIDs(reqs *proto.LookupRequests) []string {
	var ids []string
	for _, v := range reqs.Lookuprequest {
		ids = append(ids, v.NodeID)
	}
	return ids
}

//nodesToLookupResponses returns LookupResponses from the nodes
func nodesToLookupResponses(nodes []*proto.Node) *proto.LookupResponses {
	var rs []*proto.LookupResponse
	for _, v := range nodes {
		r := &proto.LookupResponse{Node: v}
		rs = append(rs, r)
	}
	return &proto.LookupResponses{Lookupresponse: rs}
}
