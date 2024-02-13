// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
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

		aliases, err := cache.EnsureAliases(ctx, []storj.NodeID{n1, n2})
		require.NoError(t, err)
		require.Equal(t, []metabase.NodeAlias{1, 2}, aliases)

		nx1 := testrand.NodeID()
		aliases, err = cache.EnsureAliases(ctx, []storj.NodeID{nx1, n1, n2})
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

		aliases, err := cache.EnsureAliases(ctx, []storj.NodeID{n1, n2})
		require.EqualError(t, err, "metabase: failed to update node alias db: io.EOF")
		require.Empty(t, aliases)

		nodes, err := cache.Nodes(ctx, []metabase.NodeAlias{1, 2})
		require.EqualError(t, err, "metabase: failed to refresh node alias db: io.EOF")
		require.Empty(t, nodes)
	})

	t.Run("EnsureAliases refresh once", func(t *testing.T) {
		for repeat := 0; repeat < 3; repeat++ {
			database := &NodeAliasDB{}
			cache := metabase.NewNodeAliasCache(database)
			n1, n2 := testrand.NodeID(), testrand.NodeID()

			start := make(chan struct{})
			const N = 4
			var waiting sync.WaitGroup
			waiting.Add(N)

			var group errgroup.Group
			for k := 0; k < N; k++ {
				group.Go(func() error {
					waiting.Done()
					<-start

					_, err := cache.EnsureAliases(ctx, []storj.NodeID{n1, n2})
					return err
				})
			}

			waiting.Wait()
			close(start)
			require.NoError(t, group.Wait())

			require.Equal(t, int64(1), database.ListNodeAliasesCount())
		}
	})

	t.Run("Nodes refresh once", func(t *testing.T) {
		for repeat := 0; repeat < 3; repeat++ {
			n1, n2 := testrand.NodeID(), testrand.NodeID()

			database := &NodeAliasDB{}
			err := database.EnsureNodeAliases(ctx, metabase.EnsureNodeAliases{
				Nodes: []storj.NodeID{n1, n2},
			})
			require.NoError(t, err)

			cache := metabase.NewNodeAliasCache(database)

			start := make(chan struct{})
			const N = 4
			var waiting sync.WaitGroup
			waiting.Add(N)

			var group errgroup.Group
			for k := 0; k < N; k++ {
				group.Go(func() error {
					waiting.Done()
					<-start

					_, err := cache.Nodes(ctx, []metabase.NodeAlias{1, 2})
					return err
				})
			}

			waiting.Wait()
			close(start)
			require.NoError(t, group.Wait())

			require.Equal(t, int64(1), database.ListNodeAliasesCount())
		}
	})
}

func TestNodeAliasCache_DB(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("missing aliases", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			cache := metabase.NewNodeAliasCache(db)
			nodes, err := cache.Nodes(ctx, []metabase.NodeAlias{1, 2, 3})
			require.EqualError(t, err, "metabase: aliases missing in database: [1 2 3]")
			require.Empty(t, nodes)
		})

		t.Run("auto add nodes", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			cache := metabase.NewNodeAliasCache(db)

			n1, n2 := testrand.NodeID(), testrand.NodeID()

			aliases, err := cache.EnsureAliases(ctx, []storj.NodeID{n1})
			require.NoError(t, err)
			require.Equal(t, []metabase.NodeAlias{1}, aliases)

			aliases, err = cache.EnsureAliases(ctx, []storj.NodeID{n2})
			require.NoError(t, err)
			require.Equal(t, []metabase.NodeAlias{2}, aliases)

			aliases, err = cache.EnsureAliases(ctx, []storj.NodeID{n1, n2})
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

	{
		aggregate := metabase.NewNodeAliasMap(nil)
		alpha := metabase.NewNodeAliasMap([]metabase.NodeAliasEntry{
			{ID: n1, Alias: 1},
			{ID: n2, Alias: 2},
		})
		aggregate.Merge(alpha)
		beta := metabase.NewNodeAliasMap([]metabase.NodeAliasEntry{
			{ID: n3, Alias: 5},
		})
		aggregate.Merge(beta)

		aliases, missing := aggregate.Aliases([]storj.NodeID{n1, n2, n3})
		require.Empty(t, missing)
		require.Equal(t, []metabase.NodeAlias{1, 2, 5}, aliases)

		nodes2, missing2 := aggregate.Nodes([]metabase.NodeAlias{1, 2, 5})
		require.Empty(t, missing2)
		require.Equal(t, []storj.NodeID{n1, n2, n3}, nodes2)

		nodes3, missing3 := aggregate.Nodes([]metabase.NodeAlias{3, 4})
		require.Empty(t, nodes3)
		require.Equal(t, []metabase.NodeAlias{3, 4}, missing3)

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

func BenchmarkNodeAliasCache_ConvertAliasesToPieces(b *testing.B) {
	ctx := context.Background()

	aliasDB := &NodeAliasDB{}
	cache := metabase.NewNodeAliasCache(aliasDB)

	nodeIDs := make([]storj.NodeID, 80)
	for i := range nodeIDs {
		nodeIDs[i] = testrand.NodeID()
	}
	aliases, err := cache.EnsureAliases(ctx, nodeIDs)
	if err != nil {
		b.Fatal(err)
	}

	aliasPieces := make([]metabase.AliasPiece, len(aliases))
	for i, alias := range aliases {
		aliasPieces[i] = metabase.AliasPiece{Number: uint16(i), Alias: alias}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pieces, err := cache.ConvertAliasesToPieces(ctx, aliasPieces)
		if err != nil {
			b.Fatal(err)
		}
		runtime.KeepAlive(pieces)
	}
}

var _ metabase.NodeAliasDB = (*NodeAliasDB)(nil)

// NodeAliasDB is an inmemory alias database for testing.
type NodeAliasDB struct {
	mu      sync.Mutex
	fail    error
	last    metabase.NodeAlias
	entries []metabase.NodeAliasEntry

	ensureNodeAliasesCount int64
	listNodeAliasesCount   int64
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
	atomic.AddInt64(&db.ensureNodeAliasesCount, 1)

	if err := db.ShouldFail(); err != nil {
		return err
	}
	for _, id := range opts.Nodes {
		db.Ensure(id)
	}
	return nil
}

func (db *NodeAliasDB) EnsureNodeAliasesCount() int64 {
	return atomic.LoadInt64(&db.ensureNodeAliasesCount)
}

func (db *NodeAliasDB) ListNodeAliases(ctx context.Context) (_ []metabase.NodeAliasEntry, err error) {
	atomic.AddInt64(&db.listNodeAliasesCount, 1)

	if err := db.ShouldFail(); err != nil {
		return nil, err
	}

	db.mu.Lock()
	xs := append([]metabase.NodeAliasEntry{}, db.entries...)
	db.mu.Unlock()

	return xs, nil
}

func (db *NodeAliasDB) ListNodeAliasesCount() int64 {
	return atomic.LoadInt64(&db.listNodeAliasesCount)
}
