// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/zeebo/mwc"

	"storj.io/common/storj"
)

var topologySelectorTask = mon.Task()
var topologySelectorSelectionTask = mon.Task()

// TopologySelector selects nodes using weights and topology structure. Topology is a tree and defined by attributes. (eg. datacenter / server / instance).
// Number of selected nodes should be defined for each level.
// It works well, if the number of elements on each level are higher than the requested selection.
// TODO: existing selection nodes are not used to restrict selection. Repair may use groups too many times.
func TopologySelector(weightFunc NodeValue, groups string, selections string, initFilter NodeFilter) NodeSelectorInit {

	var selectionPattern []int
	for _, pattern := range strings.Split(groups, ",") {
		n, err := strconv.Atoi(pattern)
		if err != nil {
			panic("Topology selector: invalid group pattern. Must be comma separated integers.")
		}
		selectionPattern = append(selectionPattern, n)
	}

	var attributes []NodeAttribute
	for _, attributeName := range strings.Split(selections, ",") {
		a, err := CreateNodeAttribute(attributeName)
		if err != nil {
			panic("Topology selector: invalid selection pattern. Must be comma separated node attributes.")
		}
		attributes = append(attributes, a)
	}

	return func(ctx context.Context, nodes []*SelectedNode, filter NodeFilter) NodeSelector {
		defer topologySelectorTask(&ctx)(nil)

		root := &Nodes{}
		for _, node := range nodes {
			if filter != nil && !filter.Match(node) {
				continue
			}

			if initFilter != nil && !initFilter.Match(node) {
				continue
			}
			root.Add(node, attributes, weightFunc(*node))
		}

		return func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (_ []*SelectedNode, err error) {
			defer topologySelectorSelectionTask(&ctx)(&err)
			selection := root.Select(selectionPattern, n, excluded)
			if len(selection) > n {
				selection = selection[:n]
			}
			return selection, nil
		}
	}
}

// Nodes is a tree structure to store nodes and their attributes.
// Example: first level of the tree contains groups for tag:datacenter, second level contains groups for tag:servers, third level contains nodes.
// Selection on each level are predefined (example: select 3 datacenters, 2 servers from each datacenter, 1 node from each server).
// Selection is based on weight.
type Nodes struct {
	Name         string
	Groups       []*Nodes // if len(Groups) > 0, len(Nodes) == 0 and vice versa.
	Nodes        []*SelectedNode
	NodeSelector NodeSelector // nil if len(nodes) == 0
	Random       WeightedRandom
}

// Add adds a node to the tree. Based on the attributes (like datacenter,server,...) we build a tree.
// Note: weights are cumulative, on datacenter level, the weight is the sum of all the servers weights.
func (n *Nodes) Add(node *SelectedNode, attributes []NodeAttribute, weight float64) {

	// leaf, only nodes
	if len(attributes) == 0 {
		n.Nodes = append(n.Nodes, node)
		n.Random = append(n.Random, weight)
		return
	}
	attr := attributes[0](*node)

	// find the right group for the attribute
	ix := -1
	for i, g := range n.Groups {
		if g.Name == attr {
			ix = i
		}
	}

	// no such group, yet
	if ix == -1 {
		n.Groups = append(n.Groups, &Nodes{
			Name: attr,
		})
		n.Random = append(n.Random, float64(0))
		ix = len(n.Groups) - 1
	}

	n.Groups[ix].Add(node, attributes[1:], weight)
	n.Random[ix] += weight

}

// Select selects nodes based on the selection pattern. parameter defines the desired number of groups from each level.
// Number of selected nodes will be the multiplication of all the nodes.
func (n *Nodes) Select(splits []int, m int, excluded []storj.NodeID) (selection []*SelectedNode) {
	// in case of leaf nodes, we have the instances, and we select based on the weights. No more sub-groups here.
	if len(n.Groups) == 0 {
		selectedIx := n.Random.Random(m, collectIndexes(n.Nodes, excluded))
		for _, ix := range selectedIx {
			selection = append(selection, n.Nodes[ix])
		}
		return selection
	}

	// we select the groups based on the weights
	selectedIx := n.Random.Random(splits[0], []int{})
	effectiveSplits := len(selectedIx) // either the requested split, or less if we don't have so many groups

	// from each selected group, we select the nodes
	for group, ix := range selectedIx {

		// the remaining required is divided by the remaining groups
		amount := (m - len(selection)) / (effectiveSplits - group)

		selection = append(selection, n.Groups[ix].Select(splits[1:], amount, excluded)...)
	}

	return selection
}

func collectIndexes(nodes []*SelectedNode, ids []storj.NodeID) []int {
	var indexes []int
	for ix, node := range nodes {
		for i := range ids {
			if ids[i] == node.ID {
				indexes = append(indexes, ix)
				break
			}
		}
	}
	return indexes
}

// WeightedItem is a helper struct for WeightedRandom. Includes the original index and randomized Score.
type WeightedItem struct {
	Index int
	Score float64
}

// WeightedRandom provides random selection based on Efraimidis-Spirakis algorithm.
// see: http://utopia.duth.gr/~pefraimi/research/data/2007EncOfAlg.pdf
type WeightedRandom []float64

// Random returns k random indexes based on weights.
func (w WeightedRandom) Random(k int, exclusion []int) []int {
	n := len(w)
	if k > n {
		k = n
	}

	rng := mwc.Rand()

	items := make([]WeightedItem, n)
	for i, weight := range w {

		r := rng.Float64()
		// Handle edge case of zero weight
		if weight <= 0 || excluded(exclusion, i) {
			items[i] = WeightedItem{Index: i, Score: math.Inf(-1)}
			continue
		}

		key := math.Pow(r, 1.0/weight)
		items[i] = WeightedItem{Index: i, Score: key}
	}

	// find the k largest keys. sequential search for max k times.
	for i := 0; i < k; i++ {
		maxIdx := i
		for j := i + 1; j < n; j++ {
			if items[j].Score > items[maxIdx].Score {
				maxIdx = j
			}
		}

		// swap max if we found it
		if maxIdx != i {
			items[i], items[maxIdx] = items[maxIdx], items[i]
		}
	}

	// collect indexes of k largest elements
	result := make([]int, k)
	for i := 0; i < k; i++ {
		result[i] = items[i].Index
	}
	return result
}

func excluded(exclusion []int, i int) bool {
	for _, e := range exclusion {
		if e == i {
			return true
		}
	}
	return false
}
