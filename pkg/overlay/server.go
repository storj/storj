// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"bytes"
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// ServerError creates class of errors for stack traces
var ServerError = errs.Class("Server Error")

// Server implements our overlay RPC service
type Server struct {
	log                 *zap.Logger
	cache               *Cache
	metrics             *monkit.Registry
	nodeSelectionConfig *NodeSelectionConfig
}

// NewServer creates a new Overlay Server
func NewServer(log *zap.Logger, cache *Cache, nodeSelectionConfig *NodeSelectionConfig) *Server {
	return &Server{
		cache:               cache,
		log:                 log,
		metrics:             monkit.Default,
		nodeSelectionConfig: nodeSelectionConfig,
	}
}

// Close closes resources
func (server *Server) Close() error { return nil }

// Lookup finds the address of a node in our overlay network
func (server *Server) Lookup(ctx context.Context, req *pb.LookupRequest) (_ *pb.LookupResponse, err error) {
	defer mon.Task()(&ctx)(&err)

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
func (server *Server) BulkLookup(ctx context.Context, reqs *pb.LookupRequests) (_ *pb.LookupResponses, err error) {
	defer mon.Task()(&ctx)(&err)

	ns, err := server.cache.GetAll(ctx, lookupRequestsToNodeIDs(reqs))
	if err != nil {
		return nil, ServerError.New("could not get nodes requested %s\n", err)
	}
	return nodesToLookupResponses(ns), nil
}

// FilterNodesRequest are the requirements for nodes from the overlay cache
type FilterNodesRequest struct {
	MinReputation         *pb.NodeStats
	Restrictions          *pb.NodeRestrictions
	Excluded              []pb.NodeID
	ReputableNodeAmount   int64
	NewNodePercentage     float64
	NewNodeAuditThreshold int64
}

// FindStorageNodes searches the overlay network for nodes that meet the provided requirements
func (server *Server) FindStorageNodes(ctx context.Context, req *pb.FindStorageNodesRequest) (resp *pb.FindStorageNodesResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	minStats := &pb.NodeStats{
		AuditCount:        server.nodeSelectionConfig.AuditCount,
		AuditSuccessRatio: server.nodeSelectionConfig.AuditSuccessRatio,
		UptimeCount:       server.nodeSelectionConfig.UptimeCount,
		UptimeRatio:       server.nodeSelectionConfig.UptimeRatio,
	}

	filterNodesReq := &FilterNodesRequest{
		MinReputation:         minStats,
		Restrictions:          req.GetOpts().GetRestrictions(),
		Excluded:              req.GetOpts().ExcludedNodes,
		ReputableNodeAmount:   req.GetMinNodes(),
		NewNodePercentage:     server.nodeSelectionConfig.NewNodePercentage,
		NewNodeAuditThreshold: server.nodeSelectionConfig.NewNodeAuditThreshold,
	}

	foundNodes, err := server.cache.db.FilterNodes(ctx, filterNodesReq)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &pb.FindStorageNodesResponse{
		Nodes: foundNodes,
	}, nil
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
