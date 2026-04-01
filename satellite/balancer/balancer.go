// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package balancer

import (
	"context"
	"sort"
	"sync/atomic"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/taskqueue"
)

var mon = monkit.Package()

// Error is the error class for the balancer package.
var Error = errs.Class("balancer")

// Config holds the configuration for the Balancer observer.
type Config struct {
	// StreamID is the Redis stream name used for balancer jobs.
	StreamID string `help:"Redis stream name for balancer jobs" default:"balancer"`
	// NodeFilter is the filter expression for selecting participating nodes.
	NodeFilter string `help:"filter expression for participating nodes"`
	// Grouping is the node attribute used to group nodes (e.g., last_net, country, tag:signer/key).
	Grouping string `help:"node attribute for grouping nodes" default:"last_net"`
}

// Job represents a rebalancing task in the queue.
type Job struct {
	StreamID   uuid.UUID    `redis:"stream_id"`
	Position   uint64       `redis:"position"`
	SourceNode storj.NodeID `redis:"source_node"`
	DestNode   storj.NodeID `redis:"dest_node"`
}

// nodeInfo holds computed balancing data for a single node.
type nodeInfo struct {
	node         nodeselection.SelectedNode
	group        string
	expectedFree int64 // average free disk for the group
	currentFree  int64 // current FreeDisk from overlay
}

// Balancer implements rangedloop.Observer.
// It identifies segments that should be moved to rebalance disk usage across nodes.
type Balancer struct {
	log             *zap.Logger
	config          Config
	uploadNodeCache *overlay.UploadNodeCache
	placements      nodeselection.PlacementDefinitions
	client          *taskqueue.Client

	// state populated during Start, read-only during Fork/Process
	nodeCache           map[storj.NodeID]*nodeInfo
	groupDestCandidates map[string][]*nodeInfo
}

// NewBalancer creates a new Balancer observer.
func NewBalancer(
	log *zap.Logger,
	uploadNodeCache *overlay.UploadNodeCache,
	placements nodeselection.PlacementDefinitions,
	client *taskqueue.Client,
	config Config,
) *Balancer {
	return &Balancer{
		log:             log,
		config:          config,
		uploadNodeCache: uploadNodeCache,
		placements:      placements,
		client:          client,
	}
}

var _ rangedloop.Observer = (*Balancer)(nil)

// Start is called at the beginning of each segment loop.
func (b *Balancer) Start(ctx context.Context, startTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	b.nodeCache = make(map[storj.NodeID]*nodeInfo)
	b.groupDestCandidates = make(map[string][]*nodeInfo)

	// Parse node filter.
	var parsedFilter nodeselection.NodeFilter
	if b.config.NodeFilter != "" {
		f, err := nodeselection.FilterFromString(
			b.config.NodeFilter,
			nodeselection.NewPlacementConfigEnvironment(nil, nil),
		)
		if err != nil {
			return Error.Wrap(err)
		}
		parsedFilter = f
	} else {
		parsedFilter = nodeselection.AnyFilter{}
	}

	// Parse grouping attribute.
	groupAttr, err := nodeselection.CreateNodeAttribute(b.config.Grouping)
	if err != nil {
		return Error.Wrap(err)
	}

	// Load all participating nodes from the upload node cache.
	cachedNodes, err := b.uploadNodeCache.GetAllNodes(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	nodes := make([]nodeselection.SelectedNode, len(cachedNodes))
	for i, n := range cachedNodes {
		nodes[i] = *n
	}

	// Filter and group nodes.
	type groupAccum struct {
		totalFree int64
		count     int64
	}
	groups := make(map[string]*groupAccum)
	var filtered []nodeselection.SelectedNode

	for _, node := range nodes {
		if !parsedFilter.Match(&node) {
			continue
		}
		filtered = append(filtered, node)
		group := groupAttr(node)
		acc, ok := groups[group]
		if !ok {
			acc = &groupAccum{}
			groups[group] = acc
		}
		acc.totalFree += node.FreeDisk
		acc.count++
	}

	// Calculate averages and populate nodeCache.
	groupAvg := make(map[string]int64, len(groups))
	for name, acc := range groups {
		if acc.count > 0 {
			groupAvg[name] = acc.totalFree / acc.count
		}
	}

	for i := range filtered {
		node := &filtered[i]
		group := groupAttr(*node)
		b.nodeCache[node.ID] = &nodeInfo{
			node:         *node,
			group:        group,
			expectedFree: groupAvg[group],
			currentFree:  node.FreeDisk,
		}
	}

	// Pre-build sorted destination candidates per group.
	for _, info := range b.nodeCache {
		surplus := info.currentFree - info.expectedFree
		if surplus > 0 {
			b.groupDestCandidates[info.group] = append(
				b.groupDestCandidates[info.group], info,
			)
		}
	}
	for _, candidates := range b.groupDestCandidates {
		sort.Slice(candidates, func(i, j int) bool {
			return (candidates[i].currentFree - candidates[i].expectedFree) >
				(candidates[j].currentFree - candidates[j].expectedFree)
		})
	}

	b.log.Info("Balancer started",
		zap.Int("participating_nodes", len(b.nodeCache)),
		zap.Int("groups", len(groups)),
		zap.Int("placements", len(b.placements)),
	)

	return nil
}

// Fork creates a new partial for processing a range.
func (b *Balancer) Fork(ctx context.Context) (rangedloop.Partial, error) {
	return &balancerFork{
		observer: b,
	}, nil
}

// Join merges partial results. No-op since jobs are pushed during Process.
func (b *Balancer) Join(ctx context.Context, partial rangedloop.Partial) error {
	return nil
}

// Finish is called after all segments are processed.
func (b *Balancer) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	b.log.Info("Balancer finished")
	return nil
}

