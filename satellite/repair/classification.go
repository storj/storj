// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package repair

import (
	"golang.org/x/exp/maps"

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
	Missing map[uint16]struct{}
	// Retrievable contains all Piece Numbers that are retrievable; that is, all piece numbers
	// from the segment that are NOT in Missing.
	Retrievable map[uint16]struct{}

	// Suspended is a set of Piece Numbers which reside on nodes which are suspended.
	Suspended map[uint16]struct{}
	// Clumped is a set of Piece Numbers which are to be considered unhealthy because of IP
	// clumping. (If DoDeclumping is disabled, this set will be empty.)
	Clumped map[uint16]struct{}
	// Exiting is a set of Piece Numbers which are considered unhealthy because the node on
	// which they reside has initiated graceful exit.
	Exiting map[uint16]struct{}
	// OutOfPlacement is a set of Piece Numbers which are unhealthy because of placement rules.
	// (If DoPlacementCheck is disabled, this set will be empty.)
	OutOfPlacement map[uint16]struct{}
	// InExcludedCountry is a set of Piece Numbers which are unhealthy because they are in
	// Excluded countries.
	InExcludedCountry map[uint16]struct{}

	// ForcingRepair is the set of pieces which force a repair operation for this segment (that
	// includes, currently, only pieces in OutOfPlacement).
	ForcingRepair map[uint16]struct{}
	// Unhealthy contains all Piece Numbers which are in Missing OR Suspended OR Clumped OR
	// Exiting OR OutOfPlacement OR InExcludedCountry.
	Unhealthy map[uint16]struct{}
	// UnhealthyRetrievable is the set of pieces that are "unhealthy-but-retrievable". That is,
	// pieces that are in Unhealthy AND Retrievable.
	UnhealthyRetrievable map[uint16]struct{}
	// Healthy contains all Piece Numbers from the segment which are not in Unhealthy.
	// (Equivalently: all Piece Numbers from the segment which are NOT in Missing OR
	// Suspended OR Clumped OR Exiting OR OutOfPlacement OR InExcludedCountry).
	Healthy map[uint16]struct{}
}

// ClassifySegmentPieces classifies the pieces of a segment into the categories
// represented by a PiecesCheckResult. Pieces may be put into multiple
// categories.
func ClassifySegmentPieces(pieces metabase.Pieces, nodes []nodeselection.SelectedNode, excludedCountryCodes map[location.CountryCode]struct{}, doPlacementCheck, doDeclumping bool, filter nodeselection.NodeFilter) (result PiecesCheckResult) {
	result.ExcludeNodeIDs = make([]storj.NodeID, len(pieces))
	for i, p := range pieces {
		result.ExcludeNodeIDs[i] = p.StorageNode
	}

	// check excluded countries and remove online nodes from missing pieces
	result.Missing = make(map[uint16]struct{})
	result.Suspended = make(map[uint16]struct{})
	result.Exiting = make(map[uint16]struct{})
	result.Retrievable = make(map[uint16]struct{})
	result.InExcludedCountry = make(map[uint16]struct{})
	for index, nodeRecord := range nodes {
		pieceNum := pieces[index].Number

		if !nodeRecord.ID.IsZero() && pieces[index].StorageNode != nodeRecord.ID {
			panic("wrong order")
		}

		if nodeRecord.ID.IsZero() || !nodeRecord.Online {
			// node ID was not found, or the node is disqualified or exited,
			// or it is offline
			result.Missing[pieceNum] = struct{}{}
		} else {
			// node is expected to be online and receiving requests.
			result.Retrievable[pieceNum] = struct{}{}
		}

		if nodeRecord.Suspended {
			result.Suspended[pieceNum] = struct{}{}
		}
		if nodeRecord.Exiting {
			result.Exiting[pieceNum] = struct{}{}
		}

		if _, excluded := excludedCountryCodes[nodeRecord.CountryCode]; excluded {
			result.InExcludedCountry[pieceNum] = struct{}{}
		}
	}

	if doDeclumping && nodeselection.GetAnnotation(filter, nodeselection.AutoExcludeSubnet) != nodeselection.AutoExcludeSubnetOFF {
		// if multiple pieces are on the same last_net, keep only the first one. The rest are
		// to be considered retrievable but unhealthy.

		lastNets := make(map[string]struct{}, len(pieces))
		result.Clumped = make(map[uint16]struct{})

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
					result.Clumped[pieceNum] = struct{}{}
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

		result.OutOfPlacement = make(map[uint16]struct{})
		for index, nodeRecord := range nodes {
			if nodeRecord.ID.IsZero() {
				continue
			}
			if filter.Match(&nodeRecord) {
				continue
			}
			pieceNum := pieces[index].Number
			result.OutOfPlacement[pieceNum] = struct{}{}
		}
	}

	// ForcingRepair = OutOfPlacement only, for now
	result.ForcingRepair = make(map[uint16]struct{})
	maps.Copy(result.ForcingRepair, result.OutOfPlacement)

	// Unhealthy = Missing OR Suspended OR Clumped OR OutOfPlacement OR InExcludedCountry
	result.Unhealthy = make(map[uint16]struct{})
	maps.Copy(result.Unhealthy, result.Missing)
	maps.Copy(result.Unhealthy, result.Suspended)
	maps.Copy(result.Unhealthy, result.Clumped)
	maps.Copy(result.Unhealthy, result.Exiting)
	maps.Copy(result.Unhealthy, result.OutOfPlacement)
	maps.Copy(result.Unhealthy, result.InExcludedCountry)

	// UnhealthyRetrievable = Unhealthy AND Retrievable
	result.UnhealthyRetrievable = make(map[uint16]struct{})
	for pieceNum := range result.Unhealthy {
		if _, isRetrievable := result.Retrievable[pieceNum]; isRetrievable {
			result.UnhealthyRetrievable[pieceNum] = struct{}{}
		}
	}

	// Healthy = NOT Unhealthy
	result.Healthy = make(map[uint16]struct{})
	for _, piece := range pieces {
		if _, found := result.Unhealthy[piece.Number]; !found {
			result.Healthy[piece.Number] = struct{}{}
		}
	}
	return result
}
