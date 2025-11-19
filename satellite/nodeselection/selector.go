// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"golang.org/x/exp/slices"

	"storj.io/common/storj"
)

var unvettedSelectorTask = mon.Task()
var unvettedSelectorSelectionTask = mon.Task()

// UnvettedSelector selects new nodes first based on newNodeFraction, and selects old nodes for the remaining.
func UnvettedSelector(newNodeFraction float64, init NodeSelectorInit) NodeSelectorInit {
	return func(ctx context.Context, nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		defer unvettedSelectorTask(&ctx)(nil)

		var newNodes []*SelectedNode
		var oldNodes []*SelectedNode
		for _, node := range nodes {
			if node.Vetted {
				oldNodes = append(oldNodes, node)
			} else {
				newNodes = append(newNodes, node)
			}
		}

		newSelector := init(ctx, newNodes, filter)
		oldSelector := init(ctx, oldNodes, filter)

		// in case we have lots of old nodes, and just a few new nodes, we shouldn't overuse the new nodes, but use the natural fraction.
		actualFraction := newNodeFraction
		totalAvailable := len(newNodes) + len(oldNodes)
		if totalAvailable > 0 {
			actualAvailableFraction := float64(len(newNodes)) / float64(totalAvailable)
			if actualAvailableFraction < newNodeFraction {
				actualFraction = actualAvailableFraction
			}
		}

		return func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (_ []*SelectedNode, err error) {
			defer unvettedSelectorSelectionTask(&ctx)(&err)

			if math.IsNaN(newNodeFraction) || newNodeFraction <= 0 {
				return oldSelector(ctx, requester, n, excluded, alreadySelected)
			}

			var newNodeCount int
			if r := float64(n) * actualFraction; r < 1 {
				// Don't select any unvetted node.
				// Add 1 to random result to return 100 if the random function returns 99 and avoid to
				// always fail this condition if r is greater or equal than 0.99.
				if int(r*100) > (rand.Intn(100) + 1) {
					return oldSelector(ctx, requester, n, excluded, alreadySelected)
				}

				// Select one unvetted node.
				newNodeCount = 1

			} else {
				// Truncate to select the whole number part of unvetted nodes.
				newNodeCount = int(r)
			}

			selectedNewNodes, err := newSelector(ctx, requester, newNodeCount, excluded, alreadySelected)
			if err != nil {
				return selectedNewNodes, err
			}

			remaining := n - len(selectedNewNodes)
			selectedOldNodes, err := oldSelector(ctx, requester, remaining, excluded, alreadySelected)
			if err != nil {
				return selectedNewNodes, err
			}
			return append(selectedOldNodes, selectedNewNodes...), nil
		}
	}
}

var filteredSelectorTask = mon.Task()

// FilteredSelector uses another selector on the filtered list of nodes.
func FilteredSelector(preFilter NodeFilter, init NodeSelectorInit) NodeSelectorInit {
	return func(ctx context.Context, nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		defer filteredSelectorTask(&ctx)(nil)
		var filteredNodes []*SelectedNode
		for _, node := range nodes {
			if preFilter.Match(node) {
				filteredNodes = append(filteredNodes, node)
			}
		}
		return init(ctx, filteredNodes, filter)
	}
}

var attributeGroupSelectorTask = mon.Task()
var attributeGroupSelectorSelectionTask = mon.Task()

