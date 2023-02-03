// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// ErrEmptyQueue is used to indicate that the queue is empty.
var ErrEmptyQueue = errs.Class("empty audit queue")

// VerifyQueue controls manipulation of a database-based queue of segments to be
// verified; that is, segments chosen at random from all segments on the
// satellite, for which workers should perform audits. We will try to download a
// stripe of data across all pieces in the segment and ensure that all pieces
// conform to the same polynomial.
type VerifyQueue interface {
	Push(ctx context.Context, segments []Segment, maxBatchSize int) (err error)
	Next(ctx context.Context) (Segment, error)
}

// ReverifyQueue controls manipulation of a queue of pieces to be _re_verified;
// that is, a node timed out when we requested an audit of the piece, and now
// we need to follow up with that node until we get a proper answer to the
// audit. (Or until we try too many times, and disqualify the node.)
type ReverifyQueue interface {
	Insert(ctx context.Context, piece *PieceLocator) (err error)
	GetNextJob(ctx context.Context, retryInterval time.Duration) (job *ReverificationJob, err error)
	Remove(ctx context.Context, piece *PieceLocator) (wasDeleted bool, err error)
	GetByNodeID(ctx context.Context, nodeID storj.NodeID) (audit *ReverificationJob, err error)
	GetAllContainedNodes(ctx context.Context) ([]storj.NodeID, error)
}

// ByStreamIDAndPosition allows sorting of a slice of segments by stream ID and position.
type ByStreamIDAndPosition []Segment

func (b ByStreamIDAndPosition) Len() int {
	return len(b)
}

func (b ByStreamIDAndPosition) Less(i, j int) bool {
	comparison := b[i].StreamID.Compare(b[j].StreamID)
	if comparison < 0 {
		return true
	}
	if comparison > 0 {
		return false
	}
	return b[i].Position.Less(b[j].Position)
}

func (b ByStreamIDAndPosition) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}
