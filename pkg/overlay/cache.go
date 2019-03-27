// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"errors"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
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
var ErrNodeNotFound = errs.Class("node not found")

// ErrBucketNotFound is returned if a bucket is unable to be found in the routing table
var ErrBucketNotFound = errs.New("bucket not found")

// ErrNotEnoughNodes is when selecting nodes failed with the given parameters
var ErrNotEnoughNodes = errs.Class("not enough nodes")

// OverlayError creates class of errors for stack traces
var OverlayError = errs.Class("overlay error")

// DB implements the database for overlay.Cache
type DB interface {
	// SelectStorageNodes looks up nodes based on criteria
	SelectStorageNodes(ctx context.Context, count int, criteria *NodeCriteria) ([]*pb.Node, error)
	// SelectNewStorageNodes looks up nodes based on new node criteria
	SelectNewStorageNodes(ctx context.Context, count int, criteria *NewNodeCriteria) ([]*pb.Node, error)

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

	// CreateStats initializes the stats for node.
	CreateStats(ctx context.Context, nodeID storj.NodeID, initial *NodeStats) (stats *NodeStats, err error)
	// GetStats returns node stats.
	GetStats(ctx context.Context, nodeID storj.NodeID) (stats *NodeStats, err error)
	// FindInvalidNodes finds a subset of storagenodes that have stats below provided reputation requirements.
	FindInvalidNodes(ctx context.Context, nodeIDs storj.NodeIDList, maxStats *NodeStats) (invalid storj.NodeIDList, err error)
	// UpdateStats all parts of single storagenode's stats.
	UpdateStats(ctx context.Context, request *UpdateRequest) (stats *NodeStats, err error)
	// UpdateOperator updates the email and wallet for a given node ID for satellite payments.
	UpdateOperator(ctx context.Context, node storj.NodeID, updatedOperator pb.NodeOperator) (stats *NodeStats, err error)
	// UpdateUptime updates a single storagenode's uptime stats.
	UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool) (stats *NodeStats, err error)
	// UpdateBatch for updating multiple storage nodes' stats.
	UpdateBatch(ctx context.Context, requests []*UpdateRequest) (statslist []*NodeStats, failed []*UpdateRequest, err error)
	// CreateEntryIfNotExists creates a node stats entry if it didn't already exist.
	CreateEntryIfNotExists(ctx context.Context, value *pb.Node) (stats *NodeStats, err error)
}

// FindStorageNodesRequest defines easy request parameters.
type FindStorageNodesRequest struct {
	MinimumRequiredNodes int
	RequestedCount       int

	FreeBandwidth int64
	FreeDisk      int64

	ExcludedNodes []storj.NodeID
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

// UpdateRequest is used to update a node status.
type UpdateRequest struct {
	NodeID       storj.NodeID
	AuditSuccess bool
	IsUp         bool
}

// NodeStats contains statistics about a node.
type NodeStats struct {
	NodeID             storj.NodeID
	AuditSuccessRatio  float64
	AuditSuccessCount  int64
	AuditCount         int64
	UptimeRatio        float64
	UptimeSuccessCount int64
	UptimeCount        int64
	Operator           pb.NodeOperator
}

// Cache is used to store and handle node information
type Cache struct {
	log         *zap.Logger
	db          DB
	preferences NodeSelectionConfig
}

// NewCache returns a new Cache
func NewCache(log *zap.Logger, db DB, preferences NodeSelectionConfig) *Cache {
	return &Cache{
		log:         log,
		db:          db,
		preferences: preferences,
	}
}

// Close closes resources
func (cache *Cache) Close() error { return nil }

// Inspect lists limited number of items in the cache
func (cache *Cache) Inspect(ctx context.Context) (storage.Keys, error) {
	// TODO: implement inspection tools
	return nil, errors.New("not implemented")
}

// List returns a list of nodes from the cache DB
func (cache *Cache) List(ctx context.Context, cursor storj.NodeID, limit int) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	return cache.db.List(ctx, cursor, limit)
}

// Paginate returns a list of `limit` nodes starting from `start` offset.
func (cache *Cache) Paginate(ctx context.Context, offset int64, limit int) (_ []*pb.Node, _ bool, err error) {
	defer mon.Task()(&ctx)(&err)
	return cache.db.Paginate(ctx, offset, limit)
}

// Get looks up the provided nodeID from the overlay cache
func (cache *Cache) Get(ctx context.Context, nodeID storj.NodeID) (_ *pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	if nodeID.IsZero() {
		return nil, ErrEmptyNode
	}
	return cache.db.Get(ctx, nodeID)
}

// OfflineNodes returns indices of the nodes that are offline
func (cache *Cache) OfflineNodes(ctx context.Context, nodes []storj.NodeID) (offline []int, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: optimize
	results, err := cache.GetAll(ctx, nodes)
	if err != nil {
		return nil, err
	}

	for i, r := range results {
		if r == nil {
			offline = append(offline, i)
		}
	}

	return offline, nil
}

// FindStorageNodes searches the overlay network for nodes that meet the provided requirements
func (cache *Cache) FindStorageNodes(ctx context.Context, req FindStorageNodesRequest) ([]*pb.Node, error) {
	return cache.FindStorageNodesWithPreferences(ctx, req, &cache.preferences)
}

