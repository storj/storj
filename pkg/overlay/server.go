// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"bytes"
	"context"
	"fmt"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// ServerError creates class of errors for stack traces
var ServerError = errs.Class("Server Error")

// Server implements our overlay RPC service
type Server struct {
	log       *zap.Logger
	cache     *Cache
	metrics   *monkit.Registry
	nodeStats *pb.NodeStats
}

// NewServer creates a new Overlay Server
func NewServer(log *zap.Logger, cache *Cache, nodeStats *pb.NodeStats) *Server {
	return &Server{
		cache:     cache,
		log:       log,
		metrics:   monkit.Default,
		nodeStats: nodeStats,
	}
}

// Lookup finds the address of a node in our overlay network
func (server *Server) Lookup(ctx context.Context, req *pb.LookupRequest) (*pb.LookupResponse, error) {
	na, err := server.cache.Get(ctx, req.NodeId)

	if err != nil {
		server.log.Error("Error looking up node", zap.Error(err), zap.String("nodeID", req.NodeId.String()))
		return nil, err
	}

	return &pb.LookupResponse{
		Node: na,
	}, nil
}

// BulkLookup finds the addresses of nodes in our overlay network
func (server *Server) BulkLookup(ctx context.Context, reqs *pb.LookupRequests) (*pb.LookupResponses, error) {
	ns, err := server.cache.GetAll(ctx, lookupRequestsToNodeIDs(reqs))
	if err != nil {
		return nil, ServerError.New("could not get nodes requested %s\n", err)
	}
	return nodesToLookupResponses(ns), nil
}

// FindStorageNodes searches the overlay network for nodes that meet the provided requirements
func (server *Server) FindStorageNodes(ctx context.Context, req *pb.FindStorageNodesRequest) (resp *pb.FindStorageNodesResponse, err error) {
	opts := req.GetOpts()
	maxNodes := req.GetMaxNodes()
	if maxNodes <= 0 {
		maxNodes = opts.GetAmount()
	}

	excluded := opts.ExcludedNodes
	restrictions := opts.GetRestrictions()
	reputation := server.nodeStats

	var startID storj.NodeID
	result := []*pb.Node{}
	for {
		var nodes []*pb.Node
		nodes, startID, err = server.populate(ctx, req.Start, maxNodes, restrictions, reputation, excluded)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		resultNodes := []*pb.Node{}
		usedAddrs := make(map[string]bool)
		for _, n := range nodes {
			addr := n.Address.GetAddress()
			excluded = append(excluded, n.Id) // exclude all nodes on next iteration
			if !usedAddrs[addr] {
				resultNodes = append(resultNodes, n)
				usedAddrs[addr] = true
			}
		}
		if len(resultNodes) <= 0 {
			break
		}

		result = append(result, resultNodes...)

		if len(result) >= int(maxNodes) || startID == (storj.NodeID{}) {
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

func (server *Server) getNodes(ctx context.Context, ids storj.NodeIDList) ([]*pb.Node, error) {
	values, err := server.cache.db.GetAll(ctx, ids)
	return values, Error.Wrap(err)
}

// TODO: nicer method arguments
func (server *Server) populate(ctx context.Context,
	startID storj.NodeID, maxNodes int64,
	minRestrictions *pb.NodeRestrictions,
	minReputation *pb.NodeStats,
	excluded storj.NodeIDList) ([]*pb.Node, storj.NodeID, error) {
	// TODO: move the query into db
	limit := int(maxNodes * 2)
	nodes, err := server.cache.db.List(ctx, startID, limit)
	if err != nil {
		server.log.Error("Error listing nodes", zap.Error(err))
		return nil, storj.NodeID{}, Error.Wrap(err)
	}

	var nextStart storj.NodeID
	result := []*pb.Node{}
	for _, v := range nodes {
		if v == nil {
			continue
		}

		nextStart = v.Id
		if v.Type != pb.NodeType_STORAGE {
			continue
		}

		restrictions := v.GetRestrictions()
		reputation := v.GetReputation()

		if restrictions.GetFreeBandwidth() < minRestrictions.GetFreeBandwidth() ||
			restrictions.GetFreeDisk() < minRestrictions.GetFreeDisk() ||
			reputation.GetUptimeRatio() < minReputation.GetUptimeRatio() ||
			reputation.GetUptimeCount() < minReputation.GetUptimeCount() ||
			reputation.GetAuditSuccessRatio() < minReputation.GetAuditSuccessRatio() ||
			reputation.GetAuditCount() < minReputation.GetAuditCount() ||
			contains(excluded, v.Id) {
			continue
		}
		result = append(result, v)
	}

	return result, nextStart, nil
}

// contains checks if item exists in list
func contains(nodeIDs storj.NodeIDList, searchID storj.NodeID) bool {
	for _, id := range nodeIDs {
		if bytes.Equal(id.Bytes(), searchID.Bytes()) {
			return true
		}
	}
	return false
}

// lookupRequestsToNodeIDs returns the nodeIDs from the LookupRequests
func lookupRequestsToNodeIDs(reqs *pb.LookupRequests) (ids storj.NodeIDList) {
	for _, v := range reqs.LookupRequest {
		ids = append(ids, v.NodeId)
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
	return &pb.LookupResponses{LookupResponse: rs}
}
