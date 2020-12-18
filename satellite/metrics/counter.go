// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metrics

import (
	"context"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo"
)

// Counter implements the metainfo loop observer interface for data science metrics collection.
//
// architecture: Observer
type Counter struct {
	RemoteDependent int64
	Inline          int64
	Total           int64
	streamIDCursor  uuid.UUID
}

// NewCounter instantiates a new counter to be subscribed to the metainfo loop.
func NewCounter() *Counter {
	return &Counter{}
}

// Object increments the count for total objects and for inline objects in case the object has no segments.
func (counter *Counter) Object(ctx context.Context, object *metainfo.Object) (err error) {
	defer mon.Task()(&ctx)(&err)

	counter.Total++

	if object.SegmentCount == 0 {
		counter.Inline++
		return nil
	}

	if !counter.streamIDCursor.IsZero() {
		return Error.New("unexpected cursor: wants zero, got %s", counter.streamIDCursor.String())
	}

	counter.streamIDCursor = object.StreamID

	return nil
}

// RemoteSegment increments the count for objects with remote segments.
func (counter *Counter) RemoteSegment(ctx context.Context, segment *metainfo.Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	if counter.streamIDCursor.IsZero() {
		return nil
	}

	if counter.streamIDCursor != segment.StreamID {
		return Error.New("unexpected cursor: wants %s, got %s", segment.StreamID.String(), counter.streamIDCursor.String())
	}

	counter.RemoteDependent++

	// reset the cursor to ensure we don't count multi-segment objects more than once.
	counter.streamIDCursor = uuid.UUID{}

	return nil
}

// InlineSegment increments the count for inline objects.
func (counter *Counter) InlineSegment(ctx context.Context, segment *metainfo.Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	if counter.streamIDCursor.IsZero() {
		return nil
	}

	if counter.streamIDCursor != segment.StreamID {
		return Error.New("unexpected cursor: wants %s, got %s", segment.StreamID.String(), counter.streamIDCursor.String())
	}

	counter.Inline++

	// reset the cursor to ensure we don't count multi-segment objects more than once.
	counter.streamIDCursor = uuid.UUID{}

	return nil
}
