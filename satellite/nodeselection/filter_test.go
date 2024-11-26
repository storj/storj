// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/shared/location"
)

func TestCriteria_ExcludeNodeID(t *testing.T) {
	included := testrand.NodeID()
	excluded := testrand.NodeID()

	criteria := NodeFilters{}.WithExcludedIDs([]storj.NodeID{excluded})

	assert.False(t, criteria.Match(&SelectedNode{
		ID: excluded,
	}))

	assert.True(t, criteria.Match(&SelectedNode{
		ID: included,
	}))
}

func TestCriteria_ExcludedNodeNetworks(t *testing.T) {
	criteria := NodeFilters{}
	criteria = append(criteria, ExcludedNodeNetworks{
		&SelectedNode{
			LastNet: "192.168.1.0",
		}, &SelectedNode{
			LastNet: "192.168.2.0",
		},
	})

	assert.False(t, criteria.Match(&SelectedNode{
		LastNet: "192.168.1.0",
	}))

	assert.False(t, criteria.Match(&SelectedNode{
		LastNet: "192.168.2.0",
	}))

	assert.True(t, criteria.Match(&SelectedNode{
		LastNet: "192.168.3.0",
	}))
}

func TestAnnotations(t *testing.T) {
	k := WithAnnotation(NodeFilters{}, "foo", "bar")
	require.Equal(t, "bar", k.GetAnnotation("foo"))

	k = NodeFilters{WithAnnotation(NodeFilters{}, "foo", "bar")}
	require.Equal(t, "bar", k.GetAnnotation("foo"))

	k = Annotation{
		Key:   "foo",
		Value: "bar",
	}
	require.Equal(t, "bar", k.GetAnnotation("foo"))

	// annotation can be used as pure filters
	l := Annotation{Key: "foo", Value: "bar"}
	require.True(t, l.Match(&SelectedNode{}))

	require.Equal(t, `annotation("foo","bar")`, l.String())
}

func TestCriteria_Geofencing(t *testing.T) {
	eu := NodeFilters{}.WithCountryFilter(location.NewSet(EuCountries...))
	us := NodeFilters{}.WithCountryFilter(location.NewSet(location.UnitedStates))

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
			assert.Equal(t, c.expected, c.criteria.Match(&SelectedNode{
				CountryCode: c.country,
			}))
		})
	}
}

func TestCountryFilter_FromString(t *testing.T) {
	cases := []struct {
		definition      []string
		canonical       string
		mustIncluded    []location.CountryCode
		mustNotIncluded []location.CountryCode
	}{
		{
			definition:      []string{"HU"},
			canonical:       `country("HU")`,
			mustIncluded:    []location.CountryCode{location.Hungary},
			mustNotIncluded: []location.CountryCode{location.Germany, location.UnitedStates},
		},
		{
			definition:      []string{"EU"},
			canonical:       "country(\"AT\",\"BE\",\"BG\",\"CY\",\"CZ\",\"DE\",\"DK\",\"EE\",\"ES\",\"FI\",\"FR\",\"GR\",\"HR\",\"HU\",\"IE\",\"IT\",\"LT\",\"LU\",\"LV\",\"MT\",\"NL\",\"PL\",\"PT\",\"RO\",\"SE\",\"SI\",\"SK\")",
			mustIncluded:    []location.CountryCode{location.Hungary, location.Germany, location.Austria},
			mustNotIncluded: []location.CountryCode{location.Iceland, location.UnitedStates, location.UnitedKingdom},
		},
		{
			definition:      []string{"EEA"},
			canonical:       "country(\"AT\",\"BE\",\"BG\",\"CY\",\"CZ\",\"DE\",\"DK\",\"EE\",\"ES\",\"FI\",\"FR\",\"GR\",\"HR\",\"HU\",\"IE\",\"IS\",\"IT\",\"LI\",\"LT\",\"LU\",\"LV\",\"MT\",\"NL\",\"NO\",\"PL\",\"PT\",\"RO\",\"SE\",\"SI\",\"SK\")",
			mustIncluded:    []location.CountryCode{location.Hungary, location.Germany, location.Austria, location.Iceland},
			mustNotIncluded: []location.CountryCode{location.UnitedStates, location.UnitedKingdom},
		},
		{
			definition:      []string{"EU", "US"},
			canonical:       "country(\"AT\",\"BE\",\"BG\",\"CY\",\"CZ\",\"DE\",\"DK\",\"EE\",\"ES\",\"FI\",\"FR\",\"GR\",\"HR\",\"HU\",\"IE\",\"IT\",\"LT\",\"LU\",\"LV\",\"MT\",\"NL\",\"PL\",\"PT\",\"RO\",\"SE\",\"SI\",\"SK\",\"US\")",
			mustIncluded:    []location.CountryCode{location.Hungary, location.Germany, location.UnitedStates},
			mustNotIncluded: []location.CountryCode{location.Russia},
		},
		{
			definition:      []string{"NONE"},
			canonical:       "country(\"\")",
			mustIncluded:    []location.CountryCode{},
			mustNotIncluded: []location.CountryCode{location.Germany, location.UnitedStates, location.Hungary},
		},
		{
			definition:      []string{"*", "!RU", "!BY"},
			canonical:       "country(\"*\",\"!BY\",\"!RU\")",
			mustIncluded:    []location.CountryCode{location.Hungary},
			mustNotIncluded: []location.CountryCode{location.Russia, location.Belarus},
		},
		{
			definition:      []string{"EU", "!DE"},
			canonical:       "country(\"AT\",\"BE\",\"BG\",\"CY\",\"CZ\",\"DK\",\"EE\",\"ES\",\"FI\",\"FR\",\"GR\",\"HR\",\"HU\",\"IE\",\"IT\",\"LT\",\"LU\",\"LV\",\"MT\",\"NL\",\"PL\",\"PT\",\"RO\",\"SE\",\"SI\",\"SK\")",
			mustIncluded:    []location.CountryCode{location.Hungary, location.TheNetherlands},
			mustNotIncluded: []location.CountryCode{location.Germany, location.UnitedStates, location.Russia},
		},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.definition, "_"), func(t *testing.T) {
			filter, err := NewCountryFilterFromString(tc.definition)
			require.NoError(t, err)
			for _, c := range tc.mustIncluded {
				require.True(t, filter.Match(&SelectedNode{
					CountryCode: c,
				}), "Country %s should be included", c.String())
			}
			for _, c := range tc.mustNotIncluded {
				require.False(t, filter.Match(&SelectedNode{
					CountryCode: c,
				}), "Country %s shouldn't be included", c.String())
			}
			require.Equal(t, tc.canonical, filter.String())
		})
	}
}

