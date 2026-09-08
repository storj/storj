// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
)

// signerZero is the tag signer of config_test.yaml ($SIGNER_ZERO).
var signerZero = func() storj.NodeID {
	id, err := storj.NodeIDFromString("1111111111111111111111111111111VyS547o")
	if err != nil {
		panic(err)
	}
	return id
}()

// groupTestNode creates a node with three generic, group related attributes. The tags are signed by
// signerZero, so they resolve both as `tag:key` (any signer) and as `tag:$SIGNER_ZERO/key`.
func groupTestNode(name string, groupA, groupB, groupC string) *SelectedNode {
	return &SelectedNode{
		ID:    groupTestNodeID(name),
		Email: name,
		Tags: NodeTags{
			{Signer: signerZero, Name: "groupA", Value: []byte(groupA)},
			{Signer: signerZero, Name: "groupB", Value: []byte(groupB)},
			{Signer: signerZero, Name: "groupC", Value: []byte(groupC)},
		},
	}
}

var groupTestNodeIDs = map[string]storj.NodeID{}

func groupTestNodeID(name string) storj.NodeID {
	id, found := groupTestNodeIDs[name]
	if !found {
		id = testrand.NodeID()
		groupTestNodeIDs[name] = id
	}
	return id
}

// successTrackerFunc is an UploadSuccessTracker with a fixed, per node score.
type successTrackerFunc func(node *SelectedNode) float64

func (f successTrackerFunc) Get(storj.NodeID) func(node *SelectedNode) float64 {
	return f
}

// testGroupAttribute is the group definition used by the tests: nodes are merged either by groupA,
// or by the composite key of groupB and groupC.
func testGroupAttribute(t *testing.T) *GroupAttribute {
	keyA, err := SameAttributes("tag:groupA")
	require.NoError(t, err)
	keyBC, err := SameAttributes("tag:groupB", "tag:groupC")
	require.NoError(t, err)
	group, err := NewGroupAttribute(keyA, keyBC)
	require.NoError(t, err)
	return group
}

// groupsOf returns the nodes (by email) for each resolved group label.
func groupsOf(attr NodeAttribute, nodes []*SelectedNode) map[string][]string {
	groups := map[string][]string{}
	for _, node := range nodes {
		label := attr(*node)
		groups[label] = append(groups[label], node.Email)
	}
	return groups
}

