// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"math"

	"storj.io/storj/satellite/repair"
)

// Health defines an interface for calculating segment health.
type Health interface {
	// Calculate returns a value corresponding to the health of a segment.
	// Lower health values indicate segments that should be repaired first.
	Calculate(ctx context.Context, numHealthy, minPieces, numForcingRepair int) float64
}

// ProbabilityHealth implements Health using the original SegmentHealth logic.
type ProbabilityHealth struct {
	failureRate float64
	nodeCache   *ReliabilityCache
}

// NewProbabilityHealth creates a new ProbabilityHealth instance.
func NewProbabilityHealth(failureRate float64, nodeCache *ReliabilityCache) *ProbabilityHealth {
	return &ProbabilityHealth{
		failureRate: failureRate,
		nodeCache:   nodeCache,
	}
}

// Calculate returns a value corresponding to the health of a segment.
// It uses the original repair.SegmentHealth logic with node count estimation.
func (h *ProbabilityHealth) Calculate(ctx context.Context, numHealthy, minPieces, numForcingRepair int) float64 {
	totalNumNodes, err := h.nodeCache.NumNodes(ctx)
	if err != nil {
		// fallback to a reasonable default if we can't get node count
		totalNumNodes = 10000
	}
	if totalNumNodes == 0 {
		totalNumNodes = 1
	}

	return repair.SegmentHealth(numHealthy, minPieces, totalNumNodes, h.failureRate, numForcingRepair)
}

// NormalizedHealth implements Health using a normalized health calculation (healthy -k).
type NormalizedHealth struct {
}

// NewNormalizedHealth creates a new NormalizedHealth instance.
func NewNormalizedHealth() *NormalizedHealth {
	return &NormalizedHealth{}
}

// Calculate returns a value corresponding to the health of a segment.
func (n *NormalizedHealth) Calculate(ctx context.Context, numHealthy, minPieces, numForcingRepair int) float64 {
	base := float64(numHealthy-minPieces+1) / float64(minPieces)
	if numForcingRepair > 0 {
		// pop pieces are put between 0.2 and 0.4 importance
		popSignificance := math.Min(float64(numForcingRepair)/float64(minPieces), 1)
		return math.Min(base, 0.4-0.2*popSignificance)
	}
	return base
}
