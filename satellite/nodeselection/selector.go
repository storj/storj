// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	mathrand "math/rand" // Using mathrand here because crypto-graphic randomness is not required and simplifies code.
)

// SelectByID implements selection from nodes with every node having equal probability.
type SelectByID []*SelectedNode

var _ Selector = (SelectByID)(nil)

// Count returns the number of maximum number of nodes that it can return.
func (nodes SelectByID) Count() int { return len(nodes) }

// Select selects upto n nodes.
func (nodes SelectByID) Select(n int, nodeFilter NodeFilter) []*SelectedNode {
	if n <= 0 {
		return nil
	}

	selected := []*SelectedNode{}
	for _, idx := range mathrand.Perm(len(nodes)) {
		node := nodes[idx]

		if !nodeFilter.Match(node) {
			continue
		}

		selected = append(selected, node.Clone())
		if len(selected) >= n {
			break
		}
	}

	return selected
}

// SelectBySubnet implements selection from nodes with every subnet having equal probability.
type SelectBySubnet []Subnet

var _ Selector = (SelectBySubnet)(nil)

// Subnet groups together nodes with the same subnet.
type Subnet struct {
	Net   string
	Nodes []*SelectedNode
}

// SelectBySubnetFromNodes creates SelectBySubnet selector from nodes.
func SelectBySubnetFromNodes(nodes []*SelectedNode) SelectBySubnet {
	bynet := map[string][]*SelectedNode{}
	for _, node := range nodes {
		bynet[node.LastNet] = append(bynet[node.LastNet], node)
	}

	var subnets SelectBySubnet
	for net, nodes := range bynet {
		subnets = append(subnets, Subnet{
			Net:   net,
			Nodes: nodes,
		})
	}

	return subnets
}

// Count returns the number of maximum number of nodes that it can return.
func (subnets SelectBySubnet) Count() int { return len(subnets) }

// Select selects upto n nodes.
func (subnets SelectBySubnet) Select(n int, filter NodeFilter) []*SelectedNode {
	if n <= 0 {
		return nil
	}

	selected := []*SelectedNode{}
	r := NewRandomOrder(len(subnets))
	for r.Next() {
		subnet := subnets[r.At()]

		rs := NewRandomOrder(len(subnet.Nodes))
		for rs.Next() {
			if filter.Match(subnet.Nodes[rs.At()]) {
				selected = append(selected, subnet.Nodes[rs.At()].Clone())
				break
			}
		}
		if len(selected) >= n {
			break
		}
	}

	return selected
}
