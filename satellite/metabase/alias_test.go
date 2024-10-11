// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"math/rand"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestNodeAliases(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("Zero", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			nodes := []storj.NodeID{
				testrand.NodeID(),
				{},
			}
			metabasetest.EnsureNodeAliases{
				Opts: metabase.EnsureNodeAliases{
					Nodes: nodes,
				},
				ErrClass: &metabase.Error,
				ErrText:  "tried to add alias to zero node",
			}.Check(ctx, t, db)
		})

		t.Run("Empty", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			aliasesAfter := metabasetest.ListNodeAliases{}.Check(ctx, t, db)
			require.Len(t, aliasesAfter, 0)
		})

		t.Run("Valid", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			nodes := []storj.NodeID{
				testrand.NodeID(),
				testrand.NodeID(),
				testrand.NodeID(),
			}
			nodes = append(nodes, nodes...) // add duplicates to our slice

			metabasetest.EnsureNodeAliases{
				Opts: metabase.EnsureNodeAliases{
					Nodes: nodes,
				},
			}.Check(ctx, t, db)

			metabasetest.EnsureNodeAliases{
				Opts: metabase.EnsureNodeAliases{
					Nodes: nodes,
				},
			}.Check(ctx, t, db)

			aliases := metabasetest.ListNodeAliases{}.Check(ctx, t, db)
			require.Len(t, aliases, 3)

			for _, entry := range aliases {
				require.True(t, nodesContains(nodes, entry.ID))
				require.LessOrEqual(t, int(entry.Alias), len(nodes))
			}

			metabasetest.EnsureNodeAliases{
				Opts: metabase.EnsureNodeAliases{
					Nodes: []storj.NodeID{testrand.NodeID()},
				},
			}.Check(ctx, t, db)

			aliasesAfter := metabasetest.ListNodeAliases{}.Check(ctx, t, db)
			require.Len(t, aliasesAfter, 4)
		})

		t.Run("GetNodeAliasEntries", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			nodes := []storj.NodeID{
				testrand.NodeID(),
				testrand.NodeID(),
				testrand.NodeID(),
			}

			metabasetest.EnsureNodeAliases{
				Opts: metabase.EnsureNodeAliases{
					Nodes: nodes,
				},
			}.Check(ctx, t, db)

			aliases := metabasetest.ListNodeAliases{}.Check(ctx, t, db)
			require.Len(t, aliases, 3)

			byid := metabasetest.GetNodeAliasEntries{
				Opts: metabase.GetNodeAliasEntries{
					Nodes: []storj.NodeID{aliases[1].ID},
				},
			}.Check(ctx, t, db)
			require.Len(t, byid, 1)
			require.Equal(t, []metabase.NodeAliasEntry{aliases[1]}, byid)

			byalias := metabasetest.GetNodeAliasEntries{
				Opts: metabase.GetNodeAliasEntries{
					Aliases: []metabase.NodeAlias{aliases[2].Alias},
				},
			}.Check(ctx, t, db)
			require.Len(t, byalias, 1)
			require.Equal(t, []metabase.NodeAliasEntry{aliases[2]}, byalias)

			bymix := metabasetest.GetNodeAliasEntries{
				Opts: metabase.GetNodeAliasEntries{
					Nodes:   []storj.NodeID{aliases[0].ID, aliases[1].ID},
					Aliases: []metabase.NodeAlias{aliases[2].Alias},
				},
			}.Check(ctx, t, db)
			require.Len(t, bymix, 3)

			sort.Slice(aliases, func(i, k int) bool { return aliases[i].Alias < aliases[k].Alias })
			sort.Slice(bymix, func(i, k int) bool { return bymix[i].Alias < bymix[k].Alias })

			require.Equal(t, aliases, bymix)

			missing := metabasetest.GetNodeAliasEntries{
				Opts: metabase.GetNodeAliasEntries{
					Nodes:   []storj.NodeID{{100}},
					Aliases: []metabase.NodeAlias{10000},
				},
			}.Check(ctx, t, db)
			require.Len(t, missing, 0)
		})

		t.Run("Concurrent", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			nodes := make([]storj.NodeID, 128)
			for i := range nodes {
				nodes[i] = testrand.NodeID()
			}

			var group errgroup.Group
			for k := range nodes {
				node := nodes[k]
				group.Go(func() error {
					return db.EnsureNodeAliases(ctx, metabase.EnsureNodeAliases{
						Nodes: []storj.NodeID{node},
					})
				})
			}
			require.NoError(t, group.Wait())

			aliases := metabasetest.ListNodeAliases{}.Check(ctx, t, db)
			seen := map[metabase.NodeAlias]bool{}
			require.Len(t, aliases, len(nodes))
			for _, entry := range aliases {
				require.True(t, nodesContains(nodes, entry.ID))
				require.LessOrEqual(t, int(entry.Alias), len(nodes))

				require.False(t, seen[entry.Alias])
				seen[entry.Alias] = true
			}
		})

		t.Run("Stress Concurrent", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			nodes := make([]storj.NodeID, 128)
			for i := range nodes {
				nodes[i] = testrand.NodeID()
			}
			group, gctx := errgroup.WithContext(ctx)
			for k := 0; k < 16; k++ {
				group.Go(func() error {
					loc := nodes
					for len(loc) > 0 {
						k := testrand.Intn(10)
						if k > len(loc) {
							k = len(loc)
						}
						var batch []storj.NodeID
						batch, loc = loc[:k], loc[k:]
						err := db.EnsureNodeAliases(gctx,
							metabase.EnsureNodeAliases{Nodes: batch},
						)
						if err != nil {
							panic(err)
						}

						if gctx.Err() != nil {
							break
						}
					}
					return nil //nolint: nilerr // the relevant errors are properly handled
				})
			}
			require.NoError(t, group.Wait())
		})

		t.Run("Stress Concurrent Random Order", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			nodes := make([]storj.NodeID, 128)
			for i := range nodes {
				nodes[i] = testrand.NodeID()
			}

			var group errgroup.Group
			const N = 16
			var preparations sync.WaitGroup
			preparations.Add(N)
			for k := 0; k < N; k++ {
				group.Go(func() error {
					batch := append([]storj.NodeID{}, nodes...)
					rand.Shuffle(len(batch), func(i, k int) {
						batch[i], batch[k] = batch[k], batch[i]
					})

					batch = batch[:len(batch)*2/3]

					preparations.Done()
					preparations.Wait()
					err := db.EnsureNodeAliases(ctx,
						metabase.EnsureNodeAliases{Nodes: batch},
					)
					if err != nil {
						return errs.Wrap(err)
					}

					return nil
				})
			}
			require.NoError(t, group.Wait())
		})

		t.Run("Stress Concurrent Swapped Order", func(t *testing.T) {
			// this test is trying to trigger deadlocks by having multiple
			// inserts in reverse order from each other (this used to be a
			// problem).
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			nodes := make([]storj.NodeID, 128)
			for i := range nodes {
				nodes[i] = testrand.NodeID()
			}

			var group errgroup.Group
			const N = 16
			var preparations sync.WaitGroup
			preparations.Add(N)
			for k := 0; k < N; k++ {
				k := k
				group.Go(func() error {
					batch := append([]storj.NodeID{}, nodes...)
					if k%2 == 0 {
						sort.Sort(storj.NodeIDList(batch))
					} else {
						sort.Sort(sort.Reverse(storj.NodeIDList(batch)))
					}

					preparations.Done()
					preparations.Wait()
					err := db.EnsureNodeAliases(ctx,
						metabase.EnsureNodeAliases{Nodes: batch},
					)
					if err != nil {
						return errs.Wrap(err)
					}

					return nil
				})
			}
			require.NoError(t, group.Wait())
		})
	})
}

func nodesContains(nodes []storj.NodeID, v storj.NodeID) bool {
	for _, n := range nodes {
		if n == v {
			return true
		}
	}
	return false
}
