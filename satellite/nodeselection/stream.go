// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// Package nodeselection provides functionality for selecting storage nodes.
package nodeselection

import (
	"errors"
	"sort"

	"storj.io/common/storj"
)

// NodeSequence is a function that returns the next node in a sequence or nil when exhausted.
type NodeSequence func() *SelectedNode

// NodeStream creates a sequence of nodes, filtering out excluded nodes and those already selected.
type NodeStream func(requester storj.NodeID, excluded []storj.NodeID, alreadySelected []*SelectedNode) NodeSequence

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

// StreamFilter creates a Node selector based on streaming constructs.
// Streaming can select unbounded number of nodes until it finds enough good (or no more nodes). Can be slow.
func StreamFilter(filter StreamConstraint) func(stream NodeStream) NodeStream {
	return func(stream NodeStream) NodeStream {
		return func(requester storj.NodeID, excluded []storj.NodeID, alreadySelected []*SelectedNode) NodeSequence {
			iterator := stream(requester, excluded, alreadySelected)
			buffer := append(make([]*SelectedNode, 0, len(alreadySelected)), alreadySelected...)
			return func() *SelectedNode {
				for {
					next := iterator()
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

// Stream creates a node selector that uses the provided seed function to generate a stream of nodes.
// Additional processing steps can be applied to the stream before selection.
func Stream(seed func(nodes []*SelectedNode) NodeStream, steps ...func(NodeStream) NodeStream) NodeSelectorInit {
	return func(allNodes []*SelectedNode, filter NodeFilter) NodeSelector {
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

		return func(requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
			iterator := stream(requester, excluded, alreadySelected)
			for {
				next := iterator()
				if next == nil {
					return nil, errors.New("not enough nodes from stream")
				}

				if containsID(excluded, next.ID) {
					continue
				}

				if containsNode(alreadySelected, next) {
					continue
				}

				if containsNode(selected, next) {
					continue
				}

				selected = append(selected, next)

				if len(selected) == n {
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

// RandomStream creates a NodeStream that returns nodes in a random order.
// It skips nodes that are in the excluded list or already selected.
func RandomStream(allNodes []*SelectedNode) NodeStream {
	return func(requester storj.NodeID, excluded []storj.NodeID, alreadySelected []*SelectedNode) NodeSequence {
		order := NewRandomOrder(len(allNodes))
		return func() *SelectedNode {
			for order.Next() {
				candidate := allNodes[order.At()]
				if containsID(excluded, candidate.ID) {
					continue
				}
				if containsNode(alreadySelected, candidate) {
					continue
				}
				return candidate
			}
			return nil
		}
	}
}

// ChoiceOfNStream creates a stream processor that selects the best node from each batch of n nodes.
// The best node is determined by the highest score according to the provided ScoreNode.
func ChoiceOfNStream(n int64, score ScoreNode) func(NodeStream) NodeStream {
	return func(stream NodeStream) NodeStream {
		return func(requester storj.NodeID, excluded []storj.NodeID, alreadySelected []*SelectedNode) NodeSequence {
			iterator := stream(requester, excluded, alreadySelected)
			return func() *SelectedNode {
				buffer := make([]*SelectedNode, 0, n)

				for len(buffer) < int(n) {
					next := iterator()
					if next == nil {
						break
					}
					buffer = append(buffer, next)
				}

				if len(buffer) == 0 {
					return nil
				}

				sc := score.Get(requester)
				sort.Slice(buffer, func(i, j int) bool {
					return sc(buffer[i]) > sc(buffer[j])
				})
				return buffer[0]
			}
		}
	}
}
