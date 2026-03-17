// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package balancer

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/taskqueue"
)

// InvariantConfig holds the configuration for the Invariant.
type InvariantConfig struct {
	// StreamID is the Redis stream name used for invariant fixer jobs.
	StreamID string `help:"Redis stream name for invariant fixer jobs" default:"invariant"`
	// Placement limits fixing to segments with this placement ID. -1 means all placements.
	Placement int `help:"only fix segments with this placement ID; -1 means all placements" default:"-1"`
}

// Invariant implements rangedloop.Observer.
// It finds segments with pieces violating placement invariants and generates
// jobs to move those pieces to compliant nodes.
type Invariant struct {
	log        *zap.Logger
	config     InvariantConfig
	overlay    *overlay.Service
	placements nodeselection.PlacementDefinitions
	client     *taskqueue.Client

	// state populated during Start, read-only during Fork/Process
	nodeMap   map[storj.NodeID]*nodeselection.SelectedNode
	selectors map[storj.PlacementConstraint]nodeselection.NodeSelector
}

// NewInvariantObserver creates a new Invariant.
func NewInvariantObserver(
	log *zap.Logger,
	overlay *overlay.Service,
	placements nodeselection.PlacementDefinitions,
	client *taskqueue.Client,
	config InvariantConfig,
) *Invariant {
	return &Invariant{
		log:        log,
		config:     config,
		overlay:    overlay,
		placements: placements,
		client:     client,
	}
}

var _ rangedloop.Observer = (*Invariant)(nil)

// Start is called at the beginning of each segment loop.
func (p *Invariant) Start(ctx context.Context, startTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	allNodes, err := p.overlay.UploadSelectionCache.GetAllNodes(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	p.nodeMap = make(map[storj.NodeID]*nodeselection.SelectedNode, len(allNodes))
	for _, node := range allNodes {
		p.nodeMap[node.ID] = node
	}

	p.selectors = make(map[storj.PlacementConstraint]nodeselection.NodeSelector)
	for id, placement := range p.placements {
		selectorInit := placement.Selector
		if selectorInit == nil {
			selectorInit = nodeselection.RandomSelector()
		}
		nodeFilter := placement.NodeFilter
		if nodeFilter == nil {
			nodeFilter = nodeselection.AnyFilter{}
		}
		p.selectors[id] = selectorInit(ctx, allNodes, nodeFilter)
	}

	p.log.Info("Invariant started",
		zap.Int("total_nodes", len(allNodes)),
		zap.Int("placements", len(p.placements)),
	)

	return nil
}

// Fork creates a new partial for processing a range.
func (p *Invariant) Fork(ctx context.Context) (rangedloop.Partial, error) {
	return &invariantFork{
		observer: p,
	}, nil
}

// Join merges partial results. No-op since jobs are pushed during Process.
func (p *Invariant) Join(ctx context.Context, partial rangedloop.Partial) error {
	return nil
}

// Finish is called after all segments are processed.
func (p *Invariant) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	p.log.Info("Invariant finished")
	return nil
}

// invariantFork implements rangedloop.Partial.
type invariantFork struct {
	observer *Invariant
	jobs     []any
}

var _ rangedloop.Partial = (*invariantFork)(nil)

// Process handles a batch of segments.
func (f *invariantFork) Process(ctx context.Context, segments []rangedloop.Segment) error {
	for i := range segments {
		if segments[i].Inline() {
			continue
		}
		if err := f.processSegment(ctx, &segments[i]); err != nil {
			return err
		}
	}

	// Flush remaining jobs.
	if len(f.jobs) > 0 {
		err := f.observer.client.PushBatch(ctx, f.observer.config.StreamID, f.jobs)
		if err != nil {
			return Error.Wrap(err)
		}
		f.jobs = f.jobs[:0]
	}
	return nil
}

func (f *invariantFork) processSegment(ctx context.Context, segment *rangedloop.Segment) error {
	// Skip segments that don't match the configured placement filter.
	if f.observer.config.Placement >= 0 && segment.Placement != storj.PlacementConstraint(f.observer.config.Placement) {
		return nil
	}

	// Look up placement definition.
	placement, ok := f.observer.placements[segment.Placement]
	if !ok {
		return nil
	}

	invariant := placement.Invariant
	if invariant == nil {
		return nil
	}

	// Resolve the selector for this segment's placement.
	selector, ok := f.observer.selectors[segment.Placement]
	if !ok {
		return nil
	}

	// Build nodes array parallel to segment.Pieces.
	origNodes := make([]nodeselection.SelectedNode, len(segment.Pieces))
	for i, piece := range segment.Pieces {
		if node, ok := f.observer.nodeMap[piece.StorageNode]; ok {
			origNodes[i] = *node
		}
	}

	// Evaluate invariant to find violating pieces.
	violations := invariant(segment.Pieces, origNodes)
	if violations.Count() == 0 {
		return nil
	}

	// Build excluded nodes and alreadySelected lists (shared across all attempts).
	excludedNodes := make([]storj.NodeID, 0, len(segment.Pieces))
	alreadySelected := make([]*nodeselection.SelectedNode, 0, len(segment.Pieces))
	for _, p := range segment.Pieces {
		excludedNodes = append(excludedNodes, p.StorageNode)
		if node, ok := f.observer.nodeMap[p.StorageNode]; ok {
			alreadySelected = append(alreadySelected, node)
		} else {
			alreadySelected = append(alreadySelected, &nodeselection.SelectedNode{ID: p.StorageNode})
		}
	}

	origCount := violations.Count()

	// Try each violating piece until one can be fixed.
	for i, piece := range segment.Pieces {
		if !violations.Contains(int(piece.Number)) {
			continue
		}

		// Select one replacement node.
		selected, err := selector(ctx, storj.NodeID{}, 1, excludedNodes, alreadySelected)
		if err != nil || len(selected) == 0 {
			continue
		}

		newNode := selected[0]

		// Verify it's not already in the segment.
		alreadyInSegment := false
		for _, id := range excludedNodes {
			if id == newNode.ID {
				alreadyInSegment = true
				break
			}
		}
		if alreadyInSegment {
			continue
		}

		// Simulate the swap and check that violations decrease.
		modifiedNodes := make([]nodeselection.SelectedNode, len(origNodes))
		copy(modifiedNodes, origNodes)
		modifiedNodes[i] = *newNode

		newViolations := invariant(segment.Pieces, modifiedNodes)
		if newViolations.Count() >= origCount {
			continue
		}

		job := Job{
			StreamID:   segment.StreamID,
			Position:   segment.Position.Encode(),
			SourceNode: piece.StorageNode,
			DestNode:   newNode.ID,
		}

		f.jobs = append(f.jobs, job)
		if len(f.jobs) >= 10 {
			err := f.observer.client.PushBatch(ctx, f.observer.config.StreamID, f.jobs)
			if err != nil {
				return Error.Wrap(err)
			}
			f.jobs = f.jobs[:0]
		}

		// Only move one piece per segment per pass.
		return nil
	}

	return nil
}
