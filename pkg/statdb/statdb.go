// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// DB stores node statistics
type DB interface {
	// Create adds a new stats entry for node.
	Create(ctx context.Context, nodeID storj.NodeID, initial *NodeStats) (dossier *pb.NodeDossier, err error)
	// Get returns node stats.
	Get(ctx context.Context, nodeID storj.NodeID) (dossier *pb.NodeDossier, err error)
	// FindInvalidNodes finds a subset of storagenodes that have stats below provided reputation requirements.
	FindInvalidNodes(ctx context.Context, nodeIDs storj.NodeIDList, maxStats *NodeStats) (invalid storj.NodeIDList, err error)
	// UpdateOperator updates the email and wallet for a given node ID for satellite payments.
	UpdateOperator(ctx context.Context, node storj.NodeID, updatedOperator pb.NodeOperator) (dossier *pb.NodeDossier, err error)
	// UpdateUptime updates a single storagenode's uptime stats.
	UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool) (dossier *pb.NodeDossier, err error)
	// UpdateAuditSuccess updates a single storagenode's audit stats.
	UpdateAuditSuccess(ctx context.Context, nodeID storj.NodeID, auditSuccess bool) (dossier *pb.NodeDossier, err error)
	// UpdateBatch for updating multiple storage nodes' stats.
	UpdateBatch(ctx context.Context, requests []*UpdateRequest) (statslist []*NodeStats, failed []*UpdateRequest, err error)
	// CreateEntryIfNotExists creates a node stats entry if it didn't already exist.
	CreateEntryIfNotExists(ctx context.Context, nodeID storj.NodeID) (dossier *pb.NodeDossier, err error)
	// SelectStorageNodes looks up nodes based on criteria
	SelectStorageNodes(ctx context.Context, count int, criteria *NodeCriteria) ([]*pb.Node, error)
	// SelectNewStorageNodes looks up nodes based on new node criteria
	SelectNewStorageNodes(ctx context.Context, count int, criteria *NewNodeCriteria) ([]*pb.Node, error)
	// GetAll looks up nodes based on the ids from the overlay cache
	GetAll(ctx context.Context, nodeIDs storj.NodeIDList) ([]*pb.NodeDossier, error)
	// List lists nodes starting from cursor
	List(ctx context.Context, cursor storj.NodeID, limit int) ([]*pb.NodeDossier, error)
	// Paginate will page through the database nodes
	Paginate(ctx context.Context, offset int64, limit int) ([]*pb.NodeDossier, bool, error)
	// Update updates node information
	Update(ctx context.Context, value *pb.NodeDossier) error
	// Delete deletes node based on id
	Delete(ctx context.Context, id storj.NodeID) error
}

// UpdateRequest is used to update a node status.
type UpdateRequest struct {
	NodeID       storj.NodeID
	AuditSuccess bool
	IsUp         bool
}

// NodeStats contains statistics abot a node.
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
