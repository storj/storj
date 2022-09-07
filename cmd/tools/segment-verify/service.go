// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// VerifyPieces defines how many pieces we check per segment.
const VerifyPieces = 3

// Service implements segment verification logic.
type Service struct {
	PriorityNodes NodeAliasSet
	OfflineNodes  NodeAliasSet
}

// NewService returns a new service for verifying segments.
func NewService() *Service {
	return &Service{
		PriorityNodes: NodeAliasSet{},
		OfflineNodes:  NodeAliasSet{},
	}
}

// Segment contains minimal information necessary for verifying a single Segment.
type Segment struct {
	StreamID uuid.UUID
	Position metabase.SegmentPosition
	Pieces   []metabase.AliasPiece
}

// Batch is a list of segments to be verified on a single node.
type Batch struct {
	Alias metabase.NodeAlias
	Items []*Segment
}

// Len returns the length of the batch.
func (b *Batch) Len() int { return len(b.Items) }
