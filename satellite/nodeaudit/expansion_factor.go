// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeaudit

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair"
	"storj.io/storj/shared/location"
)

var monExpansion = monkit.Package()

// PlacementExpansionStats holds expansion factor statistics for a single placement.
type PlacementExpansionStats struct {
	// SegmentCount is the number of segments in this placement.
	SegmentCount int64
	// TotalSegmentSize is the total logical size of all segments (encrypted size).
	TotalSegmentSize int64
	// TotalPieceSize is the actual storage used (number of pieces * piece size).
	TotalPieceSize int64
	// HealthySize is the storage used by healthy pieces only (healthy piece count * piece size).
	HealthySize int64
}

// ExpansionFactorConfig holds the configuration for ExpansionFactor observer.
type ExpansionFactorConfig struct {
	ExcludedCountryCodes []string `help:"list of country codes to exclude from healthy piece calculation" default:""`
	DoDeclumping         bool     `help:"enable declumping check" default:"true"`
	DoPlacementCheck     bool     `help:"enable placement check" default:"true"`
}

// ExpansionFactor implements rangedloop.Observer.
// It calculates expansion factor statistics per placement by tracking:
// - Total segment size: logical segment size
// - Total piece size: actual storage (all pieces)
// - Healthy size: storage used by healthy pieces only
type ExpansionFactor struct {
	log        *zap.Logger
	config     ExpansionFactorConfig
	overlay    *overlay.Service
	placements nodeselection.PlacementDefinitions

	excludedCountryCodes map[location.CountryCode]struct{}

	// state that gets reset on each Start
	mu             sync.Mutex
	startTime      time.Time
	placementStats map[storj.PlacementConstraint]*PlacementExpansionStats
	// nodeCache is pre-loaded at Start with all participating nodes
	nodeCache map[storj.NodeID]nodeselection.SelectedNode
}

// NewExpansionFactor creates a new ExpansionFactor observer.
func NewExpansionFactor(log *zap.Logger, overlay *overlay.Service, placements nodeselection.PlacementDefinitions, config ExpansionFactorConfig) *ExpansionFactor {
	excludedCountryCodes := make(map[location.CountryCode]struct{})
	for _, countryCode := range config.ExcludedCountryCodes {
		if cc := location.ToCountryCode(countryCode); cc != location.None {
			excludedCountryCodes[cc] = struct{}{}
		}
	}

	return &ExpansionFactor{
		log:                  log,
		config:               config,
		overlay:              overlay,
		placements:           placements,
		excludedCountryCodes: excludedCountryCodes,
	}
}

// Start is called at the beginning of each segment loop.
func (o *ExpansionFactor) Start(ctx context.Context, startTime time.Time) (err error) {
	defer monExpansion.Task()(&ctx)(&err)

	o.mu.Lock()
	defer o.mu.Unlock()

	o.startTime = startTime
	o.placementStats = make(map[storj.PlacementConstraint]*PlacementExpansionStats)

	// Pre-load all participating nodes to avoid querying the database for each batch.
	// Nodes that join after this point will be ignored, which is acceptable for statistics.
	nodes, err := o.overlay.GetAllParticipatingNodes(ctx)
	if err != nil {
		return err
	}

	o.nodeCache = make(map[storj.NodeID]nodeselection.SelectedNode, len(nodes))
	for _, node := range nodes {
		o.nodeCache[node.ID] = node
	}

	o.log.Info("ExpansionFactor loaded node cache",
		zap.Int("node_count", len(o.nodeCache)))

	return nil
}

// Fork creates a new partial for processing a range.
func (o *ExpansionFactor) Fork(ctx context.Context) (rangedloop.Partial, error) {
	return &expansionFactorFork{
		observer:       o,
		placementStats: make(map[storj.PlacementConstraint]*PlacementExpansionStats),
	}, nil
}

// Join merges partial results.
func (o *ExpansionFactor) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer monExpansion.Task()(&ctx)(&err)

	fork, ok := partial.(*expansionFactorFork)
	if !ok {
		return nil
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	for placement, stats := range fork.placementStats {
		existing, ok := o.placementStats[placement]
		if !ok {
			o.placementStats[placement] = &PlacementExpansionStats{
				SegmentCount:     stats.SegmentCount,
				TotalSegmentSize: stats.TotalSegmentSize,
				TotalPieceSize:   stats.TotalPieceSize,
				HealthySize:      stats.HealthySize,
			}
		} else {
			existing.SegmentCount += stats.SegmentCount
			existing.TotalSegmentSize += stats.TotalSegmentSize
			existing.TotalPieceSize += stats.TotalPieceSize
			existing.HealthySize += stats.HealthySize
		}
	}

	return nil
}

