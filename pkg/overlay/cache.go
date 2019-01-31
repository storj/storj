// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

const (
	// OverlayBucket is the string representing the bucket used for a bolt-backed overlay dht cache
	OverlayBucket = "overlay"
)

// ErrEmptyNode is returned when the nodeID is empty
var ErrEmptyNode = errs.New("empty node ID")

// ErrNodeNotFound is returned if a node does not exist in database
var ErrNodeNotFound = errs.New("Node not found")

// ErrBucketNotFound is returned if a bucket is unable to be found in the routing table
var ErrBucketNotFound = errs.New("Bucket not found")

// OverlayError creates class of errors for stack traces
var OverlayError = errs.Class("Overlay Error")

// DB implements the database for overlay.Cache
type DB interface {
	// SelectNodes looks up nodes based on criteria
	SelectNodes(ctx context.Context, criteria *NodeCriteria) ([]*pb.Node, error)
	// SelectNewNodes looks up nodes based on new node criteria
	SelectNewNodes(ctx context.Context, count int, criteria *NewNodeCriteria) (*sql.Rows, error)

	// Get looks up the node by nodeID
	Get(ctx context.Context, nodeID storj.NodeID) (*pb.Node, error)
	// GetAll looks up nodes based on the ids from the overlay cache
	GetAll(ctx context.Context, nodeIDs storj.NodeIDList) ([]*pb.Node, error)
	// List lists nodes starting from cursor
	List(ctx context.Context, cursor storj.NodeID, limit int) ([]*pb.Node, error)
	// Paginate will page through the database nodes
	Paginate(ctx context.Context, offset int64, limit int) ([]*pb.Node, bool, error)
	// Update updates node information
	Update(ctx context.Context, value *pb.Node) error
	// Delete deletes node based on id
	Delete(ctx context.Context, id storj.NodeID) error
	// GetWalletAddress gets the node's wallet address
	GetWalletAddress(ctx context.Context, id storj.NodeID) (string, error)
}

// Cache is used to store overlay data in Redis
type Cache struct {
	db     DB
	statDB statdb.DB
}

// NewCache returns a new Cache
func NewCache(db DB, sdb statdb.DB) *Cache {
	return &Cache{db: db, statDB: sdb}
}

// Close closes resources
func (cache *Cache) Close() error { return nil }

// Inspect lists limited number of items in the cache
func (cache *Cache) Inspect(ctx context.Context) (storage.Keys, error) {
	// TODO: implement inspection tools
	return nil, errors.New("not implemented")
}

// List returns a list of nodes from the cache DB
func (cache *Cache) List(ctx context.Context, cursor storj.NodeID, limit int) ([]*pb.Node, error) {
	return cache.db.List(ctx, cursor, limit)
}

// Paginate returns a list of `limit` nodes starting from `start` offset.
func (cache *Cache) Paginate(ctx context.Context, offset int64, limit int) ([]*pb.Node, bool, error) {
	return cache.db.Paginate(ctx, offset, limit)
}

// Get looks up the provided nodeID from the overlay cache
func (cache *Cache) Get(ctx context.Context, nodeID storj.NodeID) (*pb.Node, error) {
	if nodeID.IsZero() {
		return nil, ErrEmptyNode
	}

	return cache.db.Get(ctx, nodeID)
}

type getNodesRequest struct {
	minReputation         *pb.NodeStats
	freeBandwidth         int64
	freeDisk              int64
	excluded              []pb.NodeID
	reputableNodeAmount   int64
	newNodeAmount         int64
	newNodeAuditThreshold int64
}

