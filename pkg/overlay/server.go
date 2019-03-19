// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Service implements selecting nodes based on specified config.
type Service struct {
	log         *zap.Logger
	cache       *Cache
	preferences *NodeSelectionConfig
}

// NewService creates a new Overlay Service
func NewService(log *zap.Logger, cache *Cache, preferences *NodeSelectionConfig) *Service {
	return &Service{
		log:         log,
		cache:       cache,
		preferences: preferences,
	}
}

// Close closes resources
func (service *Service) Close() error { return nil }

// OfflineNodes returns indices of the nodes that are offline
func (service *Service) OfflineNodes(ctx context.Context, nodes []storj.NodeID) (offline []int, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: optimize
	results, err := service.cache.GetAll(ctx, nodes)
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
func (service *Service) FindStorageNodes(ctx context.Context, req FindStorageNodeRequest) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	return service.cache.FindStorageNodes(ctx, req, service.preferences)
}

// FindStorageNodeRequest defines easy request parameters.
type FindStorageNodeRequest struct {
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