func TestGroupAttribute(t *testing.T) {
	group := testGroupAttribute(t)

	t.Run("transitive merge", func(t *testing.T) {
		// n1 and n2 are merged by groupA, n2 and n3 are merged by groupB+groupC, therefore all the
		// three are in the same group, even if n1 and n3 have nothing in common.
		n1 := groupTestNode("n1", "a1", "b1", "c1")
		n2 := groupTestNode("n2", "a1", "b2", "c2")
		n3 := groupTestNode("n3", "a3", "b2", "c2")
		n4 := groupTestNode("n4", "a4", "b4", "c4")
		nodes := []*SelectedNode{n1, n2, n3, n4}

		attr := group.Resolve(nodes)

		require.Equal(t, attr(*n1), attr(*n2))
		require.Equal(t, attr(*n2), attr(*n3))
		require.NotEqual(t, attr(*n1), attr(*n4))
		require.Len(t, groupsOf(attr, nodes), 2)
	})

	t.Run("composite key requires all attributes", func(t *testing.T) {
		// only one part of the composite key is the same --> not merged
		n1 := groupTestNode("n1", "a1", "b1", "c1")
		n2 := groupTestNode("n2", "a2", "b1", "c2")
		n3 := groupTestNode("n3", "a3", "b3", "c1")
		nodes := []*SelectedNode{n1, n2, n3}

		attr := group.Resolve(nodes)

		require.Len(t, groupsOf(attr, nodes), 3)
	})

	t.Run("unknown attributes are not merged", func(t *testing.T) {
		// empty values mean unknown, they shouldn't merge nodes (but nodes are still labeled)
		n1 := groupTestNode("n1", "", "", "")
		n2 := groupTestNode("n2", "", "", "")
		n3 := groupTestNode("n3", "", "b3", "")
		n4 := groupTestNode("n4", "", "b3", "")
		nodes := []*SelectedNode{n1, n2, n3, n4}

		attr := group.Resolve(nodes)

		require.Len(t, groupsOf(attr, nodes), 4)
		for _, node := range nodes {
			require.NotEmpty(t, attr(*node))
		}
	})

	t.Run("composite values are not ambiguous", func(t *testing.T) {
		// without a proper separator, "ab"+"c" and "a"+"bc" would be the same key
		n1 := groupTestNode("n1", "a1", "ab", "c")
		n2 := groupTestNode("n2", "a2", "a", "bc")
		nodes := []*SelectedNode{n1, n2}

		attr := group.Resolve(nodes)

		require.NotEqual(t, attr(*n1), attr(*n2))
	})

	t.Run("unknown node has no group", func(t *testing.T) {
		n1 := groupTestNode("n1", "a1", "b1", "c1")
		attr := group.Resolve([]*SelectedNode{n1})

		require.Empty(t, attr(SelectedNode{}))
		require.Empty(t, attr(*groupTestNode("n2", "a1", "b1", "c1")))
	})

	t.Run("placeholder nodes are ignored", func(t *testing.T) {
		// repair uses zero value nodes for the pieces of unknown nodes, those shouldn't be merged
		// together (even if all their attributes are equal)
		n1 := groupTestNode("n1", "a1", "b1", "c1")
		missing1 := &SelectedNode{}
		missing2 := &SelectedNode{}
		nodes := []*SelectedNode{n1, missing1, missing2}

		attr := group.Resolve(nodes)

		require.Empty(t, attr(*missing1))
		require.Empty(t, attr(*missing2))
		require.NotEmpty(t, attr(*n1))
	})

	t.Run("single attribute group is like a plain attribute", func(t *testing.T) {
		key, err := SameAttributes("tag:groupA")
		require.NoError(t, err)
		single, err := NewGroupAttribute(key)
		require.NoError(t, err)

		n1 := groupTestNode("n1", "a1", "b1", "c1")
		n2 := groupTestNode("n2", "a1", "b2", "c2")
		n3 := groupTestNode("n3", "a3", "b1", "c1")
		nodes := []*SelectedNode{n1, n2, n3}

		attr := single.Resolve(nodes)

		require.Equal(t, attr(*n1), attr(*n2))
		require.NotEqual(t, attr(*n1), attr(*n3))
	})

	t.Run("independent from node order", func(t *testing.T) {
		n1 := groupTestNode("n1", "a1", "b1", "c1")
		n2 := groupTestNode("n2", "a1", "b2", "c2")
		n3 := groupTestNode("n3", "a3", "b2", "c2")
		n4 := groupTestNode("n4", "a4", "b4", "c4")

		expected := [][]string{{"n1", "n2", "n3"}, {"n4"}}

		for _, nodes := range [][]*SelectedNode{
			{n1, n2, n3, n4},
			{n4, n3, n2, n1},
			{n3, n1, n4, n2},
			{n2, n4, n1, n3},
		} {
			var groups [][]string
			for _, members := range groupsOf(group.Resolve(nodes), nodes) {
				sorted := append([]string{}, members...)
				sort.Strings(sorted)
				groups = append(groups, sorted)
			}
			sort.Slice(groups, func(i, j int) bool {
				return groups[i][0] < groups[j][0]
			})
			require.Equal(t, expected, groups)
		}
	})
}

func TestGroupAttributeString(t *testing.T) {
	require.Equal(t, `group(same(tag:groupA),same(tag:groupB,tag:groupC))`, testGroupAttribute(t).String())
}

func TestGroupAttributeErrors(t *testing.T) {
	_, err := NewGroupAttribute()
	require.Error(t, err)

	_, err = SameAttributes()
	require.Error(t, err)

	_, err = SameAttributes("no_such_attribute")
	require.Error(t, err)
}

