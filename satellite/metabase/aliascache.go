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
	GetNodeAliasEntries(ctx context.Context, opts GetNodeAliasEntries) (_ []NodeAliasEntry, err error)
}

// NodeAliasCache is a write-through cache for looking up node ID and alias mapping.
type NodeAliasCache struct {
	db         NodeAliasDB
	refreshing sync.Mutex
	latest     atomic.Value // *NodeAliasMap

	fullRefresh bool // TODO: remove once verified that it works better in production.
}

// NewNodeAliasCache creates a new cache using the specified database.
func NewNodeAliasCache(db NodeAliasDB, fullRefresh bool) *NodeAliasCache {
	cache := &NodeAliasCache{
		db:          db,
		fullRefresh: fullRefresh,
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

	var err error
	latest, err = cache.refresh(ctx, nil, missing)
	if err != nil {
		return nil, Error.New("failed to refresh node alias db: %w", err)
	}

	nodes, missing = latest.Nodes(aliases)
	if len(missing) == 0 {
		return nodes, nil
	}

	return nil, Error.New("aliases missing in database: %v", missing)
}

// EnsureAliases returns node aliases corresponding to the node ID-s,
// adding missing node ID-s to the database when needed.
func (cache *NodeAliasCache) EnsureAliases(ctx context.Context, nodes []storj.NodeID) ([]NodeAlias, error) {
	latest := cache.getLatest()

	aliases, missing := latest.Aliases(nodes)
	if len(missing) == 0 {
		return aliases, nil
	}

	var err error
	latest, err = cache.ensure(ctx, missing...)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	aliases, missing = latest.Aliases(nodes)
	if len(missing) == 0 {
		return aliases, nil
	}

	return nil, Error.New("nodes still missing after ensuring: %v", missing)
}

// Aliases returns node aliases corresponding to the node ID-s and returns an error when node is missing.
func (cache *NodeAliasCache) Aliases(ctx context.Context, nodes []storj.NodeID) ([]NodeAlias, error) {
	latest := cache.getLatest()

	aliases, missing := latest.Aliases(nodes)
	if len(missing) == 0 {
		return aliases, nil
	}

	var err error
	latest, err = cache.refresh(ctx, missing, nil)
	if err != nil {
		return nil, Error.New("failed to refresh node alias db: %w", err)
	}

	aliases, missing = latest.Aliases(nodes)
	if len(missing) > 0 {
		return aliases, Error.New("aliases missing for %v", missing)
	}

	return aliases, nil
}

// Latest returns the latest NodeAliasMap.
func (cache *NodeAliasCache) Latest(ctx context.Context) (_ *NodeAliasMap, err error) {
	defer mon.Task()(&ctx)(&err)

	latest, err := cache.refresh(ctx, nil, nil)
	if err != nil {
		return nil, Error.New("failed to refresh node alias db: %w", err)
	}

	xs := NewNodeAliasMap(nil)
	xs.Merge(latest)

	return xs, nil
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

// refresh refreshes the state of the cache, when missingNodes or missingAliases is still missing.
// When both are nil, then it always refreshes.
func (cache *NodeAliasCache) refresh(ctx context.Context, missingNodes []storj.NodeID, missingAliases []NodeAlias) (_ *NodeAliasMap, err error) {
	defer mon.Task()(&ctx)(&err)

	cache.refreshing.Lock()
	defer cache.refreshing.Unlock()

	latest := cache.getLatest()

	// Maybe some other goroutine already refreshed the list, double-check.
	if (len(missingNodes) > 0 || len(missingAliases) > 0) && latest.ContainsAll(missingNodes, missingAliases) {
		return latest, nil
	}

	var entries []NodeAliasEntry
	if cache.fullRefresh || latest == nil || latest.Size() == 0 {
		entries, err = cache.db.ListNodeAliases(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		entries, err = cache.db.GetNodeAliasEntries(ctx, GetNodeAliasEntries{
			Nodes:   missingNodes,
			Aliases: missingAliases,
		})
		if err != nil {
			return nil, err
		}
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

// EnsurePiecesToAliases converts pieces to alias pieces and automatically adds storage node
// to alias table when necessary.
func (cache *NodeAliasCache) EnsurePiecesToAliases(ctx context.Context, pieces Pieces) (_ AliasPieces, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(pieces) == 0 {
		return AliasPieces{}, nil
	}

	nodes := make([]storj.NodeID, len(pieces))
	for i, p := range pieces {
		nodes[i] = p.StorageNode
	}

	aliases, err := cache.EnsureAliases(ctx, nodes)
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
	return cache.convertAliasesToPieces(ctx, aliasPieces, make(Pieces, len(aliasPieces)))
}

// convertAliasesToPieces converts AliasPieces by populating Pieces with converted data.
func (cache *NodeAliasCache) convertAliasesToPieces(ctx context.Context, aliasPieces AliasPieces, pieces Pieces) (_ Pieces, err error) {
	if len(aliasPieces) == 0 {
		return Pieces{}, nil
	}

	if len(aliasPieces) != len(pieces) {
		return Pieces{}, Error.New("aliasPieces and pieces length must be equal")
	}

	latest := cache.getLatest()

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
			return Pieces{}, Error.New("failed to refresh node alias db: %w", err)
		}

		for i, aliasPiece := range aliasPieces {
			node, ok := latest.Node(aliasPiece.Alias)
			if !ok {
				return Pieces{}, Error.New("aliases missing in database: %v", missing)
			}
			pieces[i].Number = aliasPiece.Number
			pieces[i].StorageNode = node
		}
	}

	return pieces, nil
}
