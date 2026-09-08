// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"strconv"
	"strings"

	"storj.io/common/storj"
)

// MergeKey is a composite key of node attributes. Two nodes are merged to the same group, if the
// value of all the attributes are equal (and not empty) for both nodes.
//
// Merge keys should only use attributes which the node itself cannot choose: satellite derived ones
// (last_net, last_ip, country, id) or tags of a trusted signer (tag:signer/key). See GroupAttribute
// for why node declared attributes (tag:key without signer, wallet, email) are a hazard here even
// though they are harmless in a plain attribute().
type MergeKey struct {
	attributes []NodeAttribute
	definition string
}

// NewMergeKey creates a MergeKey from already created node attributes.
func NewMergeKey(definition string, attributes ...NodeAttribute) (MergeKey, error) {
	if len(attributes) == 0 {
		return MergeKey{}, Error.New("merge key requires at least one node attribute")
	}
	return MergeKey{
		attributes: attributes,
		definition: definition,
	}, nil
}

// SameAttributes creates a MergeKey from node attribute definitions (like "last_net" or "tag:key").
func SameAttributes(attributes ...string) (MergeKey, error) {
	if len(attributes) == 0 {
		return MergeKey{}, Error.New("same() requires at least one node attribute")
	}
	var attrs []NodeAttribute
	for _, definition := range attributes {
		attr, err := CreateNodeAttribute(definition)
		if err != nil {
			return MergeKey{}, err
		}
		attrs = append(attrs, attr)
	}
	return NewMergeKey("same("+strings.Join(attributes, ",")+")", attrs...)
}

// appendValue appends the composite value of the key to dst, and returns the extended buffer. The
// caller can reuse the buffer between nodes, so the value doesn't have to be allocated per node.
// Returns false, if any of the attributes are empty: unknown values shouldn't merge nodes together.
func (m MergeKey) appendValue(dst []byte, node *SelectedNode) ([]byte, bool) {
	// zero byte separator makes sure that ("ab","c") and ("a","bc") are different keys
	for ix, attr := range m.attributes {
		value := attr(*node)
		if value == "" {
			return dst, false
		}
		if ix > 0 {
			dst = append(dst, 0)
		}
		dst = append(dst, value...)
	}
	return dst, true
}

// String implements fmt.Stringer.
func (m MergeKey) String() string {
	return m.definition
}

// GroupAttribute is a set aware node attribute: it labels each node with the group (connected
// component) the node belongs to. Nodes are in the same group, if they are connected by any of the
// merge keys, either directly or transitively.
//
// For example, with merge keys [A] and [B,C]: if node1 and node2 have the same A, and node2 and
// node3 have both the same B and C, then all three nodes are in the same group, even if node1 and
// node3 doesn't have any attribute in common.
//
// Group labels are only meaningful inside one resolution (nodes of the same group are labeled with
// the same string), and they are not stable between two resolutions. Nodes with unknown (empty)
// attributes are never merged, they form their own single node group.
//
// Trust: merge keys should only use attributes which the node cannot choose. Transitivity changes
// the blast radius of a lying node, which is why this matters more here than for a plain
// attribute(). With a plain attribute a node which misreports only moves itself to another bucket.
// With a merge key it can join two groups it has nothing to do with: a node claiming operator A's
// value for one key and operator B's values for another merges A's and B's entire groups into one
// connected component. Since the selector keeps at most one node per group, k such nodes can merge
// k+1 real groups and shrink the selectable set, down to ErrNotEnoughNodes. The direction is not
// symmetric - forged values can only merge groups, never split them - so it is an availability
// problem and not a way to defeat declumping.
//
// Node declared attributes are: tag:key without a signer (AnyNodeTagAttribute takes the first tag
// with that name, and contact verifies tags against the node itself as well, so a node can sign its
// own), wallet and email (sent in the check-in, never verified). Satellite derived attributes
// (last_net, last_ip, last_ip_port, country, id) and tags of a trusted signer (tag:signer/key) are
// safe. This is not enforced in the config parser, because wallet and email are the natural way to
// group by operator and rejecting only the tag form would suggest a guarantee which isn't there.
type GroupAttribute struct {
	keys []MergeKey
}

// NewGroupAttribute creates a GroupAttribute from merge keys.
func NewGroupAttribute(keys ...MergeKey) (*GroupAttribute, error) {
	if len(keys) == 0 {
		return nil, Error.New("group() requires at least one merge key (like same(\"last_net\"))")
	}
	return &GroupAttribute{
		keys: keys,
	}, nil
}

