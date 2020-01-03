// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"context"

	"storj.io/common/pb"
)

// RepairQueue implements queueing for segments that need repairing.
// Implementation can be found at satellite/satellitedb/repairqueue.go.
//
// architecture: Database
type RepairQueue interface {
	// Insert adds an injured segment.
	Insert(ctx context.Context, s *pb.InjuredSegment) error
	// Select gets an injured segment.
	Select(ctx context.Context) (*pb.InjuredSegment, error)
	// Delete removes an injured segment.
	Delete(ctx context.Context, s *pb.InjuredSegment) error
	// SelectN lists limit amount of injured segments.
	SelectN(ctx context.Context, limit int) ([]pb.InjuredSegment, error)
	// Count counts the number of segments in the repair queue.
	Count(ctx context.Context) (count int, err error)
}
