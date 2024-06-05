// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slices"

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
		return func(requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) ([]*SelectedNode, error) {
			if math.IsNaN(newNodeFraction) || newNodeFraction <= 0 {
				return oldSelector(requester, n, excluded, alreadySelected)
			}

			var newNodeCount int
			if r := float64(n) * newNodeFraction; r < 1 {
				// Don't select any unvetted node.
				// Add 1 to random result to return 100 if the random function returns 99 and avoid to
				// always fail this condition if r is greater or equal than 0.99.
				if int(r*100) > (rand.Intn(100) + 1) {
					return oldSelector(requester, n, excluded, alreadySelected)
				}

				// Select one unvetted node.
				newNodeCount = 1

			} else {
				// Truncate to select the whole number part of unvetted nodes.
				newNodeCount = int(r)
			}

			selectedNewNodes, err := newSelector(requester, newNodeCount, excluded, alreadySelected)
			if err != nil {
				return selectedNewNodes, err
			}

			remaining := n - len(selectedNewNodes)
			selectedOldNodes, err := oldSelector(requester, remaining, excluded, alreadySelected)
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

		return func(requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
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

// IfSelector selects the first node attribute if the condition is true, otherwise the second node attribute.
func IfSelector(condition func(SelectedNode) bool, conditionTrue, conditionFalse NodeAttribute) NodeAttribute {
	return func(node SelectedNode) string {
		if condition(node) {
			return conditionTrue(node)
		}
		return conditionFalse(node)
	}
}

// EqualSelector returns a function that compares the node attribute with the given attribute.
func EqualSelector(nodeAttribute NodeAttribute, attribute string) func(SelectedNode) bool {
	return func(node SelectedNode) bool {
		return nodeAttribute(node) == attribute
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

		return func(id storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
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
		return func(id storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
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

// ChoiceOfTwo will repeat the selection and choose the better node pair-wise.
// NOTE: it may break other pre-conditions, like the results of the balanced selector...
func ChoiceOfTwo(tracker UploadSuccessTracker, delegate NodeSelectorInit) NodeSelectorInit {
	return ChoiceOfN(tracker, 2, delegate)
}

// ChoiceOfN will perform the selection for n*x nodes and choose the best node
// from groups of n size. n is an int64 type due to a mito scripting shortcoming
// but really an int16 should be fine.
// NOTE: it may break other pre-conditions, like the results of the balanced selector...
func ChoiceOfN(tracker UploadSuccessTracker, n int64, delegate NodeSelectorInit) NodeSelectorInit {
	return func(allNodes []*SelectedNode, filter NodeFilter) NodeSelector {
		selector := delegate(allNodes, filter)
		return func(requester storj.NodeID, m int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {

			getSuccessRate := tracker.Get(requester)
			nodes, err := selector(requester, int(n)*m, excluded, alreadySelected)
			if err != nil {
				return nil, err
			}

			// shuffle the nodes to ensure the pairwise matching is fair and unbiased when the
			// totals for either node are 0 (we just pick the first node in the pair in that case)
			rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(nodes), func(i, j int) {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			})

			// do choice selection while we have more than the total redundancy
			for len(nodes) > m {
				// we're going to choose between up to n nodes
				toChooseBetween := int(n)
				excessNodes := len(nodes) - m
				if toChooseBetween > excessNodes+1 {
					// we add one because we essentially subtract toChooseBetween nodes
					// from the list and then add the chosen node.
					toChooseBetween = excessNodes + 1
				}

				for toChooseBetween > 1 {
					success0 := getSuccessRate(nodes[0].ID)
					success1 := getSuccessRate(nodes[1].ID)

					// success0 and success1 could both potentially be NaN. we want to prefer a node if
					// it is NaN and if they are both NaN then it does not matter which we prefer (the
					// input list is randomly shuffled). note that ALL comparisons where one of the
					// operands is NaN evaluate to false. thus for the following if statement, we have
					// the following table:
					//
					//     success0 | success1 | result
					//    ----------|----------|-------
					//          NaN |      NaN | node1
					//          NaN |   number | node1
					//       number |      NaN | node0
					//       number |   number | whoever is larger

					if math.IsNaN(success1) || success1 > success0 {
						// nodes[1] is selected
						nodes = nodes[1:]
					} else {
						// nodes[0] is selected
						nodes[1] = nodes[0]
						nodes = nodes[1:]
					}
					toChooseBetween--
				}

				// move the selected node to the back
				nodes = append(nodes[1:], nodes[0])
			}
			return nodes, nil
		}
	}
}

// FilterBest is a selector, which keeps only the best nodes (based on percentage, or fixed number of nodes).
// this selector will permanently ban the worst nodes for the period of nodeselection cache refresh.
func FilterBest(tracker UploadSuccessTracker, selection string, uplink string, delegate NodeSelectorInit) NodeSelectorInit {
	var uplinkID storj.NodeID
	if uplink != "" {
		var err error
		uplinkID, err = storj.NodeIDFromString(uplink)
		if err != nil {
			panic(err)
		}
	}
	var percentage bool
	var limit int
	if strings.HasSuffix(selection, "%") {
		percentage = true
		selection = strings.TrimSuffix(selection, "%")
	}

	limit, err := strconv.Atoi(selection)
	if err != nil {
		panic(err)
	}

	return func(nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		var filteredNodes []*SelectedNode
		for _, node := range nodes {
			if filter != nil && !filter.Match(node) {
				continue
			}
			filteredNodes = append(filteredNodes, node)
		}
		nodes = filteredNodes
		getSuccessRate := tracker.Get(uplinkID)

		slices.SortFunc(nodes, func(a, b *SelectedNode) int {
			successA := getSuccessRate(a.ID)
			successB := getSuccessRate(b.ID)
			if math.IsNaN(successB) || successB > successA {
				return 1
			}
			return -1
		})

		targetNumber := limit
		// if percentage suffix is used, it's the best n% what we need.
		if percentage {
			targetNumber = len(nodes) * targetNumber / 100
		}

		// if  limit is negative, we define the long tail to be cut off.
		if targetNumber < 0 {
			targetNumber = len(nodes) + targetNumber
			if targetNumber < 0 {
				targetNumber = 0
			}
		}

		// if limit is positive, it's the number of nodes to be kept
		if targetNumber > len(nodes) {
			targetNumber = len(nodes)
		}
		nodes = nodes[:targetNumber]
		return delegate(nodes, filter)
	}
}

// BestOfN selects more nodes than the required one, and choose the fastest from those.
func BestOfN(tracker UploadSuccessTracker, ratio float64, delegate NodeSelectorInit) NodeSelectorInit {
	return func(nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		wrappedSelector := delegate(nodes, filter)
		return func(requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) ([]*SelectedNode, error) {
			getSuccessRate := tracker.Get(requester)

			nodesToSelect := int(ratio * float64(n))
			selectedNodes, err := wrappedSelector(requester, nodesToSelect, excluded, alreadySelected)
			if err != nil {
				return selectedNodes, err
			}

			if len(selectedNodes) < n {
				return selectedNodes, nil
			}

			slices.SortFunc(selectedNodes, func(a, b *SelectedNode) int {
				successA := getSuccessRate(a.ID)
				successB := getSuccessRate(b.ID)
				if math.IsNaN(successB) || successA < successB {
					return 1
				}
				return -1
			})
			return selectedNodes[:n], nil
		}
	}
}
