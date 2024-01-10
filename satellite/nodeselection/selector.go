// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"storj.io/common/storj"
)

// UnvettedSelector selects new nodes first based on newNodeFraction, and selects old nodes for the remaining.
func UnvettedSelector(newNodeFraction float64, init NodeSelectorInit) NodeSelectorInit {
	return func(nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		var newNodes []*SelectedNode
		var oldNodes []*SelectedNode
		for _, node := range nodes {
			if node.Vetted {
				oldNodes = append(oldNodes, node)
			} else {
				newNodes = append(newNodes, node)
			}
		}

		newSelector := init(newNodes, filter)
		oldSelector := init(oldNodes, filter)
		return func(n int, alreadySelected []storj.NodeID) ([]*SelectedNode, error) {
			newNodeCount := int(float64(n) * newNodeFraction)

			selectedNewNodes, err := newSelector(newNodeCount, alreadySelected)
			if err != nil {
				return selectedNewNodes, err
			}

			remaining := n - len(selectedNewNodes)
			selectedOldNodes, err := oldSelector(remaining, alreadySelected)
			if err != nil {
				return selectedNewNodes, err
			}
			return append(selectedOldNodes, selectedNewNodes...), nil
		}
	}
}

// FilteredSelector uses another selector on the filtered list of nodes.
func FilteredSelector(preFilter NodeFilter, init NodeSelectorInit) NodeSelectorInit {
	return func(nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		var filteredNodes []*SelectedNode
		for _, node := range nodes {
			if preFilter.Match(node) {
				filteredNodes = append(filteredNodes, node)
			}
		}
		return init(filteredNodes, filter)
	}
}

// AttributeGroupSelector first selects a group with equal chance (like last_net) and choose node from the group randomly.
func AttributeGroupSelector(attribute NodeAttribute) NodeSelectorInit {
	return func(nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		nodeByAttribute := make(map[string][]*SelectedNode)
		for _, node := range nodes {
			if filter != nil && !filter.Match(node) {
				continue
			}
			a := attribute(*node)
			if _, found := nodeByAttribute[a]; !found {
				nodeByAttribute[a] = make([]*SelectedNode, 0)
			}
			nodeByAttribute[a] = append(nodeByAttribute[a], node)
		}

		var attributes []string
		for k := range nodeByAttribute {
			attributes = append(attributes, k)
		}

		return func(n int, alreadySelected []storj.NodeID) (selected []*SelectedNode, err error) {
			if n == 0 {
				return selected, nil
			}
			r := NewRandomOrder(len(nodeByAttribute))
			for r.Next() {
				nodes := nodeByAttribute[attributes[r.At()]]

				if included(alreadySelected, nodes...) {
					continue
				}

				rs := NewRandomOrder(len(nodes))
				for rs.Next() {
					selected = append(selected, nodes[rs.At()].Clone())
					break

				}
				if len(selected) >= n {
					break
				}
			}
			return selected, nil
		}
	}
}

func included(alreadySelected []storj.NodeID, nodes ...*SelectedNode) bool {
	for _, node := range nodes {
		for _, id := range alreadySelected {
			if node.ID == id {
				return true
			}
		}
	}
	return false
}

// RandomSelector selects any nodes with equal chance.
func RandomSelector() NodeSelectorInit {
	return func(nodes []*SelectedNode, filter NodeFilter) NodeSelector {

		var filteredNodes []*SelectedNode
		for _, node := range nodes {
			if filter != nil && !filter.Match(node) {
				continue
			}
			filteredNodes = append(filteredNodes, node)
		}

		return func(n int, alreadySelected []storj.NodeID) (selected []*SelectedNode, err error) {
			if n == 0 {
				return selected, nil
			}
			r := NewRandomOrder(len(filteredNodes))
			for r.Next() {
				candidate := filteredNodes[r.At()]

				if included(alreadySelected, candidate) {
					continue
				}

				selected = append(selected, candidate.Clone())
				if len(selected) >= n {
					break
				}
			}
			return selected, nil
		}
	}
}