// FindStorageNodes searches the overlay network for nodes that meet the provided criteria
func (cache *Cache) FindStorageNodes(ctx context.Context, req *pb.FindStorageNodesRequest, preferences *NodeSelectionConfig) ([]*pb.Node, error) {
	minStats := &pb.NodeStats{
		AuditCount:        server.nodeSelectionConfig.AuditCount,
		AuditSuccessRatio: server.nodeSelectionConfig.AuditSuccessRatio,
		UptimeCount:       server.nodeSelectionConfig.UptimeCount,
		UptimeRatio:       server.nodeSelectionConfig.UptimeRatio,
	}

	filterNodesReq := &FilterNodesRequest{
		MinReputation:         minStats,
		MinNodes:              req.GetMinNodes(),
		Opts:                  req.GetOpts(),
		NewNodePercentage:     server.nodeSelectionConfig.NewNodePercentage,
		NewNodeAuditThreshold: server.nodeSelectionConfig.NewNodeAuditThreshold,
	}

	foundNodes, err := server.cache.db.FilterNodes(ctx, filterNodesReq)

	reputableNodeAmount := req.MinNodes
	if reputableNodeAmount <= 0 {
		reputableNodeAmount = req.Opts.GetAmount()
	}

	getReputableReq := &getNodesRequest{
		minReputation:         req.MinReputation,
		freeBandwidth:         req.Opts.GetRestrictions().FreeBandwidth,
		freeDisk:              req.Opts.GetRestrictions().FreeDisk,
		excluded:              req.Opts.ExcludedNodes,
		reputableNodeAmount:   reputableNodeAmount,
		newNodeAuditThreshold: req.NewNodeAuditThreshold,
	}

	reputableNodes, err := cache.getReputableNodes(ctx, getReputableReq)
	if err != nil {
		return nil, err
	}

	newNodeAmount := int64(float64(reputableNodeAmount) * req.NewNodePercentage)

	getNewReq := &getNodesRequest{
		freeBandwidth:         req.Opts.GetRestrictions().FreeBandwidth,
		freeDisk:              req.Opts.GetRestrictions().FreeDisk,
		excluded:              req.Opts.ExcludedNodes,
		newNodeAmount:         newNodeAmount,
		newNodeAuditThreshold: req.NewNodeAuditThreshold,
	}

	newNodes, err := cache.getNewNodes(ctx, getNewReq)
	if err != nil {
		return nil, err
	}

	var allNodes []*pb.Node
	allNodes = append(allNodes, reputableNodes...)
	allNodes = append(allNodes, newNodes...)

	if int64(len(reputableNodes)) < reputableNodeAmount {
		err := status.Errorf(codes.ResourceExhausted, fmt.Sprintf("requested %d reputable nodes, only %d reputable nodes matched the criteria requested",
			reputableNodeAmount, len(reputableNodes)))
		return allNodes, err
	}

	return allNodes, nil
}

func (cache *overlaycache) findReputableNodesQuery(ctx context.Context, req *getNodesRequest) (*sql.Rows, error) {
	auditCount := req.minReputation.AuditCount
	if req.newNodeAuditThreshold > auditCount {
		auditCount = req.newNodeAuditThreshold
	}
	auditSuccessRatio := req.minReputation.AuditSuccessRatio
	uptimeCount := req.minReputation.UptimeCount
	uptimeRatio := req.minReputation.UptimeRatio
	nodeAmt := req.reputableNodeAmount
	// ...
}

// GetAll looks up the provided ids from the overlay cache
func (cache *Cache) GetAll(ctx context.Context, ids storj.NodeIDList) ([]*pb.Node, error) {
	if len(ids) == 0 {
		return nil, OverlayError.New("no ids provided")
	}

	return cache.db.GetAll(ctx, ids)
}

// Put adds a nodeID to the redis cache with a binary representation of proto defined Node
func (cache *Cache) Put(ctx context.Context, nodeID storj.NodeID, value pb.Node) error {
	// If we get a Node without an ID (i.e. bootstrap node)
	// we don't want to add to the routing tbale
	if nodeID.IsZero() {
		return nil
	}
	if nodeID != value.Id {
		return errors.New("invalid request")
	}

	// get existing node rep, or create a new statdb node with 0 rep
	stats, err := cache.statDB.CreateEntryIfNotExists(ctx, nodeID)
	if err != nil {
		return err
	}

	value.Reputation = &pb.NodeStats{
		AuditSuccessRatio:  stats.AuditSuccessRatio,
		AuditSuccessCount:  stats.AuditSuccessCount,
		AuditCount:         stats.AuditCount,
		UptimeRatio:        stats.UptimeRatio,
		UptimeSuccessCount: stats.UptimeSuccessCount,
		UptimeCount:        stats.UptimeCount,
	}

	return cache.db.Update(ctx, &value)
}

// Delete will remove the node from the cache. Used when a node hard disconnects or fails
// to pass a PING multiple times.
func (cache *Cache) Delete(ctx context.Context, id storj.NodeID) error {
	if id.IsZero() {
		return ErrEmptyNode
	}
	return cache.db.Delete(ctx, id)
}

// ConnFailure implements the Transport Observer `ConnFailure` function
func (cache *Cache) ConnFailure(ctx context.Context, node *pb.Node, failureError error) {
	// TODO: Kademlia paper specifies 5 unsuccessful PINGs before removing the node
	// from our routing table, but this is the cache so maybe we want to treat
	// it differently.
	_, err := cache.statDB.UpdateUptime(ctx, node.Id, false)
	if err != nil {
		zap.L().Debug("error updating uptime for node in statDB", zap.Error(err))
	}
}

// ConnSuccess implements the Transport Observer `ConnSuccess` function
func (cache *Cache) ConnSuccess(ctx context.Context, node *pb.Node) {
	err := cache.Put(ctx, node.Id, *node)
	if err != nil {
		zap.L().Debug("error updating uptime for node in statDB", zap.Error(err))
	}
	_, err = cache.statDB.UpdateUptime(ctx, node.Id, true)
	if err != nil {
		zap.L().Debug("error updating statdDB with node connection info", zap.Error(err))
	}
}
