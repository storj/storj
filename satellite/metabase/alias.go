// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"sort"

	"cloud.google.com/go/spanner"
	"github.com/jackc/pgtype"
	"github.com/zeebo/errs"
	"google.golang.org/grpc/codes"

	"storj.io/common/storj"
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
	//
	// For now, we can require that all participating processes have the same adapter
	// configured as "first", and use that one as the source of truth.
	return db.adapters[0].EnsureNodeAliases(ctx, opts)
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
		_, err := s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
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
		}, spanner.TransactionOptions{
			TransactionTag:              "ensure-node-aliases",
			ExcludeTxnFromChangeStreams: true,
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

	// TODO(spanner): long term this needs to be a coordinated get across all adapters,
	// i.e. one of them needs to be the source of truth, otherwise there will be issues
	// with different db having different NodeAlias for the same node id.
	//
	// For now, we can require that all participating processes have the same adapter
	// configured as "first", and use that one as the source of truth.
	return db.adapters[0].ListNodeAliases(ctx)
}

// ListNodeAliases implements Adapter.
func (p *PostgresAdapter) ListNodeAliases(ctx context.Context) (_ []NodeAliasEntry, err error) {
	defer mon.Task()(&ctx)(&err)

	var aliases []NodeAliasEntry
	rows, err := p.db.QueryContext(ctx, `
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
		s.client.Single().ReadWithOptions(ctx, "node_aliases", spanner.AllKeys(), []string{"node_id", "node_alias"}, &spanner.ReadOptions{
			RequestTag: "list-node-aliases",
		}),
		func(row *spanner.Row, item *NodeAliasEntry) error {
			return Error.Wrap(row.Columns(&item.ID, spannerutil.Int(&item.Alias)))
		})
}

// GetNodeAliasEntries contains arguments necessary for fetching node alias entries.
type GetNodeAliasEntries struct {
	Nodes   []storj.NodeID
	Aliases []NodeAlias
}

// GetNodeAliasEntries fetches node aliases or ID-s for the specified nodes and aliases in random order.
func (db *DB) GetNodeAliasEntries(ctx context.Context, opts GetNodeAliasEntries) (entries []NodeAliasEntry, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO(spanner): long term this needs to be a coordinated get across all adapters,
	// i.e. one of them needs to be the source of truth, otherwise there will be issues
	// with different db having different NodeAlias for the same node id.
	//
	// For now, we can require that all participating processes have the same adapter
	// configured as "first", and use that one as the source of truth.
	return db.adapters[0].GetNodeAliasEntries(ctx, opts)
}

// GetNodeAliasEntries implements Adapter.
func (p *PostgresAdapter) GetNodeAliasEntries(ctx context.Context, opts GetNodeAliasEntries) (_ []NodeAliasEntry, err error) {
	defer mon.Task()(&ctx)(&err)

	var entries []NodeAliasEntry
	rows, err := p.db.QueryContext(ctx, `
		SELECT node_id, node_alias
		FROM node_aliases
		WHERE node_id = ANY($1) OR node_alias = ANY($2)
	`, pgutil.NodeIDArray(opts.Nodes), nodeAliasesArray(opts.Aliases))
	if err != nil {
		return nil, Error.New("GetNodeAliasEntries query: %w", err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var entry NodeAliasEntry
		err := rows.Scan(&entry.ID, &entry.Alias)
		if err != nil {
			return nil, Error.New("GetNodeAliasEntries scan failed: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, Error.New("GetNodeAliasEntries scan failed: %w", err)
	}

	return entries, nil
}

// GetNodeAliasEntries implements Adapter.
func (s *SpannerAdapter) GetNodeAliasEntries(ctx context.Context, opts GetNodeAliasEntries) (_ []NodeAliasEntry, err error) {
	defer mon.Task()(&ctx)(&err)

	nodeids := [][]byte{}
	for _, id := range opts.Nodes {
		nodeids = append(nodeids, id.Bytes())
	}
	aliases := []int64{}
	for _, alias := range opts.Aliases {
		aliases = append(aliases, int64(alias))
	}

	return spannerutil.CollectRows(
		s.client.Single().QueryWithOptions(ctx,
			spanner.Statement{SQL: `
					SELECT node_id, node_alias FROM node_aliases
					WHERE node_id IN unnest(@nodes) OR node_alias IN unnest(@aliases)
				`,
				Params: map[string]any{
					"nodes":   nodeids,
					"aliases": aliases,
				}}, spanner.QueryOptions{RequestTag: "get-node-alias-entries"}),
		func(row *spanner.Row, item *NodeAliasEntry) error {
			return Error.Wrap(row.Columns(&item.ID, spannerutil.Int(&item.Alias)))
		})
}

// LatestNodesAliasMap returns the latest mapping between storj.NodeID and NodeAlias.
func (db *DB) LatestNodesAliasMap(ctx context.Context) (_ *NodeAliasMap, err error) {
	defer mon.Task()(&ctx)(&err)
	return db.aliasCache.Latest(ctx)
}

// nodeAliasesArray returns an object usable by pg drivers for passing a
// []NodeAlias slice into a database as type INT4[].
func nodeAliasesArray(ints []NodeAlias) *pgtype.Int4Array {
	pgtypeInt4Array := make([]pgtype.Int4, len(ints))
	for i, someInt := range ints {
		pgtypeInt4Array[i].Int = int32(someInt)
		pgtypeInt4Array[i].Status = pgtype.Present
	}
	return &pgtype.Int4Array{
		Elements:   pgtypeInt4Array,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(ints)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}
