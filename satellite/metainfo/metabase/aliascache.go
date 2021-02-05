// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"sync"

	"storj.io/common/storj"
)

// NodeAliasDB is an interface for looking up node alises.
type NodeAliasDB interface {
	EnsureNodeAliases(ctx context.Context, opts EnsureNodeAliases) error
	ListNodeAliases(ctx context.Context) (_ []NodeAliasEntry, err error)
}

// NodeAliasCache is a write-through cache for looking up node ID and alias mapping.
type NodeAliasCache struct {
	mu     sync.Mutex
	latest *NodeAliasMap
	db     NodeAliasDB
}

// NewNodeAliasCache creates a new cache using the specified database.
func NewNodeAliasCache(db NodeAliasDB) *NodeAliasCache {
	return &NodeAliasCache{
		db:     db,
		latest: NewNodeAliasMap(nil),
	}
}

// Nodes returns node ID-s corresponding to the aliases,
// refreshing the cache once when an alias is missing.
// This results in an error when the alias is not in the database.
func (cache *NodeAliasCache) Nodes(ctx context.Context, aliases []NodeAlias) ([]storj.NodeID, error) {
	cache.mu.Lock()
	latest := cache.latest
	cache.mu.Unlock()

	nodes, missing := latest.Nodes(aliases)
	if len(missing) == 0 {
		return nodes, nil
	}

	if len(missing) > 0 {
		var err error
		latest, err = cache.refresh(ctx)
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
	cache.mu.Lock()
	latest := cache.latest
	cache.mu.Unlock()

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
	return cache.refresh(ctx)
}

// refresh refreshses the state of the cache.
func (cache *NodeAliasCache) refresh(ctx context.Context) (_ *NodeAliasMap, err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: allow only one inflight request

	entries, err := cache.db.ListNodeAliases(ctx)
	if err != nil {
		return nil, err
	}

	xs := NewNodeAliasMap(entries)

	cache.mu.Lock()
	defer cache.mu.Unlock()

	// Since we never remove node aliases we can assume that the alias map that contains more
	// entries is the latest one.
	//
	// Note: we merge the maps here rather than directly replacing.
	// This is not ideal from performance side, however it should reduce possible consistency issues.
	xs.Merge(cache.latest)
	cache.latest = xs

	return cache.latest, nil
}

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

// Merge merges the other map into m.
func (m *NodeAliasMap) Merge(other *NodeAliasMap) {
	for k, v := range other.node {
		m.node[k] = v
	}
	for k, v := range other.alias {
		m.alias[k] = v
	}
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
