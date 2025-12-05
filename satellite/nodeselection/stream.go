// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// Package nodeselection provides functionality for selecting storage nodes.
package nodeselection

import (
	"context"
	"errors"
	"fmt"

	"storj.io/common/storj"
)

// NodeSequence is a function that returns the next node in a sequence or nil when exhausted.
type NodeSequence func(ctx context.Context) *SelectedNode

// NodeStream creates a sequence of nodes, filtering out excluded nodes and those already selected.
type NodeStream func(ctx context.Context, requester storj.NodeID, excluded []storj.NodeID, alreadySelected []*SelectedNode) NodeSequence

// StreamConstraint is a function that determines if a node should be included in a stream.
// Returns true if the node should be included, false otherwise.
type StreamConstraint func([]*SelectedNode, *SelectedNode) bool

// GroupConstraint creates a constraint that limits the number of nodes with the same attribute value.
func GroupConstraint(attribute NodeAttribute, max int64) func([]*SelectedNode, *SelectedNode) bool {
	return func(nodes []*SelectedNode, node *SelectedNode) bool {
		newAttr := attribute(*node)
		counter := int64(0)
		for _, existing := range nodes {
			if attribute(*existing) == newAttr {
				counter++
				if counter == max {
					break
				}
			}
		}
		return counter < max
	}
}

var streamFilterTask = mon.Task()

// StreamFilter creates a Node selector based on streaming constructs.
// Streaming can select unbounded number of nodes until it finds enough good (or no more nodes). Can be slow.
func StreamFilter(filter StreamConstraint) func(stream NodeStream) NodeStream {
	return func(stream NodeStream) NodeStream {
		return func(ctx context.Context, requester storj.NodeID, excluded []storj.NodeID, alreadySelected []*SelectedNode) NodeSequence {
			defer streamFilterTask(&ctx)(nil)
			iterator := stream(ctx, requester, excluded, alreadySelected)
			buffer := append(make([]*SelectedNode, 0, len(alreadySelected)), alreadySelected...)
			return func(ctx context.Context) *SelectedNode {
				for {
					next := iterator(ctx)
					if next == nil {
						return nil
					}
					if filter(buffer, next) {
						buffer = append(buffer, next)
						return next
					}
				}
			}
		}
	}
}

var (
	streamTask               = mon.Task()
	streamSelectionTask      = mon.Task()
	streamEmptyMeter         = mon.Meter("stream-empty")
	streamSufficientMeter    = mon.Meter("stream-sufficient")
	nodeExcludedMeter        = mon.Meter("stream-node-excluded")
	nodeAlreadySelectedMeter = mon.Meter("stream-node-already-selected")
	nodeSelectedTwiceMeter   = mon.Meter("stream-node-selected-twice")
	nodeSelectedMeter        = mon.Meter("stream-node-selected")
)

// Stream creates a node selector that uses the provided seed function to generate a stream of nodes.
// Additional processing steps can be applied to the stream before selection.
func Stream(seed func(nodes []*SelectedNode) NodeStream, steps ...func(NodeStream) NodeStream) NodeSelectorInit {
	return func(ctx context.Context, allNodes []*SelectedNode, filter NodeFilter) NodeSelector {
		defer streamTask(&ctx)(nil)

		var filtered []*SelectedNode
		for _, node := range allNodes {
			if filter != nil && !filter.Match(node) {
				continue
			}
			filtered = append(filtered, node)
		}

		stream := seed(filtered)
		for _, step := range steps {
			stream = step(stream)
		}

		return func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
			defer streamSelectionTask(&ctx)(&err)
			iterator := stream(ctx, requester, excluded, alreadySelected)
			for {
				next := iterator(ctx)
				if next == nil {
					streamEmptyMeter.Mark(1)
					return nil, errors.New("not enough nodes from stream")
				}

				if containsID(excluded, next.ID) {
					nodeExcludedMeter.Mark(1)
					continue
				}

				if containsNode(alreadySelected, next) {
					nodeAlreadySelectedMeter.Mark(1)
					continue
				}

				if containsNode(selected, next) {
					// is this just defensiveness? why is this check here?
					nodeSelectedTwiceMeter.Mark(1)
					continue
				}

				nodeSelectedMeter.Mark(1)
				selected = append(selected, next)

				if len(selected) == n {
					streamSufficientMeter.Mark(1)

					// okay okay, let me talk about cardinality here - n is the reed solomon
					// number we're trying to select. this has very low cardinality. we have
					// a handful of different rs settings across all the different products,
					// maybe 3 or 4 active ones. so this has low cardinality
					mon.IntVal(fmt.Sprintf("stream-requested-%d-got", n)).Observe(int64(len(selected)))

					return selected, nil
				}
			}
		}
	}
}

