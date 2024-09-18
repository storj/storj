// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"math"
	"math/rand"
	"sort"
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
func ChoiceOfTwo(tracker ScoreNode, delegate NodeSelectorInit) NodeSelectorInit {
	return ChoiceOfN(tracker, 2, delegate)
}

func choiceOfNReduction(getSuccessRate func(*SelectedNode) float64, n int, nodes []*SelectedNode, desired int) []*SelectedNode {
	// shuffle the nodes to ensure the pairwise matching is fair and unbiased when the
	// totals for either node are 0 (we just pick the first node in the pair in that case)
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(nodes), func(i, j int) {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	})

	// do choice selection while we have more than the total redundancy
	for len(nodes) > desired {
		// we're going to choose between up to n nodes
		toChooseBetween := n
		excessNodes := len(nodes) - desired
		if toChooseBetween > excessNodes+1 {
			// we add one because we essentially subtract toChooseBetween nodes
			// from the list and then add the chosen node.
			toChooseBetween = excessNodes + 1
		}

		for toChooseBetween > 1 {
			success0 := getSuccessRate(nodes[0])
			success1 := getSuccessRate(nodes[1])

			// success0 and success1 could both potentially be NaN. we want to prefer a node if
			// it is NaN and if they are both NaN then it does not matter which we prefer (the
			// input list is randomly shuffled). note that ALL comparisons where one of the
			// operands is NaN evaluate to false. thus for the following if statement, we have
			// the following table:
			//
			//     success0 | success1 | result
			//    ----------|----------|-------
			//          NaN |      NaN | node1
			//          NaN |   number | node0
			//       number |      NaN | node1
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
	return nodes
}

// ChoiceOfN will perform the selection for n*x nodes and choose the best node
// from groups of n size. n is an int64 type due to a mito scripting shortcoming
// but really an int16 should be fine.
// NOTE: it may break other pre-conditions, like the results of the balanced selector...
func ChoiceOfN(tracker ScoreNode, n int64, delegate NodeSelectorInit) NodeSelectorInit {
	return func(allNodes []*SelectedNode, filter NodeFilter) NodeSelector {
		selector := delegate(allNodes, filter)
		return func(requester storj.NodeID, m int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
			nodes, err := selector(requester, int(n)*m, excluded, alreadySelected)
			if err != nil {
				return nil, err
			}

			return choiceOfNReduction(tracker.Get(requester), int(n), nodes, m), nil
		}
	}
}

// ScoreSelection can help to choose between two selections with assigning a score. The higher score is better.
type ScoreSelection func(uplink storj.NodeID, selected []*SelectedNode) float64

// ScoreNode can help to assign a score to a node. The higher score is better. float.Nan is valid, if no information is available.
type ScoreNode interface {
	Get(uplink storj.NodeID) func(node *SelectedNode) float64
}

// ScoreNodeFunc implements ScoreNode interface with a single func.
type ScoreNodeFunc func(uplink storj.NodeID, node *SelectedNode) float64

// Get implements ScoreNode.
func (s ScoreNodeFunc) Get(id storj.NodeID) func(node *SelectedNode) float64 {
	return func(node *SelectedNode) float64 {
		return s(id, node)
	}
}

// Desc is a score node, which reverses the score of the original node.
func Desc(original ScoreNode) ScoreNode {
	return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
		return -original.Get(uplink)(node)
	})
}

// PieceCount scores the node based on the piece count.
func PieceCount(divider int64) ScoreNode {
	return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
		return float64(node.PieceCount) / float64(divider)
	})
}

// LastBut scores a selection based on the worst node (but skip the worst n nodes).
func LastBut(attr ScoreNode, skip int64) ScoreSelection {
	return scoreBy(attr, func(l int) int {
		return int(skip)
	})
}

// Median scores the selection based on the median of the attribute.
func Median(attr ScoreNode) ScoreSelection {
	return scoreBy(attr, func(l int) int {
		return l/2 + 1
	})
}

