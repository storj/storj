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
		return func(n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) ([]*SelectedNode, error) {
			newNodeCount := int(float64(n) * newNodeFraction)

			selectedNewNodes, err := newSelector(newNodeCount, excluded, alreadySelected)
			if err != nil {
				return selectedNewNodes, err
			}

			remaining := n - len(selectedNewNodes)
			selectedOldNodes, err := oldSelector(remaining, excluded, alreadySelected)
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

		return func(n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
			if n == 0 {
				return selected, nil
			}
			r := NewRandomOrder(len(nodeByAttribute))
			for r.Next() {
				nodes := nodeByAttribute[attributes[r.At()]]

				if includedInNodes(alreadySelected, nodes...) {
					continue
				}

				rs := NewRandomOrder(len(nodes))
				for rs.Next() {
					candidate := nodes[rs.At()].Clone()
					if !included(excluded, candidate) && !includedInNodes(selected, candidate) {
						selected = append(selected, nodes[rs.At()].Clone())
					}
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

func includedInNodes(alreadySelected []*SelectedNode, nodes ...*SelectedNode) bool {
	for _, node := range nodes {
		for _, as := range alreadySelected {
			if node.ID == as.ID {
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

		return func(n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
			if n == 0 {
				return selected, nil
			}
			r := NewRandomOrder(len(filteredNodes))
			for r.Next() {
				candidate := filteredNodes[r.At()]

				if includedInNodes(alreadySelected, candidate) || included(excluded, candidate) || includedInNodes(selected, candidate) {
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

// FilterSelector is a specific selector, which can filter out nodes from the upload selection.
// Note: this is different from the generic filter attribute of the NodeSelectorInit, as that is applied to all node selection (upload/download/repair).
func FilterSelector(loadTimeFilter NodeFilter, init NodeSelectorInit) NodeSelectorInit {
	return func(nodes []*SelectedNode, selectionFilter NodeFilter) NodeSelector {
		var filtered []*SelectedNode
		for _, n := range nodes {
			if loadTimeFilter.Match(n) {
				filtered = append(filtered, n)
			}
		}
		return init(filtered, selectionFilter)
	}
}

// BalancedGroupBasedSelector first selects a group with equal chance (like last_net) and choose one single node randomly. .
// One group can be tried multiple times, and if the node is already selected, it will be ignored.
func BalancedGroupBasedSelector(attribute NodeAttribute) NodeSelectorInit {
	return func(nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		mon.IntVal("selector_balanced_input_nodes").Observe(int64(len(nodes)))
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

		var groupedNodes [][]*SelectedNode
		count := 0
		for _, nodeList := range nodeByAttribute {
			groupedNodes = append(groupedNodes, nodeList)
			count += len(nodeList)
		}
		mon.IntVal("selector_balanced_filtered_nodes").Observe(int64(count))
		return func(n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
			if n == 0 {
				return selected, nil
			}

			// random order to iterate on groups
			rGroup := NewRandomOrder(len(groupedNodes))

			// random orders inside each groups
			rCandidates := make([]RandomOrder, len(groupedNodes))
			for i := range rCandidates {
				rCandidates[i] = NewRandomOrder(len(groupedNodes[i]))
			}

			for {
				rGroup.Reset()

				alreadyFinished := 0

				// check all the groups in random order, each group can delegate one node in this turn
				for rGroup.Next() {
					if len(selected) >= n {
						break
					}
					rCandidate := &rCandidates[rGroup.At()]
					if rCandidate.Finished() {
						// no more chance in this group
						alreadyFinished++
						continue
					}

					nodes := groupedNodes[rGroup.At()]

					// in each group, we will try to select one, which is good enough
					for rCandidate.Next() {
						randomOne := nodes[rCandidate.At()].Clone()
						if !includedInNodes(alreadySelected, randomOne) &&
							!included(excluded, randomOne) &&
							!includedInNodes(selected, randomOne) {
							selected = append(selected, randomOne)
							break
						}
					}

				}

				if len(selected) >= n || len(rCandidates) == alreadyFinished {
					mon.IntVal("selector_balanced_requested").Observe(int64(n))
					mon.IntVal("selector_balanced_found").Observe(int64(len(selected)))
					return selected, nil
				}
			}
		}
	}
}
