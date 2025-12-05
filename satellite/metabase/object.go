// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// Object object metadata.
// TODO define separated struct.
type Object RawObject

// IsMigrated returns whether the object comes from PointerDB.
// Pointer objects are special that they are missing some information.
//
//   - TotalPlainSize = 0 and FixedSegmentSize = 0.
//   - Segment.PlainOffset = 0, Segment.PlainSize = 0
func (obj *Object) IsMigrated() bool {
	return obj.TotalPlainSize <= 0
}

// StreamVersionID returns byte representation of object stream version id.
func (obj *Object) StreamVersionID() StreamVersionID {
	return NewStreamVersionID(obj.Version, obj.StreamID)
}

// Segment segment metadata.
// TODO define separated struct.
type Segment RawSegment

// Inline returns true if segment is inline.
func (s Segment) Inline() bool {
	return s.Redundancy.IsZero() && len(s.Pieces) == 0
}

// Expired checks if segment is expired relative to now.
func (s Segment) Expired(now time.Time) bool {
	return s.ExpiresAt != nil && s.ExpiresAt.Before(now)
}

// PieceSize returns calculated piece size for segment.
func (s Segment) PieceSize() int64 {
	return s.Redundancy.PieceSize(int64(s.EncryptedSize))
}

// SegmentForRepair defines the segment data required for the repair functionality.
type SegmentForRepair struct {
	StreamID uuid.UUID
	Position SegmentPosition

	CreatedAt  time.Time // non-nillable
	RepairedAt *time.Time
	ExpiresAt  *time.Time

	RootPieceID   storj.PieceID
	EncryptedSize int32 // size of the whole segment (not a piece)
	Redundancy    storj.RedundancyScheme
	Pieces        Pieces
	Placement     storj.PlacementConstraint
}

// Inline returns true if segment is inline.
func (s SegmentForRepair) Inline() bool {
	return s.Redundancy.IsZero() && len(s.Pieces) == 0
}

// Expired checks if segment is expired relative to now.
func (s SegmentForRepair) Expired(now time.Time) bool {
	return s.ExpiresAt != nil && s.ExpiresAt.Before(now)
}

// PieceSize returns calculated piece size for segment.
func (s SegmentForRepair) PieceSize() int64 {
	return s.Redundancy.PieceSize(int64(s.EncryptedSize))
}

// SegmentForAudit defines the segment data required for the audit functionality.
type SegmentForAudit SegmentForRepair

// Expired checks if segment is expired relative to now.
func (s SegmentForAudit) Expired(now time.Time) bool {
	return s.ExpiresAt != nil && s.ExpiresAt.Before(now)
}

// PieceSize returns calculated piece size for segment.
func (s SegmentForAudit) PieceSize() int64 {
	return s.Redundancy.PieceSize(int64(s.EncryptedSize))
}