func containsID(ids []storj.NodeID, id storj.NodeID) bool {
	for _, i := range ids {
		if i == id {
			return true
		}
	}
	return false
}

func containsNode(nodes []*SelectedNode, node *SelectedNode) bool {
	for _, n := range nodes {
		if n.ID == node.ID {
			return true
		}
	}
	return false
}

var (
	randomStreamSeqTask                  = mon.Task()
	randomStreamIterTask                 = mon.Task()
	randomStreamEmptyMeter               = mon.Meter("random-stream-empty")
	randomStreamNodeExcludedMeter        = mon.Meter("random-stream-node-excluded")
	randomStreamNodeAlreadySelectedMeter = mon.Meter("random-stream-node-already-selected")
	randomStreamNodeSelectedMeter        = mon.Meter("random-stream-node-selected")
	randomStreamAllNodeCount             = mon.IntVal("random-stream-all-node-count")
)

// RandomStream creates a NodeStream that returns nodes in a random order.
// It skips nodes that are in the excluded list or already selected.
func RandomStream(allNodes []*SelectedNode) NodeStream {
	return func(ctx context.Context, requester storj.NodeID, excluded []storj.NodeID, alreadySelected []*SelectedNode) NodeSequence {
		defer randomStreamSeqTask(&ctx)(nil)
		order := NewRandomOrder(len(allNodes))
		randomStreamAllNodeCount.Observe(int64(len(allNodes)))
		return func(ctx context.Context) *SelectedNode {
			defer randomStreamIterTask(&ctx)(nil)
			for order.Next() {
				candidate := allNodes[order.At()]
				if containsID(excluded, candidate.ID) {
					randomStreamNodeExcludedMeter.Mark(1)
					continue
				}
				if containsNode(alreadySelected, candidate) {
					randomStreamNodeAlreadySelectedMeter.Mark(1)
					continue
				}
				randomStreamNodeSelectedMeter.Mark(1)
				return candidate
			}
			randomStreamEmptyMeter.Mark(1)
			return nil
		}
	}
}

var (
	choiceOfNStreamSeqTask                  = mon.Task()
	choiceOfNStreamIterTask                 = mon.Task()
	choiceOfNStreamEmptyMeter               = mon.Meter("choiceofn-stream-empty")
	choiceOfNStreamNodeExcludedMeter        = mon.Meter("choiceofn-stream-node-excluded")
	choiceOfNStreamNodeAlreadySelectedMeter = mon.Meter("choiceofn-stream-node-already-selected")
	choiceOfNStreamNodeSelectedMeter        = mon.Meter("choiceofn-stream-node-selected")
)

// ChoiceOfNStream creates a stream processor that selects the best node from each batch of n nodes.
// The best node is determined by the highest score according to the provided ScoreNode.
func ChoiceOfNStream(n int64, score ScoreNode) func(NodeStream) NodeStream {
	return func(stream NodeStream) NodeStream {
		return func(ctx context.Context, requester storj.NodeID, excluded []storj.NodeID, alreadySelected []*SelectedNode) NodeSequence {
			defer choiceOfNStreamSeqTask(&ctx)(nil)
			iterator := stream(ctx, requester, excluded, alreadySelected)
			return func(ctx context.Context) *SelectedNode {
				defer choiceOfNStreamIterTask(&ctx)(nil)
				buffer := make([]*SelectedNode, 0, n)

				for len(buffer) < int(n) {
					next := iterator(ctx)
					if next == nil {
						break
					}

					if containsID(excluded, next.ID) {
						choiceOfNStreamNodeExcludedMeter.Mark(1)
						continue
					}

					if containsNode(alreadySelected, next) {
						choiceOfNStreamNodeAlreadySelectedMeter.Mark(1)
						continue
					}

					buffer = append(buffer, next)
				}

				if len(buffer) == 0 {
					choiceOfNStreamEmptyMeter.Mark(1)
					return nil
				}

				sc := score.Get(requester)
				bestidx := 0
				bestScore := sc(buffer[bestidx])
				for i := 1; i < len(buffer); i++ {
					score := sc(buffer[i])
					if choiceOfNBetter(score, bestScore) {
						bestidx = i
						bestScore = score
					}
				}
				choiceOfNStreamNodeSelectedMeter.Mark(1)
				return buffer[bestidx]
			}
		}
	}
}