// Finish is called after all segments are processed.
func (o *ExpansionFactor) Finish(ctx context.Context) (err error) {
	defer monExpansion.Task()(&ctx)(&err)

	o.log.Info("ExpansionFactor calculation complete",
		zap.Duration("duration", time.Since(o.startTime)))

	for placement, stats := range o.placementStats {
		var realExpansion, healthyExpansion float64
		if stats.TotalSegmentSize > 0 {
			realExpansion = float64(stats.TotalPieceSize) / float64(stats.TotalSegmentSize)
			healthyExpansion = float64(stats.HealthySize) / float64(stats.TotalSegmentSize)
		}

		o.log.Info("Expansion factor for placement",
			zap.Uint16("placement", uint16(placement)),
			zap.Int64("segment_count", stats.SegmentCount),
			zap.Int64("total_segment_size_bytes", stats.TotalSegmentSize),
			zap.Int64("total_piece_size_bytes", stats.TotalPieceSize),
			zap.Int64("healthy_size_bytes", stats.HealthySize),
			zap.Float64("real_expansion_factor", realExpansion),
			zap.Float64("healthy_expansion_factor", healthyExpansion))

		// Emit monkit metrics
		monExpansion.IntVal("expansion_factor_segment_count",
			monkit.NewSeriesTag("placement", placementString(placement))).Observe(stats.SegmentCount)
		monExpansion.IntVal("expansion_factor_total_segment_size_bytes",
			monkit.NewSeriesTag("placement", placementString(placement))).Observe(stats.TotalSegmentSize)
		monExpansion.IntVal("expansion_factor_total_piece_size_bytes",
			monkit.NewSeriesTag("placement", placementString(placement))).Observe(stats.TotalPieceSize)
		monExpansion.IntVal("expansion_factor_healthy_size_bytes",
			monkit.NewSeriesTag("placement", placementString(placement))).Observe(stats.HealthySize)
		monExpansion.FloatVal("expansion_factor_real",
			monkit.NewSeriesTag("placement", placementString(placement))).Observe(realExpansion)
		monExpansion.FloatVal("expansion_factor_healthy",
			monkit.NewSeriesTag("placement", placementString(placement))).Observe(healthyExpansion)
	}

	return nil
}

func placementString(p storj.PlacementConstraint) string {
	return strconv.FormatUint(uint64(p), 10)
}

// expansionFactorFork implements rangedloop.Partial.
type expansionFactorFork struct {
	observer       *ExpansionFactor
	placementStats map[storj.PlacementConstraint]*PlacementExpansionStats
}

// Process handles a batch of segments.
func (f *expansionFactorFork) Process(ctx context.Context, segments []rangedloop.Segment) error {
	// Process each segment
	for _, segment := range segments {
		if segment.Inline() {
			continue
		}

		// Skip expired segments
		if segment.Expired(time.Now()) {
			continue
		}

		if segment.Redundancy.RequiredShares == 0 {
			continue
		}

		placement := segment.Placement
		stats, ok := f.placementStats[placement]
		if !ok {
			stats = &PlacementExpansionStats{}
			f.placementStats[placement] = stats
		}

		stats.SegmentCount++

		stats.TotalSegmentSize += int64(segment.EncryptedSize)

		pieceSize := segment.PieceSize()

		// Total piece size is number of pieces * piece size
		numPieces := len(segment.Pieces)
		stats.TotalPieceSize += int64(numPieces) * pieceSize

		// Get nodes for classification from pre-loaded cache.
		// If a node is not in cache (joined after Start), it will be a zero-value SelectedNode
		// which ClassifySegmentPieces treats as offline/missing.
		nodes := make([]nodeselection.SelectedNode, len(segment.Pieces))
		for i, piece := range segment.Pieces {
			if node, ok := f.observer.nodeCache[piece.StorageNode]; ok {
				nodes[i] = node
			}
		}

		// Classify pieces to determine healthy count
		piecesCheck := repair.ClassifySegmentPieces(
			segment.Pieces,
			nodes,
			f.observer.excludedCountryCodes,
			f.observer.config.DoPlacementCheck,
			f.observer.config.DoDeclumping,
			f.observer.placements[placement],
		)

		// Healthy size is healthy piece count * piece size
		stats.HealthySize += int64(piecesCheck.Healthy.Count()) * pieceSize
	}

	return nil
}