// FindStorageNodesWithPreferences searches the overlay network for nodes that meet the provided criteria
func (cache *Cache) FindStorageNodesWithPreferences(ctx context.Context, req FindStorageNodesRequest, preferences *NodeSelectionConfig) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: verify logic

	// TODO: add sanity limits to requested node count
	// TODO: add sanity limits to excluded nodes

	reputableNodeCount := req.MinimumRequiredNodes
	if reputableNodeCount <= 0 {
		reputableNodeCount = req.RequestedCount
	}

	auditCount := preferences.AuditCount
	if auditCount < preferences.NewNodeAuditThreshold {
		auditCount = preferences.NewNodeAuditThreshold
	}

	reputableNodes, err := cache.db.SelectStorageNodes(ctx, reputableNodeCount, &NodeCriteria{
		FreeBandwidth: req.FreeBandwidth,
		FreeDisk:      req.FreeDisk,

		AuditCount:         auditCount,
		AuditSuccessRatio:  preferences.AuditSuccessRatio,
		UptimeCount:        preferences.UptimeCount,
		UptimeSuccessRatio: preferences.UptimeRatio,

		Excluded: req.ExcludedNodes,
	})
	if err != nil {
		return nil, err
	}

	newNodeCount := int64(float64(reputableNodeCount) * preferences.NewNodePercentage)
	newNodes, err := cache.db.SelectNewStorageNodes(ctx, int(newNodeCount), &NewNodeCriteria{
		FreeBandwidth: req.FreeBandwidth,
		FreeDisk:      req.FreeDisk,

		AuditThreshold: preferences.NewNodeAuditThreshold,

		Excluded: req.ExcludedNodes,
	})
	if err != nil {
		return nil, err
	}

	nodes := []*pb.Node{}
	nodes = append(nodes, newNodes...)
	nodes = append(nodes, reputableNodes...)

	if len(reputableNodes) < reputableNodeCount {
		return nodes, ErrNotEnoughNodes.New("requested %d found %d", reputableNodeCount, len(reputableNodes))
	}

	return nodes, nil
}

// GetAll looks up the provided ids from the overlay cache
func (cache *Cache) GetAll(ctx context.Context, ids storj.NodeIDList) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(ids) == 0 {
		return nil, OverlayError.New("no ids provided")
	}

	return cache.db.GetAll(ctx, ids)
}

// Put adds a node id and proto definition into the overlay cache and stat db
func (cache *Cache) Put(ctx context.Context, nodeID storj.NodeID, value pb.Node) (err error) {
	defer mon.Task()(&ctx)(&err)

	// If we get a Node without an ID (i.e. bootstrap node)
	// we don't want to add to the routing tbale
	if nodeID.IsZero() {
		return nil
	}
	if nodeID != value.Id {
		return errors.New("invalid request")
	}

	// get existing node rep, or create a new overlay node with 0 rep
	stats, err := cache.db.CreateEntryIfNotExists(ctx, &value)
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
func (cache *Cache) Delete(ctx context.Context, id storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	if id.IsZero() {
		return ErrEmptyNode
	}

	return cache.db.Delete(ctx, id)
}

// Create adds a new stats entry for node.
func (cache *Cache) Create(ctx context.Context, nodeID storj.NodeID, initial *NodeStats) (stats *NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)
	return cache.db.CreateStats(ctx, nodeID, initial)
}

// GetStats returns node stats.
func (cache *Cache) GetStats(ctx context.Context, nodeID storj.NodeID) (stats *NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)
	return cache.db.GetStats(ctx, nodeID)
}

// FindInvalidNodes finds a subset of storagenodes that have stats below provided reputation requirements.
func (cache *Cache) FindInvalidNodes(ctx context.Context, nodeIDs storj.NodeIDList, maxStats *NodeStats) (invalid storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)
	return cache.db.FindInvalidNodes(ctx, nodeIDs, maxStats)
}

// UpdateStats all parts of single storagenode's stats.
func (cache *Cache) UpdateStats(ctx context.Context, request *UpdateRequest) (stats *NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)
	return cache.db.UpdateStats(ctx, request)
}

// UpdateOperator updates the email and wallet for a given node ID for satellite payments.
func (cache *Cache) UpdateOperator(ctx context.Context, node storj.NodeID, updatedOperator pb.NodeOperator) (stats *NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)
	return cache.db.UpdateOperator(ctx, node, updatedOperator)
}

// UpdateUptime updates a single storagenode's uptime stats.
func (cache *Cache) UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool) (stats *NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)
	return cache.db.UpdateUptime(ctx, nodeID, isUp)
}

// ConnFailure implements the Transport Observer `ConnFailure` function
func (cache *Cache) ConnFailure(ctx context.Context, node *pb.Node, failureError error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	// TODO: Kademlia paper specifies 5 unsuccessful PINGs before removing the node
	// from our routing table, but this is the cache so maybe we want to treat
	// it differently.
	_, err = cache.db.UpdateUptime(ctx, node.Id, false)
	if err != nil {
		zap.L().Debug("error updating uptime for node", zap.Error(err))
	}
}

// ConnSuccess implements the Transport Observer `ConnSuccess` function
func (cache *Cache) ConnSuccess(ctx context.Context, node *pb.Node) {
	var err error
	defer mon.Task()(&ctx)(&err)

	err = cache.Put(ctx, node.Id, *node)
	if err != nil {
		zap.L().Debug("error updating uptime for node", zap.Error(err))
	}
	_, err = cache.db.UpdateUptime(ctx, node.Id, true)
	if err != nil {
		zap.L().Debug("error updating node connection info", zap.Error(err))
	}
}
