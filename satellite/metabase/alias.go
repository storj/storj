// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"sort"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"
	"google.golang.org/grpc/codes"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/spannerutil"
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

	// TODO(spanner) long term this needs to be a coordinated insert across all adapters,
	// i.e. one of them needs to be the source of truth, otherwise there will be issues
	// with different db having different NodeAlias for the same node id.
	return db.ChooseAdapter(uuid.UUID{}).EnsureNodeAliases(ctx, opts)
}

// EnsureNodeAliases implements Adapter.
func (p *PostgresAdapter) EnsureNodeAliases(ctx context.Context, opts EnsureNodeAliases) (err error) {
	defer mon.Task()(&ctx)(&err)

	unique, err := ensureNodesUniqueness(opts.Nodes)
	if err != nil {
		return err
	}

	_, err = p.db.ExecContext(ctx, `
		INSERT INTO node_aliases(node_id)
		SELECT unnest($1::BYTEA[])
		ON CONFLICT DO NOTHING
	`, pgutil.NodeIDArray(unique))
	return Error.Wrap(err)
}

// EnsureNodeAliases implements Adapter.
func (s *SpannerAdapter) EnsureNodeAliases(ctx context.Context, opts EnsureNodeAliases) (err error) {
	defer mon.Task()(&ctx)(&err)

	unique, err := ensureNodesUniqueness(opts.Nodes)
	if err != nil {
		return err
	}

	// TODO(spanner): can this be combined into a single batch query?
	// TODO(spanner): this is inefficient, but there's a benefit from having densely packed node_aliases

	for _, id := range unique {
		_, err := s.client.ReadWriteTransaction(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
			_, err := txn.Update(ctx, spanner.Statement{
				SQL: `INSERT INTO node_aliases (
					node_id, node_alias
				) VALUES (
					@node_id,
					(SELECT COALESCE(MAX(node_alias)+1, 1) FROM node_aliases)
				)`,
				Params: map[string]any{
					"node_id": id,
				},
			})
			return Error.Wrap(err)
		})
		if spanner.ErrCode(err) == codes.AlreadyExists {
			continue
		}
		if err != nil {
			return Error.Wrap(err)
		}
	}

	return nil

}

func ensureNodesUniqueness(nodes []storj.NodeID) ([]storj.NodeID, error) {
	unique := make([]storj.NodeID, 0, len(nodes))
	seen := make(map[storj.NodeID]bool, len(nodes))

	for _, node := range nodes {
		if node.IsZero() {
			return nil, Error.New("tried to add alias to zero node")
		}
		if !seen[node] {
			seen[node] = true
			unique = append(unique, node)
		}
	}

	sort.Sort(storj.NodeIDList(unique))
	return unique, nil
}

// ListNodeAliases lists all node alias mappings.
func (db *DB) ListNodeAliases(ctx context.Context) (_ []NodeAliasEntry, err error) {
	defer mon.Task()(&ctx)(&err)

	return db.ChooseAdapter(uuid.UUID{}).ListNodeAliases(ctx)
}

// ListNodeAliases implements Adapter.
func (p *PostgresAdapter) ListNodeAliases(ctx context.Context) (_ []NodeAliasEntry, err error) {
	defer mon.Task()(&ctx)(&err)

	var aliases []NodeAliasEntry
	rows, err := p.db.Query(ctx, `
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

// ListNodeAliases implements Adapter.
func (s *SpannerAdapter) ListNodeAliases(ctx context.Context) (aliases []NodeAliasEntry, err error) {
	defer mon.Task()(&ctx)(&err)

	return spannerutil.CollectRows(
		s.client.Single().Query(ctx,
			spanner.Statement{SQL: `
				SELECT node_id, node_alias FROM node_aliases
			`}),
		func(row *spanner.Row, item *NodeAliasEntry) error {
			return Error.Wrap(row.Columns(&item.ID, spannerutil.Int(&item.Alias)))
		})
}

// LatestNodesAliasMap returns the latest mapping between storj.NodeID and NodeAlias.
func (db *DB) LatestNodesAliasMap(ctx context.Context) (_ *NodeAliasMap, err error) {
	defer mon.Task()(&ctx)(&err)
	return db.aliasCache.Latest(ctx)
}
