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
