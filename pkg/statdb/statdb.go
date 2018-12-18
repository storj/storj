// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"

	"storj.io/storj/pkg/storj"
)

// DB interface for database operations
type DB interface {
	// Create a db entry for the provided storagenode
	Create(ctx context.Context, nodeID storj.NodeID, startingStats *NodeStats) (stats *NodeStats, err error)

	// Get a storagenode's stats from the db
	Get(ctx context.Context, nodeID storj.NodeID) (stats *NodeStats, err error)

	// FindInvalidNodes finds a subset of storagenodes that have stats below provided reputation requirements
	FindInvalidNodes(ctx context.Context, nodeIDs storj.NodeIDList, maxStats *NodeStats) (invalidIDs storj.NodeIDList, err error)

	// Update all parts of single storagenode's stats in the db
	Update(ctx context.Context, updateReq *UpdateRequest) (stats *NodeStats, err error)

	// UpdateUptime updates a single storagenode's uptime stats in the db
	UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool) (stats *NodeStats, err error)

	// UpdateAuditSuccess updates a single storagenode's audit stats in the db
	UpdateAuditSuccess(ctx context.Context, nodeID storj.NodeID, auditSuccess bool) (stats *NodeStats, err error)

	// UpdateBatch for updating multiple farmers' stats in the db
	UpdateBatch(ctx context.Context, updateReqList []*UpdateRequest) (statsList []*NodeStats, failedUpdateReqs []*UpdateRequest, err error)

	// CreateEntryIfNotExists creates a statdb node entry and saves to statdb if it didn't already exist
	CreateEntryIfNotExists(ctx context.Context, nodeID storj.NodeID) (stats *NodeStats, err error)
}

// UpdateRequest is a statdb update request message
type UpdateRequest struct {
	NodeID       storj.NodeID
	AuditSuccess bool
	IsUp         bool
}

// NodeStats is a statdb node stats message
type NodeStats struct {
	NodeID             storj.NodeID
	AuditSuccessRatio  float64
	AuditSuccessCount  int64
	AuditCount         int64
	UptimeRatio        float64
	UptimeSuccessCount int64
	UptimeCount        int64
}
