// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package repair

import (
	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
)

// PiecesCheckResult contains all necessary aggregate information about the state of pieces in a
// segment. The node that should be holding each piece is evaluated to see if it is online and
// whether it is in a clumped IP network, in an excluded country, or out of placement for the
// segment.
type PiecesCheckResult struct {
	// ExcludeNodeIDs is a list of all node IDs holding pieces of this segment.
	ExcludeNodeIDs []storj.NodeID

	// Missing is a set of Piece Numbers which are to be considered as lost and irretrievable.
	// (They reside on offline/disqualified/unknown nodes.)
	Missing metabase.IntSet
	// Retrievable contains all Piece Numbers that are retrievable; that is, all piece numbers
	// from the segment that are NOT in Missing.
	Retrievable metabase.IntSet

	// Suspended is a set of Piece Numbers which reside on nodes which are suspended.
	Suspended metabase.IntSet
	// Clumped is a set of Piece Numbers which are to be considered unhealthy because of IP
	// clumping. (If DoDeclumping is disabled, this set will be empty.)
	Clumped metabase.IntSet
	// Exiting is a set of Piece Numbers which are considered unhealthy because the node on
	// which they reside has initiated graceful exit.
	Exiting metabase.IntSet
	// OutOfPlacement is a set of Piece Numbers which are unhealthy because of placement rules.
	// (If DoPlacementCheck is disabled, this set will be empty.)
	OutOfPlacement metabase.IntSet
	// InExcludedCountry is a set of Piece Numbers which are unhealthy because they are in
	// Excluded countries.
	InExcludedCountry metabase.IntSet

	// ForcingRepair is the set of pieces which force a repair operation for this segment (that
	// includes, currently, only pieces in OutOfPlacement).
	ForcingRepair metabase.IntSet
	// Unhealthy contains all Piece Numbers which are in Missing OR Suspended OR Clumped OR
	// Exiting OR OutOfPlacement OR InExcludedCountry.
	Unhealthy metabase.IntSet
	// UnhealthyRetrievable is the set of pieces that are "unhealthy-but-retrievable". That is,
	// pieces that are in Unhealthy AND Retrievable.
	UnhealthyRetrievable metabase.IntSet
	// Healthy contains all Piece Numbers from the segment which are not in Unhealthy.
	// (Equivalently: all Piece Numbers from the segment which are NOT in Missing OR
	// Suspended OR Clumped OR Exiting OR OutOfPlacement OR InExcludedCountry).
	Healthy metabase.IntSet
}

// ClassifySegmentPieces classifies the pieces of a segment into the categories
// represented by a PiecesCheckResult. Pieces may be put into multiple
// categories.
func ClassifySegmentPieces(pieces metabase.Pieces, nodes []nodeselection.SelectedNode, excludedCountryCodes map[location.CountryCode]struct{},
	doPlacementCheck, doDeclumping bool, filter nodeselection.NodeFilter, excludeNodeIDs []storj.NodeID) (result PiecesCheckResult) {
	result.ExcludeNodeIDs = excludeNodeIDs

	maxPieceNum := 0
	for _, piece := range pieces {
		if int(piece.Number) > maxPieceNum {
			maxPieceNum = int(piece.Number)
		}
	}
	maxPieceNum++

	// check excluded countries and remove online nodes from missing pieces
	result.Missing = metabase.NewIntSet(maxPieceNum)
	result.Suspended = metabase.NewIntSet(maxPieceNum)
	result.Exiting = metabase.NewIntSet(maxPieceNum)
	result.Retrievable = metabase.NewIntSet(maxPieceNum)
	result.InExcludedCountry = metabase.NewIntSet(maxPieceNum)
	for index, nodeRecord := range nodes {
		pieceNum := pieces[index].Number

		if !nodeRecord.ID.IsZero() && pieces[index].StorageNode != nodeRecord.ID {
			panic("wrong order")
		}

		if nodeRecord.ID.IsZero() || !nodeRecord.Online {
			// node ID was not found, or the node is disqualified or exited,
			// or it is offline
			result.Missing.Include(int(pieceNum))
		} else {
			// node is expected to be online and receiving requests.
			result.Retrievable.Include(int(pieceNum))
		}

		if nodeRecord.Suspended {
			result.Suspended.Include(int(pieceNum))
		}
		if nodeRecord.Exiting {
			result.Exiting.Include(int(pieceNum))
		}

		if _, excluded := excludedCountryCodes[nodeRecord.CountryCode]; excluded {
			result.InExcludedCountry.Include(int(pieceNum))
		}
	}

	if doDeclumping && nodeselection.GetAnnotation(filter, nodeselection.AutoExcludeSubnet) != nodeselection.AutoExcludeSubnetOFF {
		// if multiple pieces are on the same last_net, keep only the first one. The rest are
		// to be considered retrievable but unhealthy.

		lastNets := make(map[string]struct{}, len(pieces))
		result.Clumped = metabase.NewIntSet(maxPieceNum)

		collectClumpedPieces := func(onlineness bool) {
			for index, nodeRecord := range nodes {
				if nodeRecord.Online != onlineness {
					continue
				}
				if nodeRecord.LastNet == "" {
					continue
				}
				pieceNum := pieces[index].Number
				_, ok := lastNets[nodeRecord.LastNet]
				if ok {
					// this LastNet was already seen
					result.Clumped.Include(int(pieceNum))
				} else {
					// add to the list of seen LastNets
					lastNets[nodeRecord.LastNet] = struct{}{}
				}
			}
		}
		// go over online nodes first, so that if we have to remove clumped pieces, we prefer
		// to remove offline ones over online ones.
		collectClumpedPieces(true)
		collectClumpedPieces(false)
	}

	if doPlacementCheck {
		// mark all pieces that are out of placement.

		result.OutOfPlacement = metabase.NewIntSet(maxPieceNum)
		for index, nodeRecord := range nodes {
			if nodeRecord.ID.IsZero() {
				continue
			}
			if filter.Match(&nodeRecord) {
				continue
			}
			pieceNum := pieces[index].Number
			result.OutOfPlacement.Include(int(pieceNum))
		}
	}

	// ForcingRepair = OutOfPlacement only, for now
	result.ForcingRepair = metabase.CopyIntSet(metabase.NewIntSet(maxPieceNum),
		result.OutOfPlacement,
	)

	// Unhealthy = Missing OR Suspended OR Clumped OR Exiting OR OutOfPlacement OR InExcludedCountry
	result.Unhealthy = metabase.CopyIntSet(metabase.NewIntSet(maxPieceNum),
		result.Missing,
		result.Suspended,
		result.Clumped,
		result.Exiting,
		result.OutOfPlacement,
		result.InExcludedCountry,
	)

	// UnhealthyRetrievable = Unhealthy AND Retrievable
	// Healthy = NOT Unhealthy
	result.UnhealthyRetrievable = metabase.NewIntSet(maxPieceNum)
	result.Healthy = metabase.NewIntSet(maxPieceNum)
	for _, piece := range pieces {
		if !result.Unhealthy.Contains(int(piece.Number)) {
			result.Healthy.Include(int(piece.Number))
		} else if result.Retrievable.Contains(int(piece.Number)) {
			result.UnhealthyRetrievable.Include(int(piece.Number))
		}
	}

	return result
}
