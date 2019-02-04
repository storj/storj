// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
)

var (
	// Error is the default errs class
	Error = errs.Class("statdb error")
)

// DB stores node statistics
type DB interface {
	// Create adds a new stats entry for node.
	Create(ctx context.Context, nodeID storj.NodeID, initial *NodeStats) (stats *NodeStats, err error)
	// Get returns node stats.
	Get(ctx context.Context, nodeID storj.NodeID) (stats *NodeStats, err error)
	// FindInvalidNodes finds a subset of storagenodes that have stats below provided reputation requirements.
	FindInvalidNodes(ctx context.Context, nodeIDs storj.NodeIDList, maxStats *NodeStats) (invalid storj.NodeIDList, err error)
	// Update all parts of single storagenode's stats.
	Update(ctx context.Context, request *UpdateRequest) (stats *NodeStats, err error)
	// UpdateUptime updates a single storagenode's uptime stats.
	UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool) (stats *NodeStats, err error)
	// UpdateAuditSuccess updates a single storagenode's audit stats.
	UpdateAuditSuccess(ctx context.Context, nodeID storj.NodeID, auditSuccess bool) (stats *NodeStats, err error)
	// UpdateBatch for updating multiple storage nodes' stats.
	UpdateBatch(ctx context.Context, requests []*UpdateRequest) (statslist []*NodeStats, failed []*UpdateRequest, err error)
	// CreateEntryIfNotExists creates a node stats entry if it didn't already exist.
	CreateEntryIfNotExists(ctx context.Context, nodeID storj.NodeID) (stats *NodeStats, err error)
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
}