func TestContinentFilter_FromString(t *testing.T) {
	cases := []struct {
		code            string
		mustIncluded    []location.CountryCode
		mustNotIncluded []location.CountryCode
	}{
		{
			code:            "EU",
			mustIncluded:    []location.CountryCode{location.Hungary},
			mustNotIncluded: []location.CountryCode{location.India, location.UnitedStates},
		},
		{
			code:            "SA",
			mustIncluded:    []location.CountryCode{location.Brazil},
			mustNotIncluded: []location.CountryCode{location.Hungary},
		},
		{
			code:            "!NA",
			mustIncluded:    []location.CountryCode{location.Hungary},
			mustNotIncluded: []location.CountryCode{location.UnitedStates},
		},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			filter, err := NewContinentFilterFromString(tc.code)
			require.NoError(t, err)
			for _, c := range tc.mustIncluded {
				assert.True(t, filter.Match(&SelectedNode{
					CountryCode: c,
				}), "Country %s should be included", c.String())
			}
			for _, c := range tc.mustNotIncluded {
				assert.False(t, filter.Match(&SelectedNode{
					CountryCode: c,
				}), "Country %s shouldn't be included", c.String())
			}
		})
	}
}

func TestNodeListFilter(t *testing.T) {
	filter, err := AllowedNodesFromFile("filter_testdata.txt")
	require.NoError(t, err)
	selectedNode := func(pregeneratedIdentity int) *SelectedNode {
		return &SelectedNode{
			ID: testidentity.MustPregeneratedIdentity(pregeneratedIdentity, storj.LatestIDVersion()).ID,
		}
	}
	require.True(t, filter.Match(selectedNode(1)))
	require.True(t, filter.Match(selectedNode(2)))
	require.False(t, filter.Match(selectedNode(3)))
}

func TestAttributeFilter(t *testing.T) {
	filter, err := NewAttributeFilter("email", "==", "email@test")
	require.NoError(t, err)
	require.True(t, filter.Match(&SelectedNode{
		Email: "email@test",
	}))
	require.False(t, filter.Match(&SelectedNode{
		Email: "notemail@test",
	}))

	filter, err = NewAttributeFilter("email", "==", stringNotMatch(""))
	require.NoError(t, err)
	require.False(t, filter.Match(&SelectedNode{
		Email: "",
	}))

	filter, err = NewAttributeFilter("email", "!=", "email@test")
	require.NoError(t, err)
	require.True(t, filter.Match(&SelectedNode{
		Email: "email2@test",
	}))
	require.False(t, filter.Match(&SelectedNode{
		Email: "email@test",
	}))
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
			if filters.Match(nodes[n]) {
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
