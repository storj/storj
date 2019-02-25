// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
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
	log         *zap.Logger
	cache       *Cache
	metrics     *monkit.Registry
	preferences *NodeSelectionConfig
}

// NewServer creates a new Overlay Server
func NewServer(log *zap.Logger, cache *Cache, preferences *NodeSelectionConfig) *Server {
	return &Server{
		cache:       cache,
		log:         log,
		metrics:     monkit.Default,
		preferences: preferences,
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

// NodeCriteria are the requirements for selecting nodes
type NodeCriteria struct {
	FreeBandwidth int64
	FreeDisk      int64

	AuditCount         int64
	AuditSuccessRatio  float64
	UptimeCount        int64
	UptimeSuccessRatio float64

	Excluded []storj.NodeID
}

// NewNodeCriteria are the requirement for selecting new nodes
type NewNodeCriteria struct {
	FreeBandwidth int64
	FreeDisk      int64

	AuditThreshold int64

	Excluded []storj.NodeID
}

// FindStorageNodes searches the overlay network for nodes that meet the provided requirements
func (server *Server) FindStorageNodes(ctx context.Context, req *pb.FindStorageNodesRequest) (resp *pb.FindStorageNodesResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	return server.FindStorageNodesWithPreferences(ctx, req, server.preferences)
}

// FindStorageNodesWithPreferences searches the overlay network for nodes that meet the provided requirements
// exposed mainly for testing
func (server *Server) FindStorageNodesWithPreferences(ctx context.Context, req *pb.FindStorageNodesRequest, preferences *NodeSelectionConfig) (resp *pb.FindStorageNodesResponse, err error) {
	// TODO: use better structs for find storage nodes
	nodes, err := server.cache.FindStorageNodes(ctx, req, preferences)
	return &pb.FindStorageNodesResponse{
		Nodes: nodes,
	}, err
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
