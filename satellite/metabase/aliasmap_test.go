// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
)

func TestNodeAliasMap(t *testing.T) {
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

func TestNodeAliasMap_SamePrefix(t *testing.T) {
	n1 := storj.NodeID{0, 1, 2, 3, 1}
	n2 := storj.NodeID{0, 1, 2, 3, 2}
	n3 := storj.NodeID{0, 1, 2, 3, 3}

	nx1 := storj.NodeID{0, 1, 2, 3, 4}
	nx2 := storj.NodeID{0, 1, 2, 3, 5}

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

func BenchmarkNodeAliasMap(b *testing.B) {
	var entries []metabase.NodeAliasEntry
	for i := 0; i < 20000; i++ {
		entries = append(entries, metabase.NodeAliasEntry{
			ID:    testrand.NodeID(),
			Alias: metabase.NodeAlias(i * 3 / 2),
		})
	}

	m := metabase.NewNodeAliasMap(entries)

	testindexes := []int{}
	for i := 0; i < 100; i++ {
		testindexes = append(testindexes, testrand.Intn(len(entries)))
	}

	b.Run("Alias", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, ix := range testindexes {
				entry := &entries[ix]
				alias, ok := m.Alias(entry.ID)
				if !ok || alias != entry.Alias {
					b.Fatal("wrong result")
				}
			}
		}
	})

	b.Run("ID", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, ix := range testindexes {
				entry := &entries[ix]
				id, ok := m.Node(entry.Alias)
				if !ok || id != entry.ID {
					b.Fatal("wrong result")
				}
			}
		}
	})
}