// AttributeGroupSelector first selects a group with equal chance (like last_net) and choose node from the group randomly.
func AttributeGroupSelector(attribute NodeAttribute) NodeSelectorInit {
	return func(ctx context.Context, nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		defer attributeGroupSelectorTask(&ctx)(nil)
		nodeByAttribute := make(map[string][]*SelectedNode)
		for _, node := range nodes {
			if filter != nil && !filter.Match(node) {
				continue
			}
			a := attribute(*node)
			nodeByAttribute[a] = append(nodeByAttribute[a], node)
		}

		var attributes []string
		for k := range nodeByAttribute {
			attributes = append(attributes, k)
		}

		return func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
			defer attributeGroupSelectorSelectionTask(&ctx)(&err)
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

var randomSelectorTask = mon.Task()
var randomSelectorSelectionTask = mon.Task()

// RandomSelector selects any nodes with equal chance.
func RandomSelector() NodeSelectorInit {
	return func(ctx context.Context, nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		defer randomSelectorTask(&ctx)(nil)

		var filteredNodes []*SelectedNode
		for _, node := range nodes {
			if filter != nil && !filter.Match(node) {
				continue
			}
			filteredNodes = append(filteredNodes, node)
		}

		return func(ctx context.Context, id storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
			defer randomSelectorSelectionTask(&ctx)(&err)
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

var filterSelectorTask = mon.Task()

// FilterSelector is a specific selector, which can filter out nodes from the upload selection.
// Note: this is different from the generic filter attribute of the NodeSelectorInit, as that is applied to all node selection (upload/download/repair).
func FilterSelector(loadTimeFilter NodeFilter, init NodeSelectorInit) NodeSelectorInit {
	return func(ctx context.Context, nodes []*SelectedNode, selectionFilter NodeFilter) NodeSelector {
		defer filterSelectorTask(&ctx)(nil)
		var filtered []*SelectedNode
		for _, n := range nodes {
			if loadTimeFilter.Match(n) {
				filtered = append(filtered, n)
			}
		}
		return init(ctx, filtered, selectionFilter)
	}
}

var balancedGroupBasedSelectorTask = mon.Task()
var balancedGroupBasedSelectorSelectionTask = mon.Task()

// BalancedGroupBasedSelector first selects a group with equal chance (like last_net) and choose one single node randomly. .
// One group can be tried multiple times, and if the node is already selected, it will be ignored.
func BalancedGroupBasedSelector(attribute NodeAttribute, uploadFilter NodeFilter) NodeSelectorInit {
	return func(ctx context.Context, nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		defer balancedGroupBasedSelectorTask(&ctx)(nil)

		mon.IntVal("selector_balanced_input_nodes").Observe(int64(len(nodes)))
		nodeByAttribute := make(map[string][]*SelectedNode)
		for _, node := range nodes {
			if filter != nil && !filter.Match(node) {
				continue
			}

			if uploadFilter != nil && !uploadFilter.Match(node) {
				continue
			}
			a := attribute(*node)
			nodeByAttribute[a] = append(nodeByAttribute[a], node)

		}

		var groupedNodes [][]*SelectedNode
		count := 0
		for _, nodeList := range nodeByAttribute {
			groupedNodes = append(groupedNodes, nodeList)
			count += len(nodeList)
		}
		mon.IntVal("selector_balanced_filtered_nodes").Observe(int64(count))
		return func(ctx context.Context, id storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
			defer balancedGroupBasedSelectorSelectionTask(&ctx)(&err)

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
func ChoiceOfTwo(comparer CompareNodes, delegate NodeSelectorInit) NodeSelectorInit {
	return ChoiceOfN(comparer, 2, delegate)
}

// Compare creates a CompareNodes function from multiple ScoreNode parameters.
// It compares nodes by evaluating each score in order and using the first non-equal result.
// If all scores are equal, it returns 0.
func Compare(scoreNodes ...ScoreNode) CompareNodes {
	return func(uplink storj.NodeID) func(node1 *SelectedNode, node2 *SelectedNode) int {
		scoreFuncs := make([]func(*SelectedNode) float64, len(scoreNodes))
		for i, scoreNode := range scoreNodes {
			scoreFuncs[i] = scoreNode.Get(uplink)
		}

		return func(node1 *SelectedNode, node2 *SelectedNode) int {
			for _, scoreFunc := range scoreFuncs {
				s1 := scoreFunc(node1)
				s2 := scoreFunc(node2)

				s1NaN := math.IsNaN(s1)
				s2NaN := math.IsNaN(s2)

				if s1NaN && s2NaN {
					continue
				}
				if s1NaN {
					return 1
				}
				if s2NaN {
					return -1
				}

				if s1 > s2 {
					return 1
				}
				if s2 > s1 {
					return -1
				}
				// Equal numbers, continue to next score
			}
			return 0
		}
	}
}

var choiceOfNReductionTask = mon.Task()

// choiceOfNBetter looks at two scores and returns true if the first argument
// is "better" than the second one
func choiceOfNBetter(score1, score2 float64) bool {
	// score1 and score2 could both potentially be NaN. we want to prefer a node if
	// it is NaN and if they are both NaN then it does not matter which we prefer (assuming the
	// input list is randomly shuffled). note that ALL comparisons where one of the
	// operands is NaN evaluate to false. thus for the following if statement, we have
	// the following table:
	//
	//     score1 | score2 | result
	//    --------|--------|-------
	//        NaN |    NaN | node2
	//        NaN | number | node1
	//     number |    NaN | node2
	//     number | number | whoever is larger
	if math.IsNaN(score2) || score2 > score1 {
		// score1 is not better.
		return false
	}
	// score1 is better.
	return true
}

func choiceOfNReduction(ctx context.Context, compare func(*SelectedNode, *SelectedNode) int, n int, nodes []*SelectedNode, desired int) []*SelectedNode {
	defer choiceOfNReductionTask(&ctx)(nil)
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
			if compare(nodes[0], nodes[1]) > 0 {
				// nodes[0] is selected
				nodes[1] = nodes[0] // place the winner (nodes[0]) into nodes[1] slot
				nodes = nodes[1:]   // discard the original nodes[0] slot, effectively keeping the winner
			} else {
				nodes = nodes[1:] // discard nodes[0], effectively keeping nodes[1]
			}
			toChooseBetween--
		}

		// move the selected node to the back
		nodes = append(nodes[1:], nodes[0])
	}
	return nodes
}

var (
	choiceOfNTask                                  = mon.Task()
	choiceOfNInternalSelectionTask                 = mon.Task()
	choiceOfNInternalSelectionExcludedCount        = mon.IntVal("choice-of-n-excluded-count")
	choiceOfNInternalSelectionAlreadySelectedCount = mon.IntVal("choice-of-n-already-selected-count")
)

// ChoiceOfN will perform the selection for n*x nodes and choose the best node
// from groups of n size. n is an int64 type due to a mito scripting shortcoming
// but really an int16 should be fine.
// NOTE: it may break other pre-conditions, like the results of the balanced selector...
func ChoiceOfN(comparison CompareNodes, n int64, delegate NodeSelectorInit) NodeSelectorInit {
	return func(ctx context.Context, allNodes []*SelectedNode, filter NodeFilter) NodeSelector {
		defer choiceOfNTask(&ctx)(nil)
		selector := delegate(ctx, allNodes, filter)
		return func(ctx context.Context, requester storj.NodeID, m int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
			defer choiceOfNInternalSelectionTask(&ctx)(&err)
			choiceOfNInternalSelectionExcludedCount.Observe(int64(len(excluded)))
			choiceOfNInternalSelectionAlreadySelectedCount.Observe(int64(len(alreadySelected)))
			nodes, err := selector(ctx, requester, int(n)*m, excluded, alreadySelected)
			if err != nil {
				return nil, err
			}
			// okay okay, let me talk about cardinality here - n is the value that
			// is choice of n. it is configured at process start time, so, say, 3.
			// n is likely some low number like 2 or 3 practically always.
			// m is the reed solomon number we're trying to select. this also has
			// very low cardinality. we have a handful of different rs settings
			// across all the different products, maybe 3 or 4 active ones. so the
			// result, n*m, has low cardinality.
			mon.IntVal(fmt.Sprintf("choice-of-n-requested-%d-got", int(n)*m)).Observe(int64(len(nodes)))

			return choiceOfNReduction(ctx, comparison(requester), int(n), nodes, m), nil
		}
	}
}

// ScoreSelection can help to choose between two selections with assigning a score. The higher score is better.
type ScoreSelection func(uplink storj.NodeID, selected []*SelectedNode) float64

// ScoreNode can help to assign a score to a node. The higher score is better. float.Nan is valid, if no information is available.
type ScoreNode interface {
	Get(uplink storj.NodeID) func(node *SelectedNode) float64
}

// CompareNodes compare two nodes for a specific client (uplink). Returns < 0 if node2 is better, returns > 0 if node1 is better, and returns 0 if both are equal.
type CompareNodes func(uplink storj.NodeID) func(node1 *SelectedNode, node2 *SelectedNode) int

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
			a := attr(*node)
			attributes = append(attributes, a)
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

var choiceOfNSelectionTask = mon.Task()
var choiceOfNSelectionSelectionTask = mon.Task()

// ChoiceOfNSelection is similar to ChoiceOfN, but doesn't break the pre-conditions of the original selector.
// it chooses from selections, without mixing nodes. scoreSources are ar judging the selections in order.
func ChoiceOfNSelection(n int64, delegate NodeSelectorInit, scoreSource ...ScoreSelection) NodeSelectorInit {
	return func(ctx context.Context, allNodes []*SelectedNode, filter NodeFilter) NodeSelector {
		defer choiceOfNSelectionTask(&ctx)(nil)
		selector := delegate(ctx, allNodes, filter)
		return func(ctx context.Context, requester storj.NodeID, m int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (selected []*SelectedNode, err error) {
			defer choiceOfNSelectionSelectionTask(&ctx)(&err)
			var bestSelection []*SelectedNode
			var bestScores []float64

			for i := 0; i < int(n); i++ {
				nodes, err := selector(ctx, requester, m, excluded, alreadySelected)
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

var filterBestTask = mon.Task()

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

	return func(ctx context.Context, nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		defer filterBestTask(&ctx)(nil)
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
		return delegate(ctx, nodes, filter)
	}
}

var bestOfNTask = mon.Task()
var bestOfNSelectionTask = mon.Task()

// BestOfN selects more nodes than the required one, and choose the fastest from those.
func BestOfN(tracker ScoreNode, ratio float64, delegate NodeSelectorInit) NodeSelectorInit {
	return func(ctx context.Context, nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		defer bestOfNTask(&ctx)(nil)

		wrappedSelector := delegate(ctx, nodes, filter)
		return func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (_ []*SelectedNode, err error) {
			defer bestOfNSelectionTask(&ctx)(&err)
			getSuccessRate := tracker.Get(requester)

			nodesToSelect := int(ratio * float64(n))
			selectedNodes, err := wrappedSelector(ctx, requester, nodesToSelect, excluded, alreadySelected)
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

var enoughFastTask = mon.Task()
var enoughFastSelectionTask = mon.Task()

// EnoughFast will select `ratio` times more nodes. The fastest nodes (under splitLine) will be used the selectionRation nodes, remaining wil be chosen from the second part.
func EnoughFast(tracker UploadSuccessTracker, ratio float64, splitLine float64, selectionRatio float64, delegate NodeSelectorInit) NodeSelectorInit {
	return func(ctx context.Context, nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		defer enoughFastTask(&ctx)(nil)
		wrappedSelector := delegate(ctx, nodes, filter)
		return func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (_ []*SelectedNode, err error) {
			defer enoughFastSelectionTask(&ctx)(&err)
			getSuccessRate := tracker.Get(requester)

			nodesToSelect := int(ratio * float64(n))
			selectedNodes, err := wrappedSelector(ctx, requester, nodesToSelect, excluded, alreadySelected)
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

var dualSelectorTask = mon.Task()
var dualSelectorSelectionTask = mon.Task()

// DualSelector selects fraction of nodes with first, and remaining with the second selector.
func DualSelector(fraction float64, first NodeSelectorInit, second NodeSelectorInit) NodeSelectorInit {
	return func(ctx context.Context, nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		defer dualSelectorTask(&ctx)(nil)
		if math.IsNaN(fraction) || fraction < 0 || fraction > 1 {
			panic("fraction is of the dual selector is invalid")
		}
		firstSelector := first(ctx, nodes, filter)
		secondSelector := second(ctx, nodes, filter)
		return func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (_ []*SelectedNode, err error) {
			defer dualSelectorSelectionTask(&ctx)(&err)

			firstSelectionCount := RoundWithProbability(float64(n) * fraction)

			var selectedFirstNodes []*SelectedNode
			if firstSelectionCount > 0 {
				selectedFirstNodes, err = firstSelector(ctx, requester, firstSelectionCount, excluded, alreadySelected)
				if err != nil {
					mon.Counter("dual_selector_failure").Inc(1)
				}
			}

			remaining := n - len(selectedFirstNodes)
			selectedSecondNodes, err := secondSelector(ctx, requester, remaining, excluded, append(alreadySelected, selectedFirstNodes...))
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

var weightedSelectorTask = mon.Task()
var weightedSelectorSelectionTask = mon.Task()

// WeightedSelector selects randomly from nodes, but supporting custom probabilities.
// Each node value is raised to the valuePower power, and then added to
// valueBallast.
// Some nodes can be selected more often than others.
// The implementation is based on Walker's alias method: https://www.youtube.com/watch?v=retAwpUv42E
func WeightedSelector(weightFunc NodeValue, initFilter NodeFilter) NodeSelectorInit {
	return func(ctx context.Context, nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		defer weightedSelectorTask(&ctx)(nil)
		var filtered []*SelectedNode
		for _, node := range nodes {
			if filter != nil && !filter.Match(node) {
				continue
			}

			if initFilter != nil && !initFilter.Match(node) {
				continue
			}
			filtered = append(filtered, node)
		}

		n := len(filtered)

		normalized := make([]float64, n)
		total := float64(0)
		for ix, node := range filtered {
			nodeValue := weightFunc(*node)
			total += nodeValue
			normalized[ix] = nodeValue
		}

		// in case of all value is zero, we need to select nodes with the same chance
		// it's safe to use 1, instead of all values --> total will be len(filtered)
		if total == 0 {
			total = float64(len(filtered))
		}

		for ix := range filtered {
			normalized[ix] = normalized[ix] / total * float64(n)
		}

		threshold := float64(1)
		// initialize the buckets
		var underfull []int
		var overfull []int
		for ix := range filtered {
			if normalized[ix] < threshold {
				underfull = append(underfull, ix)
			} else {
				overfull = append(overfull, ix)
			}
		}

		alias := make([]int, n)
		// pour the overfull buckets into the underfull buckets
		for len(underfull) > 0 && len(overfull) > 0 {
			// select one is above and one with under
			uf := underfull[0]
			of := overfull[0]
			underfull = underfull[1:]
			overfull = overfull[1:]

			alias[uf] = of
			normalized[of] -= threshold - normalized[uf]

			if normalized[of] < threshold {
				underfull = append(underfull, of)
			} else if normalized[of] > threshold {
				overfull = append(overfull, of)
			}
		}
		return func(ctx context.Context, requester storj.NodeID, selectN int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (_ []*SelectedNode, err error) {
			defer weightedSelectorSelectionTask(&ctx)(&err)
			var selected []*SelectedNode
			if n == 0 {
				return selected, nil
			}
			for i := 0; i < selectN*5; i++ {
				r := rand.Intn(n)
				var selectedNode *SelectedNode
				if normalized[r] > rand.Float64() {
					selectedNode = filtered[r]
				} else {
					selectedNode = filtered[alias[r]]
				}
				if includedInNodes(alreadySelected, selectedNode) || included(excluded, selectedNode) || includedInNodes(selected, selectedNode) {
					continue
				}
				selected = append(selected, selectedNode)
				if len(selected) == selectN {
					break
				}
			}
			return selected, nil
		}
	}
}

// NeedMore is a stateful function, which returns true, if more nodes are needed.
type NeedMore func() func(node *SelectedNode) bool

// AtLeast is a needMore function, which will return true, if the number groups (specified by the given attribute) is less than the minimum value.
func AtLeast(attribute NodeAttribute, min interface{}) NeedMore {
	return func() func(node *SelectedNode) bool {
		var minv int64
		switch m := min.(type) {
		case int64:
			minv = m
		case int:
			minv = int64(m)
		case func() int64:
			minv = m()
		default:
			panic("min value for atleast must be int64, int or func()int64")
		}
		current := map[string]int64{}
		return func(node *SelectedNode) bool {
			current[attribute(*node)]++
			return int64(len(current)) < minv
		}
	}
}

// Reduce is a NodeSelectorInit, which will reduce the number of nodes selected by the delegate.
func Reduce(delegate NodeSelectorInit, sortOrder CompareNodes, needMoreChecks ...NeedMore) NodeSelectorInit {
	return func(ctx context.Context, nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		var checks []func(node *SelectedNode) bool
		for _, inv := range needMoreChecks {
			// some checks may use current time (like daily), we should evaluate during each init (called during cache refresh)
			checks = append(checks, inv())
		}

		if sortOrder != nil {
			slices.SortFunc(nodes, sortOrder(storj.NodeID{}))
		}
		var filtered []*SelectedNode
		for _, node := range nodes {
			if filter != nil && !filter.Match(node) {
				continue
			}

			filtered = append(filtered, node)

			needMore := false
			for _, check := range checks {
				needMore = needMore || check(node)
			}
			if !needMore {
				break
			}
		}
		return delegate(ctx, filtered, filter)
	}
}

// DailyPeriods returns a function, which returns the value of the period in days based on the current hour.
func DailyPeriods(values ...int64) func() int64 {
	return func() int64 {
		return DailyPeriodsForHour(time.Now().UTC().Hour(), values)
	}
}

// DailyPeriodsForHour returns the value of the period in days based on the given hour.
func DailyPeriodsForHour(hour int, values []int64) int64 {
	adjustedIndex := hour * len(values) / 24.0
	return values[adjustedIndex]
}

// MultiSelector can combine multiple selectors. It will call each selector in order, and combine the results.
func MultiSelector(selectors ...NodeSelectorInit) NodeSelectorInit {
	return func(ctx context.Context, nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		var all []NodeSelector
		for _, delegate := range selectors {
			all = append(all, delegate(ctx, nodes, filter))
		}
		return func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (nodes []*SelectedNode, err error) {
			for _, delegate := range all {
				selectedNodes, selectErr := delegate(ctx, requester, n/len(selectors), excluded, alreadySelected)
				if err != nil {
					err = errs.Combine(err, selectErr)
				}
				nodes = append(nodes, selectedNodes...)
			}
			return nodes, err
		}
	}
}

// FixedSelector selector can override the number of the required nodes, and select more (or less).
func FixedSelector(fixed int64, delegate NodeSelectorInit) NodeSelectorInit {
	return func(ctx context.Context, nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		selector := delegate(ctx, nodes, filter)
		return func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (nodes []*SelectedNode, err error) {
			return selector(ctx, requester, int(fixed), excluded, alreadySelected)
		}
	}
}
