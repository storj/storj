// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
)

func TestNodeFilter_AutoExcludeSubnet(t *testing.T) {

	criteria := NodeFilters{}.WithAutoExcludeSubnets()

	assert.True(t, criteria.MatchInclude(&SelectedNode{
		LastNet: "192.168.0.1",
	}))

	assert.False(t, criteria.MatchInclude(&SelectedNode{
		LastNet: "192.168.0.1",
	}))

	assert.True(t, criteria.MatchInclude(&SelectedNode{
		LastNet: "192.168.1.1",
	}))
}

func TestCriteria_ExcludeNodeID(t *testing.T) {
	included := testrand.NodeID()
	excluded := testrand.NodeID()

	criteria := NodeFilters{}.WithExcludedIDs([]storj.NodeID{excluded})

	assert.False(t, criteria.MatchInclude(&SelectedNode{
		ID: excluded,
	}))

	assert.True(t, criteria.MatchInclude(&SelectedNode{
		ID: included,
	}))

}

func TestCriteria_NodeIDAndSubnet(t *testing.T) {
	excluded := testrand.NodeID()

	criteria := NodeFilters{}.
		WithExcludedIDs([]storj.NodeID{excluded}).
		WithAutoExcludeSubnets()

	// due to node id criteria
	assert.False(t, criteria.MatchInclude(&SelectedNode{
		ID:      excluded,
		LastNet: "192.168.0.1",
	}))

	// should be included as previous one excluded and
	// not stored for subnet exclusion
	assert.True(t, criteria.MatchInclude(&SelectedNode{
		ID:      testrand.NodeID(),
		LastNet: "192.168.0.2",
	}))

}

func TestCriteria_Geofencing(t *testing.T) {
	eu := NodeFilters{}.WithCountryFilter(func(code location.CountryCode) bool {
		for _, c := range location.EuCountries {
			if c == code {
				return true
			}
		}
		return false
	})

	us := NodeFilters{}.WithCountryFilter(func(code location.CountryCode) bool {
		return code == location.UnitedStates
	})

	cases := []struct {
		name     string
		country  location.CountryCode
		criteria NodeFilters
		expected bool
	}{
		{
			name:     "US matches US selector",
			country:  location.UnitedStates,
			criteria: us,
			expected: true,
		},
		{
			name:     "Germany is EU",
			country:  location.Germany,
			criteria: eu,
			expected: true,
		},
		{
			name:     "US is not eu",
			country:  location.UnitedStates,
			criteria: eu,
			expected: false,
		},
		{
			name:     "Empty country doesn't match region",
			country:  location.CountryCode(0),
			criteria: eu,
			expected: false,
		},
		{
			name:     "Empty country doesn't match country",
			country:  location.CountryCode(0),
			criteria: us,
			expected: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, c.criteria.MatchInclude(&SelectedNode{
				CountryCode: c.country,
			}))
		})
	}
}

// BenchmarkNodeFilterFullTable checks performances of rule evaluation on ALL storage nodes.
func BenchmarkNodeFilterFullTable(b *testing.B) {
	filters := NodeFilters{}
	filters = append(filters, NodeFilterFunc(func(node *SelectedNode) bool {
		return true
	}))
	filters = append(filters, NodeFilterFunc(func(node *SelectedNode) bool {
		return true
	}))
	filters = append(filters, NodeFilterFunc(func(node *SelectedNode) bool {
		return true
	}))
	filters = filters.WithAutoExcludeSubnets()
	benchmarkFilter(b, filters)
}

func benchmarkFilter(b *testing.B, filters NodeFilters) {
	nodeNo := 25000
	if testing.Short() {
		nodeNo = 20
	}
	nodes := generatedSelectedNodes(b, nodeNo)

	b.ResetTimer()
	c := 0
	for j := 0; j < b.N; j++ {
		for n := 0; n < len(nodes); n++ {
			if filters.MatchInclude(nodes[n]) {
				c++
			}
		}
	}

}

func generatedSelectedNodes(b *testing.B, nodeNo int) []*SelectedNode {
	nodes := make([]*SelectedNode, nodeNo)
	ctx := testcontext.New(b)
	for i := 0; i < nodeNo; i++ {
		node := SelectedNode{}
		identity, err := testidentity.NewTestIdentity(ctx)
		require.NoError(b, err)
		node.ID = identity.ID
		node.LastNet = fmt.Sprintf("192.168.%d.0", i%256)
		node.LastIPPort = fmt.Sprintf("192.168.%d.%d:%d", i%256, i%65536, i%1000+1000)
		node.CountryCode = []location.CountryCode{location.None, location.UnitedStates, location.Germany, location.Hungary, location.Austria}[i%5]
		nodes[i] = &node
	}
	return nodes
}
