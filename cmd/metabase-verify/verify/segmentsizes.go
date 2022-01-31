// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package verify

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase/segmentloop"
)

// SegmentSizes verifies segments table plain_offset and plain_size.
type SegmentSizes struct {
	Log *zap.Logger

	segmentState
}

type segmentState struct {
	StreamID uuid.UUID

	ExpectedOffset int64
}

// LoopStarted is called at each start of a loop.
func (verify *SegmentSizes) LoopStarted(ctx context.Context, info segmentloop.LoopInfo) (err error) {
	return nil
}

// RemoteSegment implements the Observer interface.
func (verify *SegmentSizes) RemoteSegment(ctx context.Context, seg *segmentloop.Segment) error {
	return verify.advanceSegment(ctx, seg)
}

// InlineSegment implements the Observer interface.
func (verify *SegmentSizes) InlineSegment(ctx context.Context, seg *segmentloop.Segment) error {
	return verify.advanceSegment(ctx, seg)
}

func (verify *SegmentSizes) advanceSegment(ctx context.Context, seg *segmentloop.Segment) error {
	if verify.segmentState.StreamID != seg.StreamID {
		verify.segmentState = segmentState{
			StreamID: seg.StreamID,
		}
	}

	if seg.PlainSize > seg.EncryptedSize {
		verify.Log.Error("plain size larger than encrypted size",
			zap.Any("stream_id", seg.StreamID.String()),
			zap.Any("position", seg.Position),

			zap.Int32("plain size", seg.PlainSize),
			zap.Int32("encrypted size", seg.EncryptedSize))
	}

	if verify.ExpectedOffset != seg.PlainOffset {
		verify.Log.Error("invalid offset",
			zap.Any("stream_id", seg.StreamID.String()),
			zap.Any("position", seg.Position),

			zap.Int64("expected", verify.ExpectedOffset),
			zap.Int64("actual", seg.PlainOffset))
	}
	verify.ExpectedOffset += int64(seg.PlainSize)

	return nil
}
