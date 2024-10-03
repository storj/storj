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

		maxPieceNum := 0
		for _, piece := range pieces {
			if int(piece.Number) > maxPieceNum {
				maxPieceNum = int(piece.Number)
			}
		}
		maxPieceNum++

		res := intset.NewSet(maxPieceNum)

		for index, nodeRecord := range nodes {
			attribute, ok := attr(nodeRecord).(string)
			if !ok || attribute == "" {
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

// ClumpingByAnyTag tries to limit the number of nodes with the same tag value.
func ClumpingByAnyTag(key string, maxAllowed int) Invariant {
	return ClumpingByAttribute(AnyNodeTagAttribute(key), maxAllowed)
}
