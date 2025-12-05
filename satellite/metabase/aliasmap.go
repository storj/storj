// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"storj.io/common/storj"
	"storj.io/storj/shared/nodeidmap"
)

// NodeAliasMap contains bidirectional mapping between node ID and a NodeAlias.
//
// The node ID to NodeAlias lookup is implemented as a map of 4-byte node ID
// prefixes to a linked list of node ID/alias pairs, so that the whole ID
// does not need to be hashed with each lookup.
type NodeAliasMap struct {
	node  []storj.NodeID
	alias nodeidmap.Map[NodeAlias]
}

// NewNodeAliasMap creates a new alias map from the given entries.
func NewNodeAliasMap(entries []NodeAliasEntry) *NodeAliasMap {
	m := &NodeAliasMap{
		node:  make([]storj.NodeID, len(entries)),
		alias: nodeidmap.MakeSized[NodeAlias](len(entries)),
	}
	for _, e := range entries {
		m.setNode(e.Alias, e.ID)
		m.setAlias(e.ID, e.Alias)
	}
	return m
}

// setNode sets a value in `m.node` and increases the size when necessary.
func (m *NodeAliasMap) setNode(alias NodeAlias, value storj.NodeID) {
	if int(alias) >= len(m.node) {
		m.node = append(m.node, make([]storj.NodeID, int(alias)-len(m.node)+1)...)
	}
	m.node[alias] = value
}

// setAlias sets a value in `m.alias`.
func (m *NodeAliasMap) setAlias(id storj.NodeID, alias NodeAlias) {
	m.alias.Store(id, alias)
}

// Merge merges the other map into m.
func (m *NodeAliasMap) Merge(other *NodeAliasMap) {
	for k, v := range other.node {
		if !v.IsZero() {
			m.setNode(NodeAlias(k), v)
		}
	}

	m.alias.Add(other.alias, func(_, new NodeAlias) NodeAlias {
		return new
	})
}

// Node returns NodeID for the given alias.
func (m *NodeAliasMap) Node(alias NodeAlias) (x storj.NodeID, ok bool) {
	if int(alias) >= len(m.node) {
		return storj.NodeID{}, false
	}
	v := m.node[alias]
	return v, !v.IsZero()
}

// Alias returns alias for the given node ID.
func (m *NodeAliasMap) Alias(node storj.NodeID) (x NodeAlias, ok bool) {
	x, ok = m.alias.Load(node)
	if !ok {
		return -1, false
	}
	return x, true
}

// Nodes returns NodeID-s for the given aliases and aliases that are not in this map.
func (m *NodeAliasMap) Nodes(aliases []NodeAlias) (xs []storj.NodeID, missing []NodeAlias) {
	xs = make([]storj.NodeID, 0, len(aliases))
	for _, p := range aliases {
		if x, ok := m.Node(p); ok {
			xs = append(xs, x)
		} else {
			missing = append(missing, p)
		}
	}
	return xs, missing
}

// Aliases returns aliases-s for the given node ID-s and node ID-s that are not in this map.
func (m *NodeAliasMap) Aliases(nodes []storj.NodeID) (xs []NodeAlias, missing []storj.NodeID) {
	xs = make([]NodeAlias, 0, len(nodes))
	for _, n := range nodes {
		if x, ok := m.Alias(n); ok {
			xs = append(xs, x)
		} else {
			missing = append(missing, n)
		}
	}
	return xs, missing
}

// ContainsAll returns true when the table contains all entries.
func (m *NodeAliasMap) ContainsAll(nodeIDs []storj.NodeID, nodeAliases []NodeAlias) bool {
	for _, id := range nodeIDs {
		if _, ok := m.Alias(id); !ok {
			return false
		}
	}
	for _, alias := range nodeAliases {
		if _, ok := m.Node(alias); !ok {
			return false
		}
	}
	return true
}

// Size returns the number of entries in this map. Contrast with Max.
func (m *NodeAliasMap) Size() int {
	if m == nil {
		return 0
	}

	return m.alias.Count()
}

// Max returns the largest node alias in this map, -1 otherwise. Contrast with Size.
func (m *NodeAliasMap) Max() NodeAlias {
	return NodeAlias(len(m.node) - 1)
}