func TestGroupAttributeInvariant(t *testing.T) {
	// n1-n2-n3 are transitively in the same group, n4 is alone
	nodes := []SelectedNode{
		*groupTestNode("n1", "a1", "b1", "c1"),
		*groupTestNode("n2", "a1", "b2", "c2"),
		*groupTestNode("n3", "a3", "b2", "c2"),
		*groupTestNode("n4", "a4", "b4", "c4"),
	}
	var pieces metabase.Pieces
	for ix, node := range nodes {
		pieces = append(pieces, metabase.Piece{
			Number:      uint16(ix),
			StorageNode: node.ID,
		})
	}

	t.Run("group", func(t *testing.T) {
		invariant, err := InvariantFromString(`maxcontrol(group(same("tag:groupA"),same("tag:groupB","tag:groupC")),1)`)
		require.NoError(t, err)

		result := invariant(pieces, nodes)

		// n1 is fine, n2 and n3 are in the same group as n1, n4 is alone
		require.Equal(t, 2, result.Count())
		require.True(t, result.Contains(1))
		require.True(t, result.Contains(2))
	})

	t.Run("single attributes are not enough", func(t *testing.T) {
		// neither of the attributes alone detects the clumping
		for _, expr := range []string{
			`maxcontrol("tag:groupA",1)`,
			`maxcontrol("tag:groupB",1)`,
		} {
			invariant, err := InvariantFromString(expr)
			require.NoError(t, err)
			require.Equal(t, 1, invariant(pieces, nodes).Count(), expr)
		}
	})

	t.Run("placeholder nodes are not clumped", func(t *testing.T) {
		invariant, err := InvariantFromString(`maxcontrol(group(same("tag:groupA"),same("tag:groupB","tag:groupC")),1)`)
		require.NoError(t, err)

		withMissing := []SelectedNode{
			*groupTestNode("n1", "a1", "b1", "c1"),
			{},
			{},
		}
		result := invariant(pieces[:len(withMissing)], withMissing)
		require.Equal(t, 0, result.Count())
	})
}

// TestGroupAttributeMergeKeyTrust pins the trust note of GroupAttribute: a merge key built from a
// node declared attribute lets one node merge two unrelated groups, which a plain attribute cannot
// do. The signed tag form is immune, which is why config_test.yaml uses it.
func TestGroupAttributeMergeKeyTrust(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// two unrelated operators: a1/a2 share groupA, b1/b2 share groupB+groupC
	honest := []*SelectedNode{
		groupTestNode("a1", "opA", "", ""),
		groupTestNode("a2", "opA", "", ""),
		groupTestNode("b1", "", "opB", "x"),
		groupTestNode("b2", "", "opB", "x"),
	}
	// one node claiming the values of both operators, self signed
	liar := groupTestNode("liar", "opA", "opB", "x")
	liar.Tags = NodeTags{
		{Signer: liar.ID, Name: "groupA", Value: []byte("opA")},
		{Signer: liar.ID, Name: "groupB", Value: []byte("opB")},
		{Signer: liar.ID, Name: "groupC", Value: []byte("x")},
	}

	selectable := func(t *testing.T, expr string, nodes []*SelectedNode) int {
		init, err := SelectorFromString(`attribute(`+expr+`)`, nil)
		require.NoError(t, err)
		selected, err := init(ctx, nodes, nil)(ctx, storj.NodeID{}, len(nodes), nil, nil)
		require.NoError(t, err)
		return len(selected)
	}

	t.Run("any signer tags can be forged", func(t *testing.T) {
		expr := `group(same("tag:groupA"),same("tag:groupB","tag:groupC"))`
		require.Equal(t, 2, selectable(t, expr, honest))
		// the liar merged the two groups, so only one node of the five is selectable
		require.Equal(t, 1, selectable(t, expr, append(honest, liar)))
	})

	t.Run("signed tags are not affected", func(t *testing.T) {
		signed := "tag:" + signerZero.String() + "/"
		expr := `group(same("` + signed + `groupA"),same("` + signed + `groupB","` + signed + `groupC"))`
		require.Equal(t, 2, selectable(t, expr, honest))
		// the liar's self signed tags don't resolve, so it is its own group and the other two stand
		require.Equal(t, 3, selectable(t, expr, append(honest, liar)))
	})
}

