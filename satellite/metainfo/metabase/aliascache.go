// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"storj.io/common/storj"
)

// NodeAliasMap contains bidirectional mapping between node ID and a NodeAlias.
type NodeAliasMap struct {
	node  map[NodeAlias]storj.NodeID
	alias map[storj.NodeID]NodeAlias
}

// NewNodeAliasMap creates a new alias map from the given entries.
func NewNodeAliasMap(entries []NodeAliasEntry) *NodeAliasMap {
	m := &NodeAliasMap{
		node:  make(map[NodeAlias]storj.NodeID, len(entries)),
		alias: make(map[storj.NodeID]NodeAlias, len(entries)),
	}
	for _, e := range entries {
		m.node[e.Alias] = e.ID
		m.alias[e.ID] = e.Alias
	}
	return m
}

// Nodes returns NodeID-s for the given aliases and aliases that are not in this map.
func (m *NodeAliasMap) Nodes(aliases []NodeAlias) (xs []storj.NodeID, missing []NodeAlias) {
	xs = make([]storj.NodeID, 0, len(aliases))
	for _, p := range aliases {
		if x, ok := m.node[p]; ok {
			xs = append(xs, x)
		} else {
			missing = append(missing, p)
		}
	}
	return xs, missing
}

// Aliases returns alises-s for the given node ID-s and node ID-s that are not in this map.
func (m *NodeAliasMap) Aliases(nodes []storj.NodeID) (xs []NodeAlias, missing []storj.NodeID) {
	xs = make([]NodeAlias, 0, len(nodes))
	for _, n := range nodes {
		if x, ok := m.alias[n]; ok {
			xs = append(xs, x)
		} else {
			missing = append(missing, n)
		}
	}
	return xs, missing
}

// Size returns the number of entries in this map.
func (m *NodeAliasMap) Size() int {
	if m == nil {
		return 0
	}
	return len(m.node)
}
