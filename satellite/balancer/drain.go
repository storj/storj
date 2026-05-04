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

// DrainConfig holds the configuration for the Drain observer.
type DrainConfig struct {
	// StreamID is the Redis stream name used for drain jobs.
	StreamID string `help:"Redis stream name for drain jobs" default:"drain"`
	// NodeFilter is the filter expression selecting nodes to drain.
	NodeFilter string `help:"filter expression for nodes to drain"`
	// Selector is a node selector expression (e.g. random(), subnet()). If empty, the selector from each segment's placement is used.
	Selector string `help:"node selector expression; if empty, the selector from each segment's placement is used"`
}

// Drain implements rangedloop.Observer.
// It finds segments with pieces on nodes that should be drained and generates
// jobs to move those pieces to new nodes selected via the configured placement selector.
type Drain struct {
	log         *zap.Logger
	config      DrainConfig
	uploadCache *overlay.UploadSelectionCache
	placements  nodeselection.PlacementDefinitions
	client      *taskqueue.Client

	// state populated during Start, read-only during Fork/Process
	drainNodes map[storj.NodeID]bool
	// nodeMap maps node IDs to their full SelectedNode data (from upload selection cache).
	nodeMap map[storj.NodeID]*nodeselection.SelectedNode
	// selector is non-nil when config.Selector is set (single selector for all segments).
	selector nodeselection.NodeSelector
	// selectors is populated when config.Selector is empty (per-placement selectors).
	selectors map[storj.PlacementConstraint]nodeselection.NodeSelector
}

// NewDrain creates a new Drain observer.
func NewDrain(
	log *zap.Logger,
	uploadCache *overlay.UploadSelectionCache,
	placements nodeselection.PlacementDefinitions,
	client *taskqueue.Client,
	config DrainConfig,
) *Drain {
	return &Drain{
		log:         log,
		config:      config,
		uploadCache: uploadCache,
		placements:  placements,
		client:      client,
	}
}

var _ rangedloop.Observer = (*Drain)(nil)

// Start is called at the beginning of each segment loop.
func (d *Drain) Start(ctx context.Context, startTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	d.drainNodes = make(map[storj.NodeID]bool)

	// Parse drain node filter.
	if d.config.NodeFilter == "" {
		return Error.New("drain node filter must be configured")
	}

	drainFilter, err := nodeselection.FilterFromString(
		d.config.NodeFilter,
		nodeselection.NewPlacementConfigEnvironment(nil, nil),
	)
	if err != nil {
		return Error.Wrap(err)
	}

	// Load all upload-eligible nodes from the cache (excludes suspended, offline, exiting nodes).
	allNodes, err := d.uploadCache.GetAllNodes(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	// Build node map and identify drain nodes.
	d.nodeMap = make(map[storj.NodeID]*nodeselection.SelectedNode, len(allNodes))
	for _, node := range allNodes {
		d.nodeMap[node.ID] = node
		if drainFilter.Match(node) {
			d.drainNodes[node.ID] = true
		}
	}

	if len(d.drainNodes) == 0 {
		d.log.Warn("No nodes matched drain filter", zap.String("filter", d.config.NodeFilter))
	}

	// Initialize selector(s).
	if d.config.Selector != "" {
		// Parse the configured selector and use it for all segments.
		selectorInit, err := nodeselection.SelectorFromString(
			d.config.Selector,
			nodeselection.NewPlacementConfigEnvironment(nil, nil),
		)
		if err != nil {
			return Error.Wrap(err)
		}
		d.selector = selectorInit(ctx, allNodes, nodeselection.AnyFilter{})
		d.selectors = nil
	} else {
		// Build per-placement selectors from placement definitions.
		d.selector = nil
		d.selectors = make(map[storj.PlacementConstraint]nodeselection.NodeSelector)
		for id, placement := range d.placements {
			selectorInit := placement.Selector
			if selectorInit == nil {
				selectorInit = nodeselection.RandomSelector()
			}
			nodeFilter := placement.NodeFilter
			if nodeFilter == nil {
				nodeFilter = nodeselection.AnyFilter{}
			}
			if placement.UploadFilter != nil {
				nodeFilter = nodeselection.NodeFilters{
					nodeFilter,
					placement.UploadFilter,
				}
			}
			d.selectors[id] = selectorInit(ctx, allNodes, nodeFilter)
		}
	}

	d.log.Info("Drain started",
		zap.Int("drain_nodes", len(d.drainNodes)),
		zap.Int("total_nodes", len(allNodes)),
		zap.String("selector", d.config.Selector),
	)

	return nil
}

// Fork creates a new partial for processing a range.
func (d *Drain) Fork(ctx context.Context) (rangedloop.Partial, error) {
	return &drainFork{
		observer: d,
	}, nil
}

// Join merges partial results. No-op since jobs are pushed during Process.
func (d *Drain) Join(ctx context.Context, partial rangedloop.Partial) error {
	return nil
}

// Finish is called after all segments are processed.
func (d *Drain) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	d.log.Info("Drain finished")
	return nil
}

// drainFork implements rangedloop.Partial.
type drainFork struct {
	observer *Drain
	jobs     []any
}

var _ rangedloop.Partial = (*drainFork)(nil)

// Process handles a batch of segments.
func (f *drainFork) Process(ctx context.Context, segments []rangedloop.Segment) error {
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

func (f *drainFork) processSegment(ctx context.Context, segment *rangedloop.Segment) error {
	drainNodes := f.observer.drainNodes

	// Resolve the selector for this segment.
	selector := f.observer.selector
	if selector == nil {
		var ok bool
		selector, ok = f.observer.selectors[segment.Placement]
		if !ok {
			// No selector for this placement, skip.
			return nil
		}
	}

	// Find pieces on drain nodes.
	for _, piece := range segment.Pieces {
		if !drainNodes[piece.StorageNode] {
			continue
		}

		alreadySelected := make([]*nodeselection.SelectedNode, 0, len(segment.Pieces))
		for _, p := range segment.Pieces {
			if node, ok := f.observer.nodeMap[p.StorageNode]; ok {
				alreadySelected = append(alreadySelected, node)
			} else {
				alreadySelected = append(alreadySelected, &nodeselection.SelectedNode{ID: p.StorageNode})
			}
		}

		// Select one replacement node using the placement selector.
		selected, err := selector(ctx, storj.NodeID{}, 1, []storj.NodeID{}, alreadySelected)
		if err != nil || len(selected) == 0 {
			// No suitable replacement found, skip this piece.
			continue
		}

		newNode := selected[0]

		// Verify it's actually a new node (not already in segment).
		alreadyInSegment := false
		for _, id := range segment.Pieces {
			if id.StorageNode == newNode.ID {
				alreadyInSegment = true
				break
			}
		}
		if alreadyInSegment {
			f.observer.log.Warn("Selector returned node already in segment",
				zap.Stringer("node", newNode.ID),
				zap.Stringer("stream_id", segment.StreamID),
			)
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

		// Only move one piece per segment per drain pass
		return nil
	}

	return nil
}
