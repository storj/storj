// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metrics

import (
	"context"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase/metaloop"
)

// Counter implements the metainfo loop observer interface for data science metrics collection.
//
// architecture: Observer
type Counter struct {
	ObjectCount     int64
	RemoteDependent int64

	checkObjectRemoteness uuid.UUID
}

// NewCounter instantiates a new counter to be subscribed to the metainfo loop.
func NewCounter() *Counter {
	return &Counter{}
}

// LoopStarted is called at each start of a loop.
func (counter *Counter) LoopStarted(context.Context, metaloop.LoopInfo) (err error) {
	return nil
}

// Object increments the count for total objects and for inline objects in case the object has no segments.
func (counter *Counter) Object(ctx context.Context, object *metaloop.Object) (err error) {
	defer mon.Task()(&ctx)(&err)

	counter.ObjectCount++
	counter.checkObjectRemoteness = object.StreamID
	return nil
}

// RemoteSegment increments the count for objects with remote segments.
func (counter *Counter) RemoteSegment(ctx context.Context, segment *metaloop.Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	if counter.checkObjectRemoteness == segment.StreamID {
		counter.RemoteDependent++
		// we need to count this only once
		counter.checkObjectRemoteness = uuid.UUID{}
	}
	return nil
}

// InlineSegment increments the count for inline objects.
func (counter *Counter) InlineSegment(ctx context.Context, segment *metaloop.Segment) (err error) {
	return nil
}

// InlineObjectCount returns the count of objects that are inline only.
func (counter *Counter) InlineObjectCount() int64 {
	return counter.ObjectCount - counter.RemoteDependent
}
