// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metrics

import (
	"context"

	"storj.io/storj/satellite/metainfo"
)

// Counter implements the metainfo loop observer interface for data science metrics collection.
//
// architecture: Observer
type Counter struct {
	RemoteDependent int64
	Inline          int64
	Total           int64
}

// NewCounter instantiates a new counter to be subscribed to the metainfo loop.
func NewCounter() *Counter {
	return &Counter{}
}

// Object increments counts for inline objects and remote dependent objects.
func (counter *Counter) Object(ctx context.Context, object *metainfo.Object) (err error) {
	if object.SegmentCount == 1 && object.LastSegment.Inline {
		counter.Inline++
	} else {
		counter.RemoteDependent++
	}
	counter.Total++

	return nil
}

// RemoteSegment returns nil because counter does not interact with remote segments this way for now.
func (counter *Counter) RemoteSegment(ctx context.Context, segment *metainfo.Segment) (err error) {
	return nil
}

// InlineSegment returns nil because counter does not interact with inline segments this way for now.
func (counter *Counter) InlineSegment(ctx context.Context, segment *metainfo.Segment) (err error) {
	return nil
}
