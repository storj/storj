// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package repair

import (
	"go.uber.org/zap/zapcore"

	"storj.io/storj/private/intset"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/shared/location"
)

// PiecesCheckResult contains all necessary aggregate information about the state of pieces in a
// segment. The node that should be holding each piece is evaluated to see if it is online and
// whether it is in a clumped IP network, in an excluded country, or out of placement for the
// segment.
type PiecesCheckResult struct {
	// Missing is a set of Piece Numbers which are to be considered as lost and irretrievable.
	// (They reside on offline/disqualified/unknown nodes.)
	Missing intset.Set
	// Retrievable contains all Piece Numbers that are retrievable; that is, all piece numbers
	// from the segment that are NOT in Missing.
	Retrievable intset.Set

	// Suspended is a set of Piece Numbers which reside on nodes which are suspended.
	Suspended intset.Set
	// Clumped is a set of Piece Numbers which are to be considered unhealthy because of IP
	// clumping. (If DoDeclumping is disabled, this set will be empty.)
	Clumped intset.Set
	// Exiting is a set of Piece Numbers which are considered unhealthy because the node on
	// which they reside has initiated graceful exit.
	Exiting intset.Set
	// OutOfPlacement is a set of Piece Numbers which are unhealthy because of placement rules.
	// (If DoPlacementCheck is disabled, this set will be empty.)
	OutOfPlacement intset.Set
	// InExcludedCountry is a set of Piece Numbers which are unhealthy because they are in
	// Excluded countries.
	InExcludedCountry intset.Set

	// ForcingRepair is the set of pieces which force a repair operation for this segment (that
	// includes, currently, only pieces in OutOfPlacement).
	ForcingRepair intset.Set
	// Unhealthy contains all Piece Numbers which are in Missing OR Suspended OR Clumped OR
	// Exiting OR OutOfPlacement OR InExcludedCountry.
	Unhealthy intset.Set
	// UnhealthyRetrievable is the set of pieces that are "unhealthy-but-retrievable". That is,
	// pieces that are in Unhealthy AND Retrievable.
	UnhealthyRetrievable intset.Set
	// Healthy contains all Piece Numbers from the segment which are not in Unhealthy.
	// (Equivalently: all Piece Numbers from the segment which are NOT in Missing OR
	// Suspended OR Clumped OR Exiting OR OutOfPlacement OR InExcludedCountry).
	Healthy intset.Set
}

// ClassifySegmentPieces classifies the pieces of a segment into the categories
// represented by a PiecesCheckResult. Pieces may be put into multiple
// categories.
func ClassifySegmentPieces(pieces metabase.Pieces, nodes []nodeselection.SelectedNode, excludedCountryCodes map[location.CountryCode]struct{},
	doPlacementCheck, doDeclumping bool, placement nodeselection.Placement) (result PiecesCheckResult) {

	maxPieceNum := 0
	for _, piece := range pieces {
		if int(piece.Number) > maxPieceNum {
			maxPieceNum = int(piece.Number)
		}
	}
	maxPieceNum++

	// check excluded countries and remove online nodes from missing pieces
	result.Missing = intset.NewSet(maxPieceNum)
	result.Suspended = intset.NewSet(maxPieceNum)
	result.Exiting = intset.NewSet(maxPieceNum)
	result.Retrievable = intset.NewSet(maxPieceNum)
	result.InExcludedCountry = intset.NewSet(maxPieceNum)
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

	if doDeclumping && placement.Invariant != nil {
		result.Clumped = placement.Invariant(pieces, nodes)
	}

	if doPlacementCheck {
		// mark all pieces that are out of placement.

		result.OutOfPlacement = intset.NewSet(maxPieceNum)
		for index, nodeRecord := range nodes {
			if nodeRecord.ID.IsZero() {
				continue
			}
			if placement.NodeFilter == nil || placement.NodeFilter.Match(&nodeRecord) {
				continue
			}
			pieceNum := pieces[index].Number
			result.OutOfPlacement.Include(int(pieceNum))
		}
	}

	// ForcingRepair = OutOfPlacement only, for now
	result.ForcingRepair = intset.NewSet(maxPieceNum)
	result.ForcingRepair.Add(result.OutOfPlacement)

	// Unhealthy = Missing OR Suspended OR Clumped OR Exiting OR OutOfPlacement OR InExcludedCountry
	result.Unhealthy = intset.NewSet(maxPieceNum)
	result.Unhealthy.Add(
		result.Missing,
		result.Suspended,
		result.Clumped,
		result.Exiting,
		result.OutOfPlacement,
		result.InExcludedCountry,
	)

	// UnhealthyRetrievable = Unhealthy AND Retrievable
	// Healthy = NOT Unhealthy
	result.UnhealthyRetrievable = intset.NewSet(maxPieceNum)
	result.Healthy = intset.NewSet(maxPieceNum)
	for _, piece := range pieces {
		if !result.Unhealthy.Contains(int(piece.Number)) {
			result.Healthy.Include(int(piece.Number))
		} else if result.Retrievable.Contains(int(piece.Number)) {
			result.UnhealthyRetrievable.Include(int(piece.Number))
		}
	}

	return result
}

// MarshalLogObject implements zapcore.ObjectMarshaler.
func (result PiecesCheckResult) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt("Missing", result.Missing.Count())
	enc.AddInt("Retrievable", result.Retrievable.Count())
	enc.AddInt("Suspended", result.Suspended.Count())
	enc.AddInt("Clumped", result.Clumped.Count())
	enc.AddInt("Exiting", result.Exiting.Count())
	enc.AddInt("ForcingRepair", result.ForcingRepair.Count())
	enc.AddInt("Healthy", result.Healthy.Count())
	enc.AddInt("OutOfPlacement", result.OutOfPlacement.Count())
	enc.AddInt("Retrievable", result.Retrievable.Count())
	enc.AddInt("Suspended", result.Suspended.Count())
	enc.AddInt("Unhealthy", result.Unhealthy.Count())
	enc.AddInt("UnhealthyRetrievable", result.UnhealthyRetrievable.Count())
	return nil
}
