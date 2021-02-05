// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metainfo/metabase"
)

func TestNodeAliasCache(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	t.Run("missing aliases", func(t *testing.T) {
		cache := metabase.NewNodeAliasCache(&NodeAliasDB{})
		nodes, err := cache.Nodes(ctx, []metabase.NodeAlias{1, 2, 3})
		require.EqualError(t, err, "metabase: aliases missing in database: [1 2 3]")
		require.Empty(t, nodes)
	})

	t.Run("auto add nodes", func(t *testing.T) {
		cache := metabase.NewNodeAliasCache(&NodeAliasDB{})

		n1, n2 := testrand.NodeID(), testrand.NodeID()

		aliases, err := cache.Aliases(ctx, []storj.NodeID{n1, n2})
		require.NoError(t, err)
		require.Equal(t, []metabase.NodeAlias{1, 2}, aliases)

		nx1 := testrand.NodeID()
		aliases, err = cache.Aliases(ctx, []storj.NodeID{nx1, n1, n2})
		require.NoError(t, err)
		require.Equal(t, []metabase.NodeAlias{3, 1, 2}, aliases)

		nodes, err := cache.Nodes(ctx, aliases)
		require.NoError(t, err)
		require.Equal(t, []storj.NodeID{nx1, n1, n2}, nodes)

		nodes, err = cache.Nodes(ctx, []metabase.NodeAlias{3, 4, 1, 2})
		require.EqualError(t, err, "metabase: aliases missing in database: [4]")
		require.Empty(t, nodes)
	})

	t.Run("db error", func(t *testing.T) {
		aliasDB := &NodeAliasDB{}
		aliasDB.SetFail(errors.New("io.EOF"))
		cache := metabase.NewNodeAliasCache(aliasDB)

		n1, n2 := testrand.NodeID(), testrand.NodeID()

		aliases, err := cache.Aliases(ctx, []storj.NodeID{n1, n2})
		require.EqualError(t, err, "metabase: failed to update node alias db: io.EOF")
		require.Empty(t, aliases)

		nodes, err := cache.Nodes(ctx, []metabase.NodeAlias{1, 2})
		require.EqualError(t, err, "metabase: failed to refresh node alias db: io.EOF")
		require.Empty(t, nodes)
	})
}

func TestNodeAliasCache_DB(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("missing aliases", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			cache := metabase.NewNodeAliasCache(db)
			nodes, err := cache.Nodes(ctx, []metabase.NodeAlias{1, 2, 3})
			require.EqualError(t, err, "metabase: aliases missing in database: [1 2 3]")
			require.Empty(t, nodes)
		})

		t.Run("auto add nodes", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			cache := metabase.NewNodeAliasCache(db)

			n1, n2 := testrand.NodeID(), testrand.NodeID()

			aliases, err := cache.Aliases(ctx, []storj.NodeID{n1, n2})
			require.NoError(t, err)
			require.Equal(t, []metabase.NodeAlias{1, 2}, aliases)

			nodes, err := cache.Nodes(ctx, aliases)
			require.NoError(t, err)
			require.Equal(t, []storj.NodeID{n1, n2}, nodes)
		})
	})
}

func TestNodeAliasMap(t *testing.T) {
	defer testcontext.New(t).Cleanup()

	n1 := testrand.NodeID()
	n2 := testrand.NodeID()
	n3 := testrand.NodeID()

	nx1 := testrand.NodeID()
	nx2 := testrand.NodeID()

	{
		emptyMap := metabase.NewNodeAliasMap(nil)
		nodes, missing := emptyMap.Nodes([]metabase.NodeAlias{0, 1, 2})
		require.Empty(t, nodes)
		require.Equal(t, []metabase.NodeAlias{0, 1, 2}, missing)
	}
	{
		emptyMap := metabase.NewNodeAliasMap(nil)
		aliases, missing := emptyMap.Aliases([]storj.NodeID{n1, n2, n3})
		require.Empty(t, aliases)
		require.Equal(t, []storj.NodeID{n1, n2, n3}, missing)
	}

	m := metabase.NewNodeAliasMap([]metabase.NodeAliasEntry{
		{n1, 1},
		{n2, 2},
		{n3, 3},
	})
	require.NotNil(t, m)
	require.Equal(t, 3, m.Size())

	testNodes := []struct {
		in      []metabase.NodeAlias
		out     []storj.NodeID
		missing []metabase.NodeAlias
	}{
		{
			in: nil,
		},
		{
			in:  []metabase.NodeAlias{1, 3, 2},
			out: []storj.NodeID{n1, n3, n2},
		},
		{
			in:      []metabase.NodeAlias{5, 4},
			missing: []metabase.NodeAlias{5, 4},
		},
	}
	for _, test := range testNodes {
		out, missing := m.Nodes(test.in)

		if len(out) == 0 {
			out = nil
		}
		if len(missing) == 0 {
			missing = nil
		}

		require.EqualValues(t, test.out, out)
		require.EqualValues(t, test.missing, missing)
	}

	testAliases := []struct {
		in      []storj.NodeID
		out     []metabase.NodeAlias
		missing []storj.NodeID
	}{
		{
			in: nil,
		},
		{
			in:  []storj.NodeID{n1, n3, n2},
			out: []metabase.NodeAlias{1, 3, 2},
		},
		{
			in:      []storj.NodeID{nx2, nx1},
			missing: []storj.NodeID{nx2, nx1},
		},
		{
			in:      []storj.NodeID{n1, nx2, n3, nx1, n2},
			out:     []metabase.NodeAlias{1, 3, 2},
			missing: []storj.NodeID{nx2, nx1},
		},
	}
	for _, test := range testAliases {
		out, missing := m.Aliases(test.in)

		if len(out) == 0 {
			out = nil
		}
		if len(missing) == 0 {
			missing = nil
		}

		require.EqualValues(t, test.out, out)
		require.EqualValues(t, test.missing, missing)
	}
}

var _ metabase.NodeAliasDB = (*NodeAliasDB)(nil)

// NodeAliasDB is an inmemory alias database for testing.
type NodeAliasDB struct {
	mu      sync.Mutex
	fail    error
	last    metabase.NodeAlias
	entries []metabase.NodeAliasEntry
}

func (db *NodeAliasDB) SetFail(err error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.fail = err
}

func (db *NodeAliasDB) ShouldFail() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.fail
}

func (db *NodeAliasDB) Ensure(id storj.NodeID) {
	db.mu.Lock()
	defer db.mu.Unlock()

	for _, e := range db.entries {
		if e.ID == id {
			return
		}
	}

	db.last++
	db.entries = append(db.entries, metabase.NodeAliasEntry{
		ID:    id,
		Alias: db.last,
	})
}

func (db *NodeAliasDB) EnsureNodeAliases(ctx context.Context, opts metabase.EnsureNodeAliases) error {
	if err := db.ShouldFail(); err != nil {
		return err
	}
	for _, id := range opts.Nodes {
		db.Ensure(id)
	}
	return nil
}

func (db *NodeAliasDB) ListNodeAliases(ctx context.Context) (_ []metabase.NodeAliasEntry, err error) {
	if err := db.ShouldFail(); err != nil {
		return nil, err
	}

	db.mu.Lock()
	xs := append([]metabase.NodeAliasEntry{}, db.entries...)
	db.mu.Unlock()

	return xs, nil
}