func scoreBy(attr ScoreNode, indexer func(int) int) ScoreSelection {
	return func(uplink storj.NodeID, nodes []*SelectedNode) float64 {
		if len(nodes) == 0 {
			return 0
		}

		orig := attr.Get(uplink)

		var scores []float64
		for node := range nodes {

			val := orig(nodes[node])
			if !math.IsNaN(val) {
				scores = append(scores, val)
			}
		}
		slices.Sort(scores)
		if len(scores) == 0 {
			return math.NaN()
		}
		desiredIndex := indexer(len(scores))
		if desiredIndex < 0 || desiredIndex >= len(scores) {
			return math.NaN()
		}
		return scores[desiredIndex]
	}
}

// MaxGroup returns with the size of the biggest group in the node selection.
func MaxGroup(attr NodeAttribute) ScoreSelection {
	return func(uplink storj.NodeID, selected []*SelectedNode) float64 {
		var attributes []string
		for _, node := range selected {
			attributes = append(attributes, attr(*node))
		}
		sort.Strings(attributes)
		maxGroup := 0
		currentGroup := 0
		for ix, attr := range attributes {
			if ix > 0 && attributes[ix-1] == attr {
				currentGroup++
			} else {
				currentGroup = 1
			}
			if maxGroup < currentGroup {
				maxGroup = currentGroup
			}
		}

		return float64(maxGroup)
	}
}

// ChoiceOfNSelection is similar to ChoiceOfN, but doesn't break the pre-conditions of the original selector.
// it chooses from selections, without mixing nodes. scoreSources are ar judging the selections in order.
func ChoiceOfNSelection(n int64, delegate NodeSelectorInit, scoreSource ...ScoreSelection) NodeSelectorInit {
	return func(allNodes []*SelectedNode, filter NodeFilter) NodeSelector {
		selector := delegate(allNodes, filter)
		return func(requester storj.NodeID, m int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
			var bestSelection []*SelectedNode
			var bestScores []float64

			for i := 0; i < int(n); i++ {
				nodes, err := selector(requester, m, excluded, alreadySelected)
				if err != nil {
					return nil, err
				}
				if len(nodes) >= m {

					var score []float64
					for _, scoreFunc := range scoreSource {
						score = append(score, scoreFunc(requester, nodes))
					}

					if len(bestScores) == 0 || slices.Compare(score, bestScores) > 0 {
						bestSelection = nodes
						bestScores = score
					}

				}
			}

			return bestSelection, nil
		}
	}
}

// DownloadChoiceOfN will take a set of nodes and winnow it down using choice
// of n. n is an int64 type due to a mito scripting shortcoming but really an
// int16 should be fine.
func DownloadChoiceOfN(tracker UploadSuccessTracker, n int64) DownloadSelector {
	return func(requester storj.NodeID, possibleNodes map[storj.NodeID]*SelectedNode, needed int) (map[storj.NodeID]*SelectedNode, error) {
		nodeSlice := make([]*SelectedNode, 0, len(possibleNodes)+needed)
		for _, node := range possibleNodes {
			nodeSlice = append(nodeSlice, node)
		}

		nodeSlice = choiceOfNReduction(tracker.Get(requester), int(n), nodeSlice, needed)

		result := make(map[storj.NodeID]*SelectedNode, needed)
		for _, node := range nodeSlice {
			result[node.ID] = node
		}
		return result, nil
	}
}

