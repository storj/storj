// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"sort"

	"github.com/zeebo/errs"

	"storj.io/common/dbutil/pgutil"
	"storj.io/common/storj"
)

// NodeAlias is a metabase local alias for NodeID-s to reduce segment table size.
type NodeAlias int32

// NodeAliasEntry is a mapping between NodeID and NodeAlias.
type NodeAliasEntry struct {
	ID    storj.NodeID
	Alias NodeAlias
}

// EnsureNodeAliases contains arguments necessary for creating NodeAlias-es.
type EnsureNodeAliases struct {
	Nodes []storj.NodeID
}

// EnsureNodeAliases ensures that the supplied node ID-s have a alias.
// It's safe to concurrently try and create node ID-s for the same NodeID.
func (db *DB) EnsureNodeAliases(ctx context.Context, opts EnsureNodeAliases) (err error) {
	defer mon.Task()(&ctx)(&err)

	unique := make([]storj.NodeID, 0, len(opts.Nodes))
	seen := make(map[storj.NodeID]bool, len(opts.Nodes))

	for _, node := range opts.Nodes {
		if node.IsZero() {
			return Error.New("tried to add alias to zero node")
		}
		if !seen[node] {
			seen[node] = true
			unique = append(unique, node)
		}
	}

	sort.Sort(storj.NodeIDList(unique))

	_, err = db.db.ExecContext(ctx, `
		INSERT INTO node_aliases(node_id)
		SELECT unnest($1::BYTEA[])
		ON CONFLICT DO NOTHING
	`, pgutil.NodeIDArray(unique))
	return Error.Wrap(err)
}

// ListNodeAliases lists all node alias mappings.
func (db *DB) ListNodeAliases(ctx context.Context) (_ []NodeAliasEntry, err error) {
	defer mon.Task()(&ctx)(&err)

	var aliases []NodeAliasEntry
	rows, err := db.db.Query(ctx, `
		SELECT node_id, node_alias
		FROM node_aliases
	`)
	if err != nil {
		return nil, Error.New("ListNodeAliases query: %w", err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var entry NodeAliasEntry
		err := rows.Scan(&entry.ID, &entry.Alias)
		if err != nil {
			return nil, Error.New("ListNodeAliases scan failed: %w", err)
		}
		aliases = append(aliases, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, Error.New("ListNodeAliases scan failed: %w", err)
	}

	return aliases, nil
}

// LatestNodesAliasMap returns the latest mapping between storj.NodeID and NodeAlias.
func (db *DB) LatestNodesAliasMap(ctx context.Context) (_ *NodeAliasMap, err error) {
	defer mon.Task()(&ctx)(&err)
	return db.aliasCache.Latest(ctx)
}
