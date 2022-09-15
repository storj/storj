// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/private/dbutil/pgutil"
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

	for _, node := range opts.Nodes {
		if node.IsZero() {
			return Error.New("tried to add alias to zero node")
		}
	}

	_, err = db.db.ExecContext(ctx, `
		INSERT INTO node_aliases(node_id)
		SELECT unnest($1::BYTEA[])
		ON CONFLICT DO NOTHING
	`, pgutil.NodeIDArray(opts.Nodes))
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

// ConvertNodesToAliases converts nodeIDs to node aliases.
// Returns an error when an alias is missing.
func (db *DB) ConvertNodesToAliases(ctx context.Context, nodeIDs []storj.NodeID) (_ []NodeAlias, err error) {
	defer mon.Task()(&ctx)(&err)
	return db.aliasCache.Aliases(ctx, nodeIDs)
}

// ConvertAliasesToNodes converts aliases to node ID-s.
// Returns an error when a node alias is missing.
func (db *DB) ConvertAliasesToNodes(ctx context.Context, aliases []NodeAlias) (_ []storj.NodeID, err error) {
	defer mon.Task()(&ctx)(&err)
	return db.aliasCache.Nodes(ctx, aliases)
}
