// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metrics

import (
	"context"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase/segmentloop"
)

var (
	remoteSegmentFunc = mon.Task()
	inlineSegmentFunc = mon.Task()
)

// Counter implements the segment loop observer interface for data science metrics collection.
//
// architecture: Observer
type Counter struct {
	// number of objects that has at least one remote segment
	RemoteObjects int64
	// number of objects that has all inline segments
	InlineObjects int64

	// encrypted size
	TotalInlineBytes int64
	// encrypted size
	TotalRemoteBytes int64

	TotalInlineSegments int64
	TotalRemoteSegments int64

	lastStreamID uuid.UUID
	onlyInline   bool
}

// NewCounter instantiates a new counter to be subscribed to the metainfo loop.
func NewCounter() *Counter {
	return &Counter{
		onlyInline: true,
	}
}

// LoopStarted is called at each start of a loop.
func (counter *Counter) LoopStarted(context.Context, segmentloop.LoopInfo) (err error) {
	return nil
}

// RemoteSegment increments the count for objects with remote segments.
func (counter *Counter) RemoteSegment(ctx context.Context, segment *segmentloop.Segment) error {
	defer remoteSegmentFunc(&ctx)(nil) // method always returns nil

	counter.onlyInline = false

	counter.TotalRemoteBytes += int64(segment.EncryptedSize)
	counter.TotalRemoteSegments++

	if counter.lastStreamID.Compare(segment.StreamID) != 0 {
		counter.RemoteObjects++

		counter.lastStreamID = segment.StreamID
		counter.onlyInline = true
	}
	return nil
}

// InlineSegment increments the count for inline objects.
func (counter *Counter) InlineSegment(ctx context.Context, segment *segmentloop.Segment) error {
	defer inlineSegmentFunc(&ctx)(nil) // method always returns nil

	counter.TotalInlineBytes += int64(segment.EncryptedSize)
	counter.TotalInlineSegments++

	if counter.lastStreamID.Compare(segment.StreamID) != 0 {
		if counter.onlyInline {
			counter.InlineObjects++
		} else {
			counter.RemoteObjects++
		}

		counter.lastStreamID = segment.StreamID
		counter.onlyInline = true
	}
	return nil
}
