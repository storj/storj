// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"bytes"
	"context"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// ServerError creates class of errors for stack traces
var ServerError = errs.Class("Server Error")

// Server implements our overlay RPC service
type Server struct {
	log                   *zap.Logger
	cache                 *Cache
	metrics               *monkit.Registry
	minStats              *pb.NodeStats
	matchingNodeRatio     float64
	newNodeAuditThreshold int64
	newNodePercentage     float64
}

// NewServer creates a new Overlay Server
func NewServer(log *zap.Logger, cache *Cache, minStats *pb.NodeStats, matchingNodeRatio float64, newNodeAuditThreshold int64,
	newNodePercentage float64) *Server {
	return &Server{
		cache:                 cache,
		log:                   log,
		metrics:               monkit.Default,
		minStats:              minStats,
		matchingNodeRatio:     matchingNodeRatio,
		newNodeAuditThreshold: newNodeAuditThreshold,
		newNodePercentage:     newNodePercentage,
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
	requiredReputableNodes := req.GetMaxNodes()
	if requiredReputableNodes <= 0 {
		requiredReputableNodes = opts.GetAmount()
	}

	excluded := opts.ExcludedNodes
	restrictions := opts.GetRestrictions()
	usedAddrs := make(map[string]bool)

	newStartID := req.Start
	requiredNewNodes := int64(float64(requiredReputableNodes) * server.newNodePercentage)
	var allNewNodes []*pb.Node

	reputableStartID := req.Start
	var allReputableNodes []*pb.Node

	for {
		reputableNodes, reputableStartID, err := server.getReputableNodes(ctx, reputableStartID, requiredReputableNodes, restrictions, excluded)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		reputableNodes, excluded, usedAddrs = server.filterNodes(ctx, reputableNodes, requiredReputableNodes, usedAddrs, excluded)

		allReputableNodes = append(allReputableNodes, reputableNodes...)

		// this means we've exhausted the overlay cache looking for reputable nodes or we have enough
		if reputableStartID == (storj.NodeID{}) || int64(len(allReputableNodes)) >= requiredReputableNodes {
			break
		}
	}

	for {
		newNodes, newStartID, err := server.getNewNodes(ctx, newStartID, requiredNewNodes, restrictions, excluded)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		newNodes, excluded, usedAddrs = server.filterNodes(ctx, newNodes, requiredNewNodes, usedAddrs, excluded)

		allNewNodes = append(allNewNodes, newNodes...)

		// this means we've exhausted the overlay cache looking for new nodes or we have enough
		if newStartID == (storj.NodeID{}) || int64(len(allNewNodes)) >= requiredNewNodes {
			break
		}
	}

	var result []*pb.Node

	if int64(len(allReputableNodes)) >= requiredReputableNodes {
		result = append(result, allReputableNodes[:requiredReputableNodes]...)
	} else {
		result = append(result, allReputableNodes...)
	}

	if int64(len(allNewNodes)) >= requiredNewNodes {
		result = append(result, allNewNodes[:requiredNewNodes]...)
	} else {
		result = append(result, allNewNodes...)
	}

	if int64(len(result)) < requiredReputableNodes {
		err := status.Errorf(codes.ResourceExhausted, fmt.Sprintf("requested %d nodes, only %d nodes matched the criteria requested", requiredReputableNodes, len(result)))
		return &pb.FindStorageNodesResponse{Nodes: result}, err
	}

	return &pb.FindStorageNodesResponse{
		Nodes: result,
	}, nil
}

// filterNodes removes nodes with duplicate IP addresses and updates excluded list with all nodes
func (server *Server) filterNodes(ctx context.Context, nodes []*pb.Node, nodeRequirement int64, usedAddrs map[string]bool,
	excluded storj.NodeIDList) (resultNodes []*pb.Node, updatedExcluded storj.NodeIDList, updatedUsed map[string]bool) {

	for _, n := range nodes {
		addr := n.Address.GetAddress()
		excluded = append(excluded, n.Id) // exclude all nodes on next iteration
		updatedExcluded = excluded
		if !usedAddrs[addr] {
			resultNodes = append(resultNodes, n)
			usedAddrs[addr] = true
		}
	}
	return resultNodes, updatedExcluded, usedAddrs
}

func (server *Server) getNodes(ctx context.Context, keys storage.Keys) ([]*pb.Node, error) {
	values, err := server.cache.db.GetAll(keys)
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

func (server *Server) getReputableNodes(ctx context.Context, startID storj.NodeID, maxNodes int64,
	minRestrictions *pb.NodeRestrictions, excluded storj.NodeIDList) ([]*pb.Node, storj.NodeID, error) {

	limit := int(float64(maxNodes) * server.matchingNodeRatio)
	minReputation := server.minStats

	keys, err := server.cache.db.List(startID.Bytes(), limit)
	if err != nil {
		server.log.Error("Error listing nodes", zap.Error(err))
		return nil, storj.NodeID{}, Error.Wrap(err)
	}

	if len(keys) <= 0 {
		server.log.Info("No Keys returned from List operation")
		return []*pb.Node{}, startID, nil
	}

	nodes, err := server.getNodes(ctx, keys)
	if err != nil {
		server.log.Error("Error getting nodes", zap.Error(err))
		return nil, storj.NodeID{}, Error.Wrap(err)
	}

	for _, v := range nodes {
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
		nodes = append(nodes, v)
	}

	var nextStart storj.NodeID
	if len(keys) < limit {
		nextStart = storj.NodeID{}
	} else {
		nextStart, err = storj.NodeIDFromBytes(keys[len(keys)-1])
	}
	if err != nil {
		return nil, storj.NodeID{}, Error.Wrap(err)
	}
	return nodes, nextStart, nil
}

func (server *Server) getNewNodes(ctx context.Context, startID storj.NodeID, maxNodes int64,
	minRestrictions *pb.NodeRestrictions, excluded storj.NodeIDList) ([]*pb.Node, storj.NodeID, error) {

	limit := int(float64(maxNodes) * server.matchingNodeRatio)

	keys, err := server.cache.db.List(startID.Bytes(), limit)
	if err != nil {
		server.log.Error("Error listing nodes", zap.Error(err))
		return nil, storj.NodeID{}, Error.Wrap(err)
	}

	if len(keys) <= 0 {
		server.log.Info("No Keys returned from List operation")
		return []*pb.Node{}, startID, nil
	}

	nodes, err := server.getNodes(ctx, keys)
	if err != nil {
		server.log.Error("Error getting nodes", zap.Error(err))
		return nil, storj.NodeID{}, Error.Wrap(err)
	}

	for _, v := range nodes {
		if v.Type != pb.NodeType_STORAGE {
			continue
		}

		nodeRestrictions := v.GetRestrictions()
		nodeReputation := v.GetReputation()

		if nodeRestrictions.GetFreeBandwidth() < minRestrictions.GetFreeBandwidth() ||
			nodeRestrictions.GetFreeDisk() < minRestrictions.GetFreeDisk() ||
			contains(excluded, v.Id) {
			continue

		} else if nodeReputation.GetAuditCount() < server.newNodeAuditThreshold {
			nodes = append(nodes, v)
		}
	}

	var nextStart storj.NodeID
	if len(keys) < limit {
		nextStart = storj.NodeID{}
	} else {
		nextStart, err = storj.NodeIDFromBytes(keys[len(keys)-1])
	}
	if err != nil {
		return nil, storj.NodeID{}, Error.Wrap(err)
	}

	return nodes, nextStart, nil
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
