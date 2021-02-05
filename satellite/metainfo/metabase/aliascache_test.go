// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metainfo/metabase"
)

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
