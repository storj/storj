// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"context"
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// MockRepairQueue helps testing RepairQueue.
type MockRepairQueue struct {
	Segments []*InjuredSegment
}

// Insert implements RepairQueue.
func (m *MockRepairQueue) Insert(ctx context.Context, s *InjuredSegment) (alreadyInserted bool, err error) {
	for _, alreadyAdded := range m.Segments {
		if alreadyAdded.StreamID == s.StreamID {
			return true, nil
		}
	}
	m.Segments = append(m.Segments, s)
	return false, nil
}

// InsertBatch implements RepairQueue.
func (m *MockRepairQueue) InsertBatch(ctx context.Context, segments []*InjuredSegment) (newlyInsertedSegments []*InjuredSegment, err error) {
	for _, segment := range segments {
		inserted, err := m.Insert(ctx, segment)
		if err != nil {
			return nil, err
		}
		if inserted {
			newlyInsertedSegments = append(newlyInsertedSegments, segment)
		}
	}
	return newlyInsertedSegments, nil
}

// Select implements RepairQueue.
func (m *MockRepairQueue) Select(context.Context, int, []storj.PlacementConstraint, []storj.PlacementConstraint) ([]InjuredSegment, error) {
	panic("implement me")
}

// Release implements RepairQueue.
func (m *MockRepairQueue) Release(ctx context.Context, s InjuredSegment, repaired bool) error {
	panic("implement me")
}

// Delete implements RepairQueue.
func (m *MockRepairQueue) Delete(ctx context.Context, s InjuredSegment) error {
	panic("implement me")
}

// Clean implements RepairQueue.
func (m *MockRepairQueue) Clean(ctx context.Context, before time.Time) (deleted int64, err error) {
	panic("implement me")
}

// SelectN implements RepairQueue.
func (m *MockRepairQueue) SelectN(ctx context.Context, limit int) ([]InjuredSegment, error) {
	panic("implement me")
}

// Count implements RepairQueue.
func (m *MockRepairQueue) Count(ctx context.Context) (count int, err error) {
	panic("implement me")
}

// TestingSetAttemptedTime implements RepairQueue.
func (m *MockRepairQueue) TestingSetAttemptedTime(ctx context.Context, placement storj.PlacementConstraint, streamID uuid.UUID, position metabase.SegmentPosition, t time.Time) (rowsAffected int64, err error) {
	panic("implement me")
}

// TestingSetUpdatedTime implements RepairQueue.
func (m *MockRepairQueue) TestingSetUpdatedTime(ctx context.Context, placement storj.PlacementConstraint, streamID uuid.UUID, position metabase.SegmentPosition, t time.Time) (rowsAffected int64, err error) {
	panic("implement me")
}

// Stat implements RepairQueue.
func (m *MockRepairQueue) Stat(ctx context.Context) ([]Stat, error) {
	panic("implement me")
}

var _ RepairQueue = &MockRepairQueue{}
