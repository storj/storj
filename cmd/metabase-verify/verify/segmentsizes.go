// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package verify

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metaloop"
)

// SegmentSizes verifies segments table plain_offset and plain_size.
type SegmentSizes struct {
	Log *zap.Logger

	segmentState
}

type segmentState struct {
	Current metabase.ObjectStream
	Status  metabase.ObjectStatus

	ExpectedOffset int64
}

// Object implements the Observer interface.
func (verify *SegmentSizes) Object(ctx context.Context, obj *metaloop.Object) error {
	verify.segmentState = segmentState{
		Current: obj.ObjectStream,
		Status:  obj.Status,
	}
	return nil
}

// LoopStarted is called at each start of a loop.
func (verify *SegmentSizes) LoopStarted(ctx context.Context, info metaloop.LoopInfo) (err error) {
	return nil
}

// RemoteSegment implements the Observer interface.
func (verify *SegmentSizes) RemoteSegment(ctx context.Context, seg *metaloop.Segment) error {
	return verify.advanceSegment(ctx, seg)
}

// InlineSegment implements the Observer interface.
func (verify *SegmentSizes) InlineSegment(ctx context.Context, seg *metaloop.Segment) error {
	return verify.advanceSegment(ctx, seg)
}

func (verify *SegmentSizes) advanceSegment(ctx context.Context, seg *metaloop.Segment) error {
	if seg.PlainSize > seg.EncryptedSize {
		verify.Log.Error("plain size larger than encrypted size",
			zap.Any("object", formatObject(verify.Current)),
			zap.Any("position", seg.Position),

			zap.Int32("plain size", seg.PlainSize),
			zap.Int32("encrypted size", seg.EncryptedSize))
	}

	if verify.Status != metabase.Committed {
		return nil
	}

	if verify.ExpectedOffset != seg.PlainOffset {
		verify.Log.Error("invalid offset",
			zap.Any("object", formatObject(verify.Current)),
			zap.Any("position", seg.Position),

			zap.Int64("expected", verify.ExpectedOffset),
			zap.Int64("actual", seg.PlainOffset))
	}
	verify.ExpectedOffset += int64(seg.PlainSize)

	return nil
}