// TestClumpingByGroupMissesTransitiveConnection pins the limitation documented on ClumpingByGroup:
// invariants only see the nodes of the checked segment, so a transitive connection is invisible
// here unless the bridging node holds a piece of the same segment. This is a known gap, not a
// property to rely on - if the invariant ever resolves on the full node set, this test should be
// replaced (and the doc comments of ClumpingByGroup and maxcontrol updated with it).
func TestClumpingByGroupMissesTransitiveConnection(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// x and z are in the same group, but only through y: x shares groupA with y, and y shares
	// groupB/groupC with z. x and z have no attribute in common.
	x := groupTestNode("x", "a1", "", "")
	y := groupTestNode("y", "a1", "b2", "c2")
	z := groupTestNode("z", "", "b2", "c2")

	expr := `group(same("tag:groupA"),same("tag:groupB","tag:groupC"))`

	invariant, err := InvariantFromString(`maxcontrol(` + expr + `,1)`)
	require.NoError(t, err)

	pieces := func(nodes ...*SelectedNode) (pieces metabase.Pieces, records []SelectedNode) {
		for ix, node := range nodes {
			pieces = append(pieces, metabase.Piece{
				Number:      uint16(ix),
				StorageNode: node.ID,
			})
			records = append(records, *node)
		}
		return pieces, records
	}

	t.Run("bridging node has no piece: clumping is missed", func(t *testing.T) {
		require.Equal(t, 0, invariant(pieces(x, z)).Count())
	})

	t.Run("bridging node has a piece: clumping is detected", func(t *testing.T) {
		// the very same x and z, the only difference is that y holds a piece of this segment
		result := invariant(pieces(x, y, z))
		require.Equal(t, 2, result.Count())
		require.True(t, result.Contains(1))
		require.True(t, result.Contains(2))
	})

	t.Run("the selector does see the connection", func(t *testing.T) {
		// the asymmetry the doc comment describes: selection resolves the groups on the full node
		// set, so a new upload never puts pieces on both x and z, even though the invariant above
		// cannot flag that pair afterwards.
		selectorInit, err := SelectorFromString(`attribute(`+expr+`)`, nil)
		require.NoError(t, err)

		selector := selectorInit(ctx, []*SelectedNode{x, y, z}, nil)
		selected, err := selector(ctx, storj.NodeID{}, 2, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, 1)
	})
}

func TestGroupAttributeSelector(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	config, err := LoadConfigFromString(`
placements:
    - id: 1
      name: grouped
      selector: attribute(group(same("tag:groupA"),same("tag:groupB","tag:groupC")))
      invariant: maxcontrol(group(same("tag:groupA"),same("tag:groupB","tag:groupC")),1)
`, NewPlacementConfigEnvironment(nil, nil))
	require.NoError(t, err)

	// three groups: {n1,n2,n3} (transitively), {n4}, {n5}
	nodes := []*SelectedNode{
		groupTestNode("n1", "a1", "b1", "c1"),
		groupTestNode("n2", "a1", "b2", "c2"),
		groupTestNode("n3", "a3", "b2", "c2"),
		groupTestNode("n4", "a4", "b4", "c4"),
		groupTestNode("n5", "a5", "b5", "c5"),
	}

	selector := config[1].Selector(ctx, nodes, nil)

	for i := 0; i < 100; i++ {
		selected, err := selector(ctx, storj.NodeID{}, 3, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, 3)

		clumped := map[string]bool{}
		for _, node := range selected {
			// n1, n2 and n3 are interchangeable, only one of them can be selected
			key := node.Email
			if key == "n2" || key == "n3" {
				key = "n1"
			}
			require.False(t, clumped[key], "two nodes from the same group are selected")
			clumped[key] = true
		}
	}
}