// balancerFork implements rangedloop.Partial.
type balancerFork struct {
	observer *Balancer
	jobs     []any
}

var _ rangedloop.Partial = (*balancerFork)(nil)

// Process handles a batch of segments.
func (f *balancerFork) Process(ctx context.Context, segments []rangedloop.Segment) error {
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

func (f *balancerFork) processSegment(ctx context.Context, segment *rangedloop.Segment) error {
	nc := f.observer.nodeCache

	// Step 1: Find pieces on participating nodes that are overfull.
	// "Overfull" means expectedFree - currentFree > 0 (less free space than group average).
	type pieceCandidate struct {
		pieceIdx   int
		info       *nodeInfo
		difference int64 // expectedFree - currentFree; positive = overfull
	}

	var candidates []pieceCandidate
	for i, piece := range segment.Pieces {
		info, ok := nc[piece.StorageNode]
		if !ok {
			continue
		}
		currentFree := atomic.LoadInt64(&info.currentFree)
		diff := info.expectedFree - currentFree
		if diff > 0 {
			candidates = append(candidates, pieceCandidate{
				pieceIdx:   i,
				info:       info,
				difference: diff,
			})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Step 2: Pick the most overfull node as the source.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].difference > candidates[j].difference
	})
	source := candidates[0]
	sourceGroup := source.info.group

	// Step 3: Get destination candidates from pre-sorted list, excluding nodes
	// already holding a piece in this segment.
	usedNodes := make(map[storj.NodeID]bool, len(segment.Pieces))
	for _, piece := range segment.Pieces {
		usedNodes[piece.StorageNode] = true
	}

	destCandidatesAll := f.observer.groupDestCandidates[sourceGroup]
	if len(destCandidatesAll) == 0 {
		return nil
	}

	// Step 4: Check placement invariant for each candidate.
	placement, ok := f.observer.placements[segment.Placement]
	if !ok {
		f.observer.log.Debug("skipping segment with unknown placement",
			zap.Int("placement", int(segment.Placement)))
		return nil
	}
	invariant := placement.Invariant
	if invariant == nil {
		invariant = nodeselection.AllGood()
	}

	// Build original nodes array parallel to segment.Pieces.
	origPieces := segment.Pieces
	origNodes := make([]nodeselection.SelectedNode, len(origPieces))
	for i, piece := range origPieces {
		if info, ok := nc[piece.StorageNode]; ok {
			origNodes[i] = info.node
		}
	}

	origViolations := invariant(origPieces, origNodes)
	origCount := origViolations.Count()

	sourcePieceIdx := source.pieceIdx

	for _, dest := range destCandidatesAll {
		if usedNodes[dest.node.ID] {
			continue
		}

		// Skip if this candidate is no longer underfull due to previous moves.
		if atomic.LoadInt64(&dest.currentFree) <= dest.expectedFree {
			continue
		}

		// Build modified nodes: replace source node with destination node.
		modifiedNodes := make([]nodeselection.SelectedNode, len(origNodes))
		copy(modifiedNodes, origNodes)
		modifiedNodes[sourcePieceIdx] = dest.node

		newViolations := invariant(origPieces, modifiedNodes)
		newCount := newViolations.Count()

		if newCount <= origCount {
			job := Job{
				StreamID:   segment.StreamID,
				Position:   segment.Position.Encode(),
				SourceNode: source.info.node.ID,
				DestNode:   dest.node.ID,
			}

			// Adjust currentFree: source gains space, destination loses space.
			pieceSize := segment.PieceSize()
			atomic.AddInt64(&source.info.currentFree, pieceSize)
			atomic.AddInt64(&dest.currentFree, -pieceSize)

			f.jobs = append(f.jobs, job)
			if len(f.jobs) >= 10 {
				err := f.observer.client.PushBatch(ctx, f.observer.config.StreamID, f.jobs)
				if err != nil {
					return Error.Wrap(err)
				}
				f.jobs = f.jobs[:0]
			}
			return nil
		}
	}

	return nil
}
