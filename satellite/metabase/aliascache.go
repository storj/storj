// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"sync"
	"sync/atomic"

	"storj.io/common/storj"
)

// NodeAliasDB is an interface for looking up node alises.
type NodeAliasDB interface {
	EnsureNodeAliases(ctx context.Context, opts EnsureNodeAliases) error
	ListNodeAliases(ctx context.Context) (_ []NodeAliasEntry, err error)
}

// NodeAliasCache is a write-through cache for looking up node ID and alias mapping.
type NodeAliasCache struct {
	db         NodeAliasDB
	refreshing sync.Mutex
	latest     atomic.Value // *NodeAliasMap
}

// NewNodeAliasCache creates a new cache using the specified database.
func NewNodeAliasCache(db NodeAliasDB) *NodeAliasCache {
	cache := &NodeAliasCache{
		db: db,
	}
	cache.latest.Store(NewNodeAliasMap(nil))
	return cache
}

func (cache *NodeAliasCache) getLatest() *NodeAliasMap {
	return cache.latest.Load().(*NodeAliasMap)
}

// Nodes returns node ID-s corresponding to the aliases,
// refreshing the cache once when an alias is missing.
// This results in an error when the alias is not in the database.
func (cache *NodeAliasCache) Nodes(ctx context.Context, aliases []NodeAlias) ([]storj.NodeID, error) {
	latest := cache.getLatest()

	nodes, missing := latest.Nodes(aliases)
	if len(missing) == 0 {
		return nodes, nil
	}

	if len(missing) > 0 {
		var err error
		latest, err = cache.refresh(ctx, nil, missing)
		if err != nil {
			return nil, Error.New("failed to refresh node alias db: %w", err)
		}
	}

	nodes, missing = latest.Nodes(aliases)
	if len(missing) == 0 {
		return nodes, nil
	}

	return nil, Error.New("aliases missing in database: %v", missing)
}

// Aliases returns node aliases corresponding to the node ID-s,
// adding missing node ID-s to the database when needed.
func (cache *NodeAliasCache) Aliases(ctx context.Context, nodes []storj.NodeID) ([]NodeAlias, error) {
	latest := cache.getLatest()

	aliases, missing := latest.Aliases(nodes)
	if len(missing) == 0 {
		return aliases, nil
	}

	if len(missing) > 0 {
		var err error
		latest, err = cache.ensure(ctx, missing...)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	aliases, missing = latest.Aliases(nodes)
	if len(missing) == 0 {
		return aliases, nil
	}

	return nil, Error.New("nodes still missing after ensuring: %v", missing)
}

// ensure tries to ensure that the specified missing node ID-s are assigned a alias.
func (cache *NodeAliasCache) ensure(ctx context.Context, missing ...storj.NodeID) (_ *NodeAliasMap, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := cache.db.EnsureNodeAliases(ctx, EnsureNodeAliases{
		Nodes: missing,
	}); err != nil {
		return nil, Error.New("failed to update node alias db: %w", err)
	}
	return cache.refresh(ctx, missing, nil)
}

// refresh refreshses the state of the cache.
func (cache *NodeAliasCache) refresh(ctx context.Context, missingNodes []storj.NodeID, missingAliases []NodeAlias) (_ *NodeAliasMap, err error) {
	defer mon.Task()(&ctx)(&err)

	cache.refreshing.Lock()
	defer cache.refreshing.Unlock()

	latest := cache.getLatest()

	// Maybe some other goroutine already refreshed the list, double-check.
	if latest.ContainsAll(missingNodes, missingAliases) {
		return latest, nil
	}

	entries, err := cache.db.ListNodeAliases(ctx)
	if err != nil {
		return nil, err
	}

	// Since we never remove node aliases we can assume that the alias map that contains more
	// entries is the latest one.
	//
	// Note: we merge the maps here rather than directly replacing.
	// This is not ideal from performance side, however it should reduce possible consistency issues.

	xs := NewNodeAliasMap(entries)
	xs.Merge(latest)
	cache.latest.Store(xs)

	return xs, nil
}

// ConvertPiecesToAliases converts pieces to alias pieces.
func (cache *NodeAliasCache) ConvertPiecesToAliases(ctx context.Context, pieces Pieces) (_ AliasPieces, err error) {
	defer mon.Task()(&ctx)(&err)

	nodes := make([]storj.NodeID, len(pieces))
	for i, p := range pieces {
		nodes[i] = p.StorageNode
	}

	aliases, err := cache.Aliases(ctx, nodes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	aliasPieces := make(AliasPieces, len(aliases))
	for i, alias := range aliases {
		aliasPieces[i] = AliasPiece{
			Number: pieces[i].Number,
			Alias:  alias,
		}
	}

	return aliasPieces, nil
}

// ConvertAliasesToPieces converts alias pieces to pieces.
func (cache *NodeAliasCache) ConvertAliasesToPieces(ctx context.Context, aliasPieces AliasPieces) (_ Pieces, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(aliasPieces) == 0 {
		return Pieces{}, nil
	}

	latest := cache.getLatest()

	pieces := make(Pieces, len(aliasPieces))
	var missing []NodeAlias

	for i, aliasPiece := range aliasPieces {
		node, ok := latest.Node(aliasPiece.Alias)
		if !ok {
			missing = append(missing, aliasPiece.Alias)
			continue
		}
		pieces[i].Number = aliasPiece.Number
		pieces[i].StorageNode = node
	}

	if len(missing) > 0 {
		var err error
		latest, err = cache.refresh(ctx, nil, missing)
		if err != nil {
			return nil, Error.New("failed to refresh node alias db: %w", err)
		}

		for i, aliasPiece := range aliasPieces {
			node, ok := latest.Node(aliasPiece.Alias)
			if !ok {
				return nil, Error.New("aliases missing in database: %v", missing)
			}
			pieces[i].Number = aliasPiece.Number
			pieces[i].StorageNode = node
		}
	}

	return pieces, nil
}

// NodeAliasMap contains bidirectional mapping between node ID and a NodeAlias.
type NodeAliasMap struct {
	node  []storj.NodeID
	alias map[storj.NodeID]NodeAlias
}

// NewNodeAliasMap creates a new alias map from the given entries.
func NewNodeAliasMap(entries []NodeAliasEntry) *NodeAliasMap {
	m := &NodeAliasMap{
		node:  make([]storj.NodeID, len(entries)),
		alias: make(map[storj.NodeID]NodeAlias, len(entries)),
	}
	for _, e := range entries {
		m.setNode(e.Alias, e.ID)
		m.alias[e.ID] = e.Alias
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

// Merge merges the other map into m.
func (m *NodeAliasMap) Merge(other *NodeAliasMap) {
	for k, v := range other.node {
		if !v.IsZero() {
			m.setNode(NodeAlias(k), v)
		}
	}
	for k, v := range other.alias {
		m.alias[k] = v
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

// ContainsAll returns true when the table contains all entries.
func (m *NodeAliasMap) ContainsAll(nodeIDs []storj.NodeID, nodeAliases []NodeAlias) bool {
	for _, id := range nodeIDs {
		if _, ok := m.alias[id]; !ok {
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

// Size returns the number of entries in this map.
func (m *NodeAliasMap) Size() int {
	if m == nil {
		return 0
	}
	return len(m.alias)
}