// String implements fmt.Stringer.
func (g *GroupAttribute) String() string {
	var definitions []string
	for _, key := range g.keys {
		definitions = append(definitions, key.String())
	}
	return "group(" + strings.Join(definitions, ",") + ")"
}

// Init returns the NodeAttributeInit, which resolves the groups for a specific set of nodes.
func (g *GroupAttribute) Init() NodeAttributeInit {
	return g.Resolve
}

// Resolve calculates the groups of the given nodes, and returns a NodeAttribute which is valid only
// for these nodes. Nodes which are not part of the set are labeled with empty string.
func (g *GroupAttribute) Resolve(nodes []*SelectedNode) NodeAttribute {
	ids := g.Groups(nodes)

	// the label of a group is created only once per group (instead of once per node), and it's
	// memoized by group id, which is a dense index into the node set.
	labels := make([]string, len(nodes))
	nodeLabels := make(map[storj.NodeID]string, len(nodes))
	for ix, id := range ids {
		if id < 0 {
			continue
		}
		if labels[id] == "" {
			labels[id] = "group:" + strconv.Itoa(int(id))
		}
		nodeLabels[nodes[ix].ID] = labels[id]
	}

	return func(node SelectedNode) string {
		return nodeLabels[node.ID]
	}
}

// Groups calculates the groups of the given nodes, and returns the group id of each node, by index.
// Nodes with unknown identity (like the placeholder of a missing piece) get -1: they are not part of
// any group.
//
// Group ids are only meaningful inside one resolution: they are the index of the union-find root,
// which is picked by subtree size and not by order (so it's not the lowest index of the group).
// They are deterministic for the same node order, but not stable between two resolutions.
//
// Callers which only compare nodes for group equality (like invariants) should use this directly,
// the labels of Resolve() are only needed where a NodeAttribute is expected.
func (g *GroupAttribute) Groups(nodes []*SelectedNode) []int32 {
	groups := newUnionFind(len(nodes))

	// one map and one buffer for the whole resolution: the map is cleared between the merge keys,
	// and the buffer is reused for every composite value. Looking up a []byte converted to string
	// doesn't allocate (the compiler optimizes this exact form), only the insert does.
	firstWithValue := make(map[string]int, len(nodes))
	var value []byte
	for _, key := range g.keys {
		clear(firstWithValue)
		// nodes with the same composite value are merged. It's enough to merge them with the first
		// node of the value, as merging is transitive.
		for ix, node := range nodes {
			if node.ID.IsZero() {
				// unknown node (eg. placeholder of a missing piece), it shouldn't be part of any group
				continue
			}
			var ok bool
			value, ok = key.appendValue(value[:0], node)
			if !ok {
				continue
			}
			if first, found := firstWithValue[string(value)]; found {
				groups.union(ix, first)
			} else {
				firstWithValue[string(value)] = ix
			}
		}
	}

	ids := make([]int32, len(nodes))
	for ix, node := range nodes {
		if node.ID.IsZero() {
			ids[ix] = -1
			continue
		}
		ids[ix] = int32(groups.find(ix))
	}
	return ids
}

// unionFind is a disjoint-set datastructure, used to calculate the connected components (groups) of
// nodes.
type unionFind struct {
	parent []int32
	size   []int32
}

func newUnionFind(n int) *unionFind {
	u := &unionFind{
		parent: make([]int32, n),
		size:   make([]int32, n),
	}
	for ix := range u.parent {
		u.parent[ix] = int32(ix)
		u.size[ix] = 1
	}
	return u
}

// find returns the representative element of the set, which contains x.
func (u *unionFind) find(x int) int {
	current := int32(x)
	for u.parent[current] != current {
		// path halving: keeps the tree flat without recursion
		u.parent[current] = u.parent[u.parent[current]]
		current = u.parent[current]
	}
	return int(current)
}

// union merges the sets of a and b.
func (u *unionFind) union(a, b int) {
	rootA, rootB := int32(u.find(a)), int32(u.find(b))
	if rootA == rootB {
		return
	}
	// union by size, to keep the tree shallow
	if u.size[rootA] < u.size[rootB] {
		rootA, rootB = rootB, rootA
	}
	u.parent[rootB] = rootA
	u.size[rootA] += u.size[rootB]
}
