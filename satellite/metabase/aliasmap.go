// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"encoding/binary"

	"storj.io/common/storj"
)

// NodeAliasMap contains bidirectional mapping between node ID and a NodeAlias.
//
// The node ID to NodeAlias lookup is implemented as a map of 4-byte node ID
// prefixes to a linked list of node ID/alias pairs, so that the whole ID
// does not need to be hashed with each lookup.
type NodeAliasMap struct {
	node  []storj.NodeID
	alias map[uint32]*nodeAliasChain
}

// nodeAliasChain implements linked list on NodeAliasEntry.
type nodeAliasChain struct {
	NodeAliasEntry
	Tail *nodeAliasChain
}

// Include adds the entry to the current chain, if it already doesn't exist.
func (chain *nodeAliasChain) Include(id storj.NodeID, alias NodeAlias) {
	for {
		if chain.ID == id {
			return
		}

		if chain.Tail == nil {
			chain.Tail = &nodeAliasChain{
				NodeAliasEntry: NodeAliasEntry{ID: id, Alias: alias},
			}
			return
		}

		chain = chain.Tail
	}
}

// Clone clones the chain.
func (chain *nodeAliasChain) Clone() *nodeAliasChain {
	if chain == nil {
		return nil
	}
	return &nodeAliasChain{
		NodeAliasEntry: chain.NodeAliasEntry,
		Tail:           chain.Tail.Clone(),
	}
}

// Count counts the number of entries in this chain.
func (chain *nodeAliasChain) Count() (count int) {
	for ; chain != nil; chain = chain.Tail {
		count++
	}
	return count
}

// mergeNodeAliasChains merges a and b and doesn't include duplicates.
func mergeNodeAliasChains(a, b *nodeAliasChain) *nodeAliasChain {
	r := a.Clone()
	if r == nil {
		return b.Clone()
	}

	for ; b != nil; b = b.Tail {
		r.Include(b.ID, b.Alias)
	}

	return r
}

// NewNodeAliasMap creates a new alias map from the given entries.
func NewNodeAliasMap(entries []NodeAliasEntry) *NodeAliasMap {
	m := &NodeAliasMap{
		node:  make([]storj.NodeID, len(entries)),
		alias: make(map[uint32]*nodeAliasChain, len(entries)),
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
func (m *NodeAliasMap) setAlias(value storj.NodeID, alias NodeAlias) {
	prefix := binary.LittleEndian.Uint32(value[:4])
	entry, ok := m.alias[prefix]
	if !ok {
		m.alias[prefix] = &nodeAliasChain{
			NodeAliasEntry: NodeAliasEntry{
				ID:    value,
				Alias: alias,
			},
		}
	} else {
		entry.Include(value, alias)
	}
}

// Merge merges the other map into m.
func (m *NodeAliasMap) Merge(other *NodeAliasMap) {
	for k, v := range other.node {
		if !v.IsZero() {
			m.setNode(NodeAlias(k), v)
		}
	}

	for k, v := range other.alias {
		m.alias[k] = mergeNodeAliasChains(m.alias[k], v)
	}
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
	prefix := binary.LittleEndian.Uint32(node[:4])

	chain := m.alias[prefix]
	for ; chain != nil; chain = chain.Tail {
		if ([12]byte)(chain.ID[4:]) == ([12]byte)(node[4:]) {
			return chain.Alias, true
		}
	}

	return -1, false
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

	count := 0
	for _, c := range m.alias {
		count += c.Count()
	}

	return count
}

// Max returns the largest node alias in this map, -1 otherwise. Contrast with Size.
func (m *NodeAliasMap) Max() NodeAlias {
	return NodeAlias(len(m.node) - 1)
}