func TestGroupAttributeSelectorPartitioned(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// n1 and n3 are in the same group, but only through n2. Selectors which split the node set
	// must still resolve the group on the full set, otherwise the connection through n2 is lost
	// and both n1 and n3 become selectable.
	newNodes := func() []*SelectedNode {
		return []*SelectedNode{
			groupTestNode("n1", "a1", "b1", "c1"),
			groupTestNode("n2", "a1", "b2", "c2"),
			groupTestNode("n3", "a3", "b2", "c2"),
		}
	}

	group := `group(same("tag:groupA"),same("tag:groupB","tag:groupC"))`

	// ranks n2 as the worst node, so the selectors cutting off the long tail drop exactly the
	// connecting node.
	worstIsN2 := successTrackerFunc(func(node *SelectedNode) float64 {
		if node.Email == "n2" {
			return 0
		}
		return 1
	})

	for _, tc := range []struct {
		name     string
		selector string
		tracker  UploadSuccessTracker
		prepare  func(nodes []*SelectedNode)
	}{
		{
			// the connecting node is unvetted, the selection uses the vetted partition only
			name:     "unvetted",
			selector: `unvetted(0.0,attribute(` + group + `))`,
			prepare: func(nodes []*SelectedNode) {
				nodes[0].Vetted = true
				nodes[2].Vetted = true
			},
		},
		{
			// the connecting node is filtered out before the delegate is initialized
			name:     "filter",
			selector: `filter(select("email","!=","n2"),attribute(` + group + `))`,
			prepare:  func(nodes []*SelectedNode) {},
		},
		{
			// both wrappers: the outermost one has to provide the full set
			name:     "filter of unvetted",
			selector: `filter(select("email","!=","n2"),unvetted(0.0,attribute(` + group + `)))`,
			prepare: func(nodes []*SelectedNode) {
				nodes[0].Vetted = true
				nodes[2].Vetted = true
			},
		},
		{
			// the connecting node is cut off as the worst node of the long tail
			name:     "filterbest",
			selector: `filterbest(tracker,"-1","",attribute(` + group + `))`,
			tracker:  worstIsN2,
			prepare:  func(nodes []*SelectedNode) {},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := LoadConfigFromString(`
placements:
    - id: 1
      name: grouped
      selector: `+tc.selector+`
`, NewPlacementConfigEnvironment(tc.tracker, nil))
			require.NoError(t, err)

			nodes := newNodes()
			tc.prepare(nodes)

			selected, err := config[1].Selector(ctx, nodes, nil)(ctx, storj.NodeID{}, 3, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 1, "n1 and n3 are in the same group, only one can be selected")
		})
	}
}

func TestGroupAttributeConfigErrors(t *testing.T) {
	for _, expr := range []string{
		`attribute(group())`,
		`attribute(group(same()))`,
		`attribute(group(same("no_such_attribute")))`,
		`attribute(group(random()))`,
	} {
		_, err := SelectorFromString(expr, NewPlacementConfigEnvironment(nil, nil))
		require.Error(t, err, expr)
	}

	// attribute name shorthand is also accepted
	_, err := SelectorFromString(`attribute(group("tag:groupA","tag:groupB"))`, NewPlacementConfigEnvironment(nil, nil))
	require.NoError(t, err)
}

// BenchmarkClumpingInvariant compares the cost of the plain and the group based invariant, as the
// invariant is executed for each segment by the repair checker.
func BenchmarkClumpingInvariant(b *testing.B) {
	var nodes []SelectedNode
	var pieces metabase.Pieces
	for ix := 0; ix < 80; ix++ {
		nodes = append(nodes, *groupTestNode(fmt.Sprintf("bench%d", ix),
			fmt.Sprintf("a%d", ix/2),
			fmt.Sprintf("b%d", ix/3),
			fmt.Sprintf("c%d", ix/5)))
		pieces = append(pieces, metabase.Piece{
			Number:      uint16(ix),
			StorageNode: nodes[ix].ID,
		})
	}

	for _, tc := range []struct {
		name string
		expr string
	}{
		{"plain", `maxcontrol("tag:groupA",1)`},
		{"group", `maxcontrol(group(same("tag:groupA"),same("tag:groupB","tag:groupC")),1)`},
	} {
		b.Run(tc.name, func(b *testing.B) {
			invariant, err := InvariantFromString(tc.expr)
			require.NoError(b, err)
			for i := 0; i < b.N; i++ {
				invariant(pieces, nodes)
			}
		})
	}
}

func BenchmarkGroupAttributeResolve(b *testing.B) {
	group := &GroupAttribute{}
	for _, attributes := range [][]string{{"tag:groupA"}, {"tag:groupB", "tag:groupC"}} {
		key, err := SameAttributes(attributes...)
		require.NoError(b, err)
		group.keys = append(group.keys, key)
	}

	var nodes []*SelectedNode
	for ix := 0; ix < 25000; ix++ {
		nodes = append(nodes, groupTestNode(fmt.Sprintf("n%d", ix),
			fmt.Sprintf("a%d", ix/2),
			fmt.Sprintf("b%d", ix/3),
			fmt.Sprintf("c%d", ix/5)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		group.Resolve(nodes)
	}
}
