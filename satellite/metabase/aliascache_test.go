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
	t.Run("missing aliases", func(t *testing.T) {
		ctx := testcontext.New(t)

		cache := metabase.NewNodeAliasCache(&NodeAliasDB{}, false)
		nodes, err := cache.Nodes(ctx, []metabase.NodeAlias{1, 2, 3})
		require.EqualError(t, err, "metabase: aliases missing in database: [1 2 3]")
		require.Empty(t, nodes)
	})

	t.Run("auto add nodes", func(t *testing.T) {
		ctx := testcontext.New(t)

		cache := metabase.NewNodeAliasCache(&NodeAliasDB{}, false)

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
		ctx := testcontext.New(t)

		aliasDB := &NodeAliasDB{}
		aliasDB.SetFail(errors.New("io.EOF"))
		cache := metabase.NewNodeAliasCache(aliasDB, false)

		n1, n2 := testrand.NodeID(), testrand.NodeID()

		aliases, err := cache.EnsureAliases(ctx, []storj.NodeID{n1, n2})
		require.EqualError(t, err, "metabase: failed to update node alias db: io.EOF")
		require.Empty(t, aliases)

		nodes, err := cache.Nodes(ctx, []metabase.NodeAlias{1, 2})
		require.EqualError(t, err, "metabase: failed to refresh node alias db: io.EOF")
		require.Empty(t, nodes)
	})

	t.Run("EnsureAliases refresh once", func(t *testing.T) {
		ctx := testcontext.New(t)

		for repeat := 0; repeat < 3; repeat++ {
			database := &NodeAliasDB{}
			cache := metabase.NewNodeAliasCache(database, false)
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
		ctx := testcontext.New(t)

		for repeat := 0; repeat < 3; repeat++ {
			n1, n2 := testrand.NodeID(), testrand.NodeID()

			database := &NodeAliasDB{}
			err := database.EnsureNodeAliases(ctx, metabase.EnsureNodeAliases{
				Nodes: []storj.NodeID{n1, n2},
			})
			require.NoError(t, err)

			cache := metabase.NewNodeAliasCache(database, false)

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

			cache := metabase.NewNodeAliasCache(db, false)
			nodes, err := cache.Nodes(ctx, []metabase.NodeAlias{1, 2, 3})
			require.EqualError(t, err, "metabase: aliases missing in database: [1 2 3]")
			require.Empty(t, nodes)
		})

		t.Run("auto add nodes", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			cache := metabase.NewNodeAliasCache(db, false)

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

func BenchmarkNodeAliasCache_ConvertAliasesToPieces(b *testing.B) {
	ctx := b.Context()

	aliasDB := &NodeAliasDB{}
	cache := metabase.NewNodeAliasCache(aliasDB, false)

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

	ensureNodeAliasesCount   int64
	listNodeAliasesCount     int64
	getNodeAliasEntriesCount int64
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
func (db *NodeAliasDB) GetNodeAliasEntries(ctx context.Context, opts metabase.GetNodeAliasEntries) (_ []metabase.NodeAliasEntry, err error) {
	atomic.AddInt64(&db.getNodeAliasEntriesCount, 1)

	if err := db.ShouldFail(); err != nil {
		return nil, err
	}

	var xs []metabase.NodeAliasEntry
	db.mu.Lock()
	for i := range db.entries {
		entry := &db.entries[i]
		if nodesContains(opts.Nodes, entry.ID) || aliasesContains(opts.Aliases, entry.Alias) {
			xs = append(xs, *entry)
		}
	}
	db.mu.Unlock()

	return xs, nil
}

func (db *NodeAliasDB) GetNodeAliasEntriesCount() int64 {
	return atomic.LoadInt64(&db.getNodeAliasEntriesCount)
}

func (db *NodeAliasDB) ListNodeAliasesCount() int64 {
	return atomic.LoadInt64(&db.listNodeAliasesCount)
}

func aliasesContains(aliases []metabase.NodeAlias, v metabase.NodeAlias) bool {
	for _, n := range aliases {
		if n == v {
			return true
		}
	}
	return false
}
