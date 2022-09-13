// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"sync/atomic"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// VerifyPieces defines how many pieces we check per segment.
const VerifyPieces = 3

// ConcurrentRequests defines how many concurrent requests we do to the storagenodes.
const ConcurrentRequests = 10000

// Service implements segment verification logic.
type Service struct {
	log *zap.Logger

	PriorityNodes NodeAliasSet
	OfflineNodes  NodeAliasSet
}

// NewService returns a new service for verifying segments.
func NewService(log *zap.Logger) *Service {
	return &Service{
		log: log,

		PriorityNodes: NodeAliasSet{},
		OfflineNodes:  NodeAliasSet{},
	}
}

// Segment contains minimal information necessary for verifying a single Segment.
type Segment struct {
	StreamID uuid.UUID
	Position metabase.SegmentPosition
	Pieces   []metabase.AliasPiece

	Status Status
}

// Status contains the statistics about the segment.
type Status struct {
	Retry    int32
	Found    int32
	NotFound int32
}

// MarkFound moves a retry token from retry to found.
func (status *Status) MarkFound() {
	atomic.AddInt32(&status.Retry, -1)
	atomic.AddInt32(&status.Found, 1)
}

// MarkNotFound moves a retry token from retry to not found.
func (status *Status) MarkNotFound() {
	atomic.AddInt32(&status.Retry, -1)
	atomic.AddInt32(&status.NotFound, 1)
}

// Batch is a list of segments to be verified on a single node.
type Batch struct {
	Alias metabase.NodeAlias
	Items []*Segment
}

// Len returns the length of the batch.
func (b *Batch) Len() int { return len(b.Items) }
