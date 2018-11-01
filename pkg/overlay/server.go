// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
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
func (o *Server) Lookup(ctx context.Context, req *pb.LookupRequest) (*pb.LookupResponse, error) {
	na, err := o.cache.Get(ctx, req.NodeID)

	if err != nil {
		o.logger.Error("Error looking up node", zap.Error(err), zap.String("nodeID", req.NodeID))
		return nil, err
	}

	return &pb.LookupResponse{
		Node: na,
	}, nil
}

//BulkLookup finds the addresses of nodes in our overlay network
func (o *Server) BulkLookup(ctx context.Context, reqs *pb.LookupRequests) (*pb.LookupResponses, error) {
	ns, err := o.cache.GetAll(ctx, lookupRequestsToNodeIDs(reqs))

	if err != nil {
		return nil, ServerError.New("could not get nodes requested %s\n", err)
	}
	return nodesToLookupResponses(ns), nil
}

// FindStorageNodes searches the overlay network for nodes that meet the provided requirements
func (o *Server) FindStorageNodes(ctx context.Context, req *pb.FindStorageNodesRequest) (resp *pb.FindStorageNodesResponse, err error) {
	opts := req.GetOpts()
	maxNodes := req.GetMaxNodes()
	if maxNodes <= 0 {
		maxNodes = opts.GetAmount()
	}

	excluded := opts.GetExcludedNodes()
	restrictions := opts.GetRestrictions()
	restrictedBandwidth := restrictions.GetFreeBandwidth()
	restrictedSpace := restrictions.GetFreeDisk()

	var start storage.Key
	result := []*pb.Node{}
	for {
		var nodes []*pb.Node
		nodes, start, err = o.populate(ctx, req.GetStart(), maxNodes, restrictedBandwidth, restrictedSpace, excluded)
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

	return &pb.FindStorageNodesResponse{
		Nodes: result,
	}, nil
}

func (o *Server) getNodes(ctx context.Context, keys storage.Keys) ([]*pb.Node, error) {
	values, err := o.cache.DB.GetAll(keys)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	nodes := []*pb.Node{}
	for _, v := range values {
		n := &pb.Node{}
		if err := proto.Unmarshal(v, n); err != nil {
			return nil, Error.Wrap(err)
		}

		nodes = append(nodes, n)
	}

	return nodes, nil

}

func (o *Server) populate(ctx context.Context, starting storage.Key, maxNodes, restrictedBandwidth, restrictedSpace int64, excluded []string) ([]*pb.Node, storage.Key, error) {
	limit := int(maxNodes * 2)
	keys, err := o.cache.DB.List(starting, limit)
	if err != nil {
		o.logger.Error("Error listing nodes", zap.Error(err))
		return nil, nil, Error.Wrap(err)
	}

	if len(keys) <= 0 {
		o.logger.Info("No Keys returned from List operation")
		return []*pb.Node{}, starting, nil
	}

	result := []*pb.Node{}
	nodes, err := o.getNodes(ctx, keys)
	if err != nil {
		o.logger.Error("Error getting nodes", zap.Error(err))
		return nil, nil, Error.Wrap(err)
	}

	for _, v := range nodes {
		rest := v.GetRestrictions()

		if rest.GetFreeBandwidth() < restrictedBandwidth ||
			rest.GetFreeDisk() < restrictedSpace ||
			contains(excluded, v.Id) {
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

// contains checks if item exists in list
func contains(list []string, item string) bool {
	for _, listItem := range list {
		if listItem == item {
			return true
		}
	}
	return false
}

//lookupRequestsToNodeIDs returns the nodeIDs from the LookupRequests
func lookupRequestsToNodeIDs(reqs *pb.LookupRequests) []string {
	var ids []string
	for _, v := range reqs.Lookuprequest {
		ids = append(ids, v.NodeID)
	}
	return ids
}

// nodesToLookupResponses returns LookupResponses from the nodes
func nodesToLookupResponses(nodes []*pb.Node) *pb.LookupResponses {
	var rs []*pb.LookupResponse
	for _, v := range nodes {
		r := &pb.LookupResponse{Node: v}
		rs = append(rs, r)
	}
	return &pb.LookupResponses{Lookupresponse: rs}
}
