// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package verify

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase/rangedloop"
)

type segmentState struct {
	StreamID uuid.UUID

	ExpectedOffset int64
}

// SegmentSizes verifies segments table plain_offset and plain_size.
type SegmentSizes struct {
	Log *zap.Logger

	mu sync.Mutex
	segmentState
}

// Start is called at the beginning of each segment loop.
func (verify *SegmentSizes) Start(context.Context, time.Time) error {
	return nil
}

// Fork creates a Partial to process a chunk of all the segments. It is
// called after Start. It is not called concurrently.
func (verify *SegmentSizes) Fork(context.Context) (rangedloop.Partial, error) {
	return verify, nil
}

// Join is called for each partial returned by Fork.
func (verify *SegmentSizes) Join(context.Context, rangedloop.Partial) error {
	return nil
}

// Finish is called after all segments are processed by all observers.
func (verify *SegmentSizes) Finish(context.Context) error {
	return nil
}

// Process is called repeatedly with batches of segments.
func (verify *SegmentSizes) Process(ctx context.Context, segments []rangedloop.Segment) error {
	verify.mu.Lock()
	defer verify.mu.Unlock()

	for _, segment := range segments {
		if err := verify.advanceSegment(ctx, segment); err != nil {
			return err
		}
	}
	return nil
}

func (verify *SegmentSizes) advanceSegment(ctx context.Context, seg rangedloop.Segment) error {
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
