// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
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

	// TODO(spanner) this is not prod ready implementation
	// TODO(spanner) limited alias value to avoid out of memory
	maxAliasValue := int64(10000)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// TODO(spanner) figure out how to do something like ON CONFLICT DO NOTHING
	index := 0
	for index < len(unique) {
		entry := unique[index]
		alias := rng.Int63n(maxAliasValue) + 1
		_, err = s.client.Apply(ctx, []*spanner.Mutation{
			spanner.Insert("node_aliases", []string{"node_id", "node_alias"}, []interface{}{
				entry.Bytes(), alias,
			}),
		})
		if err != nil {
			if spanner.ErrCode(err) == codes.AlreadyExists {
				// TODO(spanner) figure out how to detect UNIQUE violation
				if strings.Contains(spanner.ErrDesc(err), "UNIQUE violation on index node_aliases_node_alias_key") {
					// go back and find unique alias
					continue
				}
			} else {
				return Error.Wrap(err)
			}
		}
		index++
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

	stmt := spanner.Statement{SQL: `
		SELECT node_id, node_alias FROM node_aliases
	`}
	iter := s.client.Single().Query(ctx, stmt)
	defer iter.Stop()

	for {
		row, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			return aliases, nil
		}
		if err != nil {
			return nil, Error.Wrap(err)
		}

		var nodeID storj.NodeID
		var nodeAlias int64
		if err := row.Columns(&nodeID, &nodeAlias); err != nil {
			return nil, Error.New("ListNodeAliases scan failed: %w", err)
		}

		aliases = append(aliases, NodeAliasEntry{
			ID:    nodeID,
			Alias: NodeAlias(nodeAlias),
		})
	}
}

// LatestNodesAliasMap returns the latest mapping between storj.NodeID and NodeAlias.
func (db *DB) LatestNodesAliasMap(ctx context.Context) (_ *NodeAliasMap, err error) {
	defer mon.Task()(&ctx)(&err)
	return db.aliasCache.Latest(ctx)
}
