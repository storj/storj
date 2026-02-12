// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeaudit

import (
	"context"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

var monColdLegacy = monkit.Package()

// NodeStats holds statistics about invalid pieces for a single node.
type NodeStats struct {
	InvalidPieceCount int64
	InvalidPieceBytes int64
}

// ColdLegacyStatConfig holds the configuration for ColdLegacyStat observer.
type ColdLegacyStatConfig struct {
	// NodeFilter is the filter expression for nodes to track.
	NodeFilter string `help:"filter expression for nodes to track"`
}

// ColdLegacyStat implements rangedloop.Observer.
// It generates statistics about pieces which are at the wrong placement.
type ColdLegacyStat struct {
	log        *zap.Logger
	config     ColdLegacyStatConfig
	overlay    *overlay.Service
	placements nodeselection.PlacementDefinitions

	// state that gets reset on each Start
	mu        sync.Mutex
	startTime time.Time
	// nodeStats maps nodeID -> statistics about invalid pieces
	nodeStats map[storj.NodeID]*NodeStats
	// nodeCache caches nodes from overlay for lookup
	nodeCache map[storj.NodeID]nodeselection.SelectedNode
	// validPlacements maps nodeID -> set of placements where the node is valid (both filter and uploadFilter match)
	validPlacements map[storj.NodeID]map[storj.PlacementConstraint]bool
	// parsedFilter is the parsed NodeFilter
	parsedFilter nodeselection.NodeFilter
}

// NewColdLegacyStat creates a new ColdLegacyStat observer.
func NewColdLegacyStat(log *zap.Logger, overlay *overlay.Service, placements nodeselection.PlacementDefinitions, config ColdLegacyStatConfig) *ColdLegacyStat {
	return &ColdLegacyStat{
		log:        log,
		config:     config,
		overlay:    overlay,
		placements: placements,
	}
}

// Start is called at the beginning of each segment loop.
func (o *ColdLegacyStat) Start(ctx context.Context, startTime time.Time) (err error) {
	defer monColdLegacy.Task()(&ctx)(&err)

	o.startTime = startTime
	o.nodeStats = make(map[storj.NodeID]*NodeStats)
	o.nodeCache = make(map[storj.NodeID]nodeselection.SelectedNode)
	o.validPlacements = make(map[storj.NodeID]map[storj.PlacementConstraint]bool)

	// Parse the node filter if configured
	if o.config.NodeFilter != "" {
		filter, err := nodeselection.FilterFromString(o.config.NodeFilter, nodeselection.NewPlacementConfigEnvironment(nil, nil))
		if err != nil {
			// Try parsing as a simple placement-style filter
			o.log.Warn("could not parse node filter as attribute filter, will match all nodes",
				zap.String("filter", o.config.NodeFilter),
				zap.Error(err))
			o.parsedFilter = nodeselection.AnyFilter{}
		} else {
			o.parsedFilter = filter
		}
	} else {
		o.parsedFilter = nodeselection.AnyFilter{}
	}

	// Load all participating nodes
	nodes, err := o.overlay.GetAllParticipatingNodes(ctx)
	if err != nil {
		return err
	}

	// Build node cache and determine which nodes match the filter
	for _, node := range nodes {
		o.nodeCache[node.ID] = node

		// Check if node matches the configured filter
		if o.parsedFilter.Match(&node) {
			// Determine which placements this node is valid for
			validForPlacements := make(map[storj.PlacementConstraint]bool)
			for placementID, placement := range o.placements {
				// Check both NodeFilter and UploadFilter (MatchForUpload checks both)
				if placement.MatchForUpload(&node) {
					validForPlacements[placementID] = true
				}
			}
			o.validPlacements[node.ID] = validForPlacements
		}
	}

	return nil
}

// Fork creates a new partial for processing a range.
func (o *ColdLegacyStat) Fork(ctx context.Context) (rangedloop.Partial, error) {
	return &coldLegacyStatFork{
		observer:  o,
		nodeStats: make(map[storj.NodeID]*NodeStats),
	}, nil
}

// Join merges partial results.
func (o *ColdLegacyStat) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer monColdLegacy.Task()(&ctx)(&err)

	fork, ok := partial.(*coldLegacyStatFork)
	if !ok {
		return nil
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	for nodeID, stats := range fork.nodeStats {
		if existing, ok := o.nodeStats[nodeID]; ok {
			existing.InvalidPieceCount += stats.InvalidPieceCount
			existing.InvalidPieceBytes += stats.InvalidPieceBytes
		} else {
			o.nodeStats[nodeID] = &NodeStats{
				InvalidPieceCount: stats.InvalidPieceCount,
				InvalidPieceBytes: stats.InvalidPieceBytes,
			}
		}
	}

	return nil
}

// GetValidPlacement checks if a node is valid for the given placement.
// Returns true if the node is not tracked (not matching NodeFilter): we skip nodes which are not selected by filter.
// Returns false if the node is tracked but not valid for the placement.
func (o *ColdLegacyStat) GetValidPlacement(nodeID storj.NodeID, placement storj.PlacementConstraint) bool {
	validPlacements, found := o.validPlacements[nodeID]
	if !found || validPlacements == nil {
		// when placement is valid, we skip it (not oop), here we skip nodes which are not tracked at all (not matching NodeFilter) in a similar way.
		return true
	}
	return validPlacements[placement]
}

// Finish is called after all segments are processed.
func (o *ColdLegacyStat) Finish(ctx context.Context) (err error) {
	defer monColdLegacy.Task()(&ctx)(&err)

	// Log and emit metrics for each node with invalid pieces
	var totalInvalidPieces int64
	var totalInvalidBytes int64
	var nodesWithInvalidPieces int

	for nodeID, stats := range o.nodeStats {
		if stats.InvalidPieceCount > 0 {
			nodesWithInvalidPieces++
			totalInvalidPieces += stats.InvalidPieceCount
			totalInvalidBytes += stats.InvalidPieceBytes

			o.log.Info("node with invalid placement pieces",
				zap.Stringer("node_id", nodeID),
				zap.Int64("invalid_piece_count", stats.InvalidPieceCount),
				zap.Int64("invalid_piece_bytes", stats.InvalidPieceBytes))
		}
	}

	// Emit aggregate metrics
	monColdLegacy.IntVal("cold_legacy_stat_total_invalid_pieces").Observe(totalInvalidPieces)
	monColdLegacy.IntVal("cold_legacy_stat_total_invalid_bytes").Observe(totalInvalidBytes)
	monColdLegacy.IntVal("cold_legacy_stat_nodes_with_invalid_pieces").Observe(int64(nodesWithInvalidPieces))

	o.log.Info("ColdLegacyStat finished",
		zap.Int("nodes_with_invalid_pieces", nodesWithInvalidPieces),
		zap.Int64("total_invalid_pieces", totalInvalidPieces),
		zap.Int64("total_invalid_bytes", totalInvalidBytes))

	return nil
}

// coldLegacyStatFork implements rangedloop.Partial.
type coldLegacyStatFork struct {
	observer  *ColdLegacyStat
	nodeStats map[storj.NodeID]*NodeStats
}

// Process handles a batch of segments.
func (f *coldLegacyStatFork) Process(ctx context.Context, segments []rangedloop.Segment) error {
	for _, segment := range segments {
		if segment.Inline() || segment.ExpiresAt != nil {
			continue
		}

		segmentPlacement := segment.Placement
		pieceSize := segment.PieceSize()

		for _, piece := range segment.Pieces {
			nodeID := piece.StorageNode

			// If the node is valid for this segment's placement, skip it
			if f.observer.GetValidPlacement(nodeID, segmentPlacement) {
				continue
			}

			stats, ok := f.nodeStats[nodeID]
			if !ok {
				stats = &NodeStats{}
				f.nodeStats[nodeID] = stats
			}
			stats.InvalidPieceCount++
			stats.InvalidPieceBytes += pieceSize
		}
	}

	return nil
}
