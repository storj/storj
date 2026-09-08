// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"storj.io/storj/private/intset"
	"storj.io/storj/satellite/metabase"
)

// Invariant checks the current placement, and identifies the pieces which should be moved.
// Used by repair jobs.
type Invariant func(pieces metabase.Pieces, nodes []SelectedNode) intset.Set

// FilterInvariant enables marking pieces as OOP (out of placement) based on a filter.
func FilterInvariant(filter NodeFilter) Invariant {
	return func(pieces metabase.Pieces, nodes []SelectedNode) intset.Set {
		res := createIntSet(pieces)
		for index, nodeRecord := range nodes {
			if !filter.Match(&nodeRecord) {
				pieceNum := pieces[index].Number
				res.Include(int(pieceNum))
			}
		}
		return res
	}
}

// CombinedInvariant combines multiple invariants into one, by taking the union of all sets of bad pieces.
func CombinedInvariant(invariants ...Invariant) Invariant {
	if len(invariants) == 0 {
		return AllGood()
	}
	return func(pieces metabase.Pieces, nodes []SelectedNode) intset.Set {
		res := invariants[0](pieces, nodes)
		for ix := 1; ix < len(invariants); ix++ {
			res.Add(invariants[ix](pieces, nodes))
		}
		return res
	}
}

// AllGood is an invariant, which accepts all piece sets as good.
func AllGood() Invariant {
	return func(pieces metabase.Pieces, nodes []SelectedNode) intset.Set {
		return intset.NewSet(0)
	}
}

// ClumpingByAttribute allows only one selected piece by attribute groups.
func ClumpingByAttribute(attr NodeAttribute, maxAllowed int) Invariant {
	return func(pieces metabase.Pieces, nodes []SelectedNode) intset.Set {
		return clumpingByAttribute(attr, maxAllowed, pieces, nodes)
	}
}

// ClumpingByGroup is same as ClumpingByAttribute, but for group attributes, which have to be
// resolved on the node set of the checked segment. Groups are compared by id, the labels of
// GroupAttribute.Resolve() are not needed here.
//
// Limitation: the groups are resolved with the nodes of the checked segment only, because Invariant
// has no access to the node cache. Two nodes are therefore seen as connected only if they share a
// merge key directly, or if every bridging node of the chain happens to hold a piece of this very
// segment. With ~80 pieces out of a network of tens of thousands of nodes that is rare, so in
// practice this degenerates to maxcontrol over the union of the individual merge keys: the
// transitive merging of group() essentially never triggers a repair here.
//
// This means the invariant is a best effort check and not an enforced bound. It under-detects, and
// for a control bound that is not the harmless direction - the pieces it misses are exactly the
// pre-existing correlated placements group() was introduced to prevent (uploaded before the tags or
// the config existed, or before the bridging node checked in). It cannot over-detect, so it never
// triggers a spurious repair, but that is a property of the failure mode, not a justification.
//
// The selector side has no such gap: AttributeGroupSelectorInit resolves on the full node set (see
// SelectorFromString), so newly uploaded segments do respect the transitive groups. Closing the gap
// here needs the group ids resolved once per loop from the full node set (a map[NodeID]int32
// refreshed like balancer.Invariant.Start() does with uploadCache.GetAllNodes()), which is a
// lifecycle Invariant doesn't have today: it is a plain func value on Placement, built at config
// parse time and called per segment by the repair checker. That is a separate change; this one is
// the first step, and until it lands, operators should read maxcontrol(group(...)) as "catches
// direct sharing" rather than as a guaranteed limit on pieces per operator.
//
// TestClumpingByGroupMissesTransitiveConnection pins the behaviour described here.
func ClumpingByGroup(group *GroupAttribute, maxAllowed int) Invariant {
	return func(pieces metabase.Pieces, nodes []SelectedNode) intset.Set {
		pointers := make([]*SelectedNode, len(nodes))
		for ix := range nodes {
			pointers[ix] = &nodes[ix]
		}

		res := createIntSet(pieces)
		usedGroups := make(map[int32]int, len(pieces))
		for index, id := range group.Groups(pointers) {
			if id < 0 {
				continue
			}
			pieceNum := pieces[index].Number
			count := usedGroups[id]
			if count >= maxAllowed {
				// this group was already seen, enough times
				res.Include(int(pieceNum))
			} else {
				// add to the list of seen groups
				usedGroups[id] = count + 1
			}
		}

		return res
	}
}

func clumpingByAttribute(attr NodeAttribute, maxAllowed int, pieces metabase.Pieces, nodes []SelectedNode) intset.Set {
	usedGroups := make(map[string]int, len(pieces))

	res := createIntSet(pieces)

	for index, nodeRecord := range nodes {
		attribute := attr(nodeRecord)
		if attribute == "" {
			continue
		}
		pieceNum := pieces[index].Number
		count := usedGroups[attribute]
		if count >= maxAllowed {
			// this group was already seen, enough times
			res.Include(int(pieceNum))
		} else {
			// add to the list of seen groups
			usedGroups[attribute] = count + 1
		}
	}

	return res
}

func createIntSet(pieces metabase.Pieces) intset.Set {
	maxPieceNum := 0
	for _, piece := range pieces {
		if int(piece.Number) > maxPieceNum {
			maxPieceNum = int(piece.Number)
		}
	}
	maxPieceNum++

	res := intset.NewSet(maxPieceNum)
	return res
}

// ClumpingByAnyTag tries to limit the number of nodes with the same tag value.
func ClumpingByAnyTag(key string, maxAllowed int) Invariant {
	return ClumpingByAttribute(AnyNodeTagAttribute(key), maxAllowed)
}