// DownloadBest will take a set of nodes and will return just the best nodes.
func DownloadBest(tracker UploadSuccessTracker) DownloadSelector {
	return func(requester storj.NodeID, possibleNodes map[storj.NodeID]*SelectedNode, needed int) (map[storj.NodeID]*SelectedNode, error) {
		nodeSlice := make([]*SelectedNode, 0, len(possibleNodes)+needed)
		for _, node := range possibleNodes {
			nodeSlice = append(nodeSlice, node)
		}

		getSuccessRate := tracker.Get(requester)

		sort.Slice(nodeSlice, func(i, j int) bool {
			success0 := getSuccessRate(nodeSlice[i])
			success1 := getSuccessRate(nodeSlice[j])

			// we do the same thing as choiceofn where we assume NaN is better
			// than not NaN. this has the additional benefit of falling back
			// to random selection behavior for full nodes, where they still
			// get a shot.
			return success0 > success1 || math.IsNaN(success0) && !math.IsNaN(success1)
		})
		if len(nodeSlice) > needed {
			nodeSlice = nodeSlice[:needed]
		}

		result := make(map[storj.NodeID]*SelectedNode, needed)
		for _, node := range nodeSlice {
			result[node.ID] = node
		}
		return result, nil
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
			successA := getSuccessRate(a)
			successB := getSuccessRate(b)
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
func BestOfN(tracker ScoreNode, ratio float64, delegate NodeSelectorInit) NodeSelectorInit {
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
				successA := getSuccessRate(a)
				successB := getSuccessRate(b)
				if math.IsNaN(successB) || successA < successB {
					return 1
				}
				return -1
			})
			return selectedNodes[:n], nil
		}
	}
}

// EnoughFast will select `ratio` times more nodes. The fastest nodes (under splitLine) will be used the selectionRation nodes, remaining wil be chosen from the second part.
func EnoughFast(tracker UploadSuccessTracker, ratio float64, splitLine float64, selectionRatio float64, delegate NodeSelectorInit) NodeSelectorInit {
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
				successA := getSuccessRate(a)
				successB := getSuccessRate(b)
				if math.IsNaN(successB) || successA < successB {
					return 1
				}
				return -1
			})
			splitIndex := int(math.Round(splitLine * float64(len(selectedNodes))))
			slowNodes := selectedNodes[splitIndex:]
			fastNodes := selectedNodes[:splitIndex]
			requiredFast := int(math.Round(float64(n) * selectionRatio))
			selectedFast := pickRandom(fastNodes, requiredFast)
			requiredSlow := n - len(selectedFast)
			return append(selectedFast, pickRandom(slowNodes, requiredSlow)...), nil
		}
	}
}

func pickRandom(nodes []*SelectedNode, required int) (res []*SelectedNode) {
	r := NewRandomOrder(len(nodes))
	for r.Next() {
		res = append(res, nodes[r.At()])
		if len(res) >= required {
			break
		}
	}
	return res
}

// DualSelector selects fraction of nodes with first, and remaining with the second selector.
func DualSelector(fraction float64, first NodeSelectorInit, second NodeSelectorInit) NodeSelectorInit {
	return func(nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		if math.IsNaN(fraction) || fraction < 0 || fraction > 1 {
			panic("fraction is of the dual selector is invalid")
		}
		firstSelector := first(nodes, filter)
		secondSelector := second(nodes, filter)
		return func(requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) ([]*SelectedNode, error) {

			firstSelectionCount := RoundWithProbability(float64(n) * fraction)

			var err error
			var selectedFirstNodes []*SelectedNode
			if firstSelectionCount > 0 {
				selectedFirstNodes, err = firstSelector(requester, firstSelectionCount, excluded, alreadySelected)
				if err != nil {
					mon.Counter("dual_selector_failure").Inc(1)
				}
			}

			remaining := n - len(selectedFirstNodes)
			selectedSecondNodes, err := secondSelector(requester, remaining, excluded, append(alreadySelected, selectedFirstNodes...))
			if err != nil {
				return selectedSecondNodes, err
			}
			return append(selectedSecondNodes, selectedFirstNodes...), nil
		}
	}
}

// RoundWithProbability is like math.Round, but instead of rounding 2.6 to 3 all the time, it will
// round up to 3 with 60% chance, and to 2 with 40% chance.
func RoundWithProbability(r float64) int {
	if int(r*100)%100 > (rand.Intn(100) + 1) {
		return int(math.Ceil(r))
	} else {
		return int(math.Floor(r))
	}
}
