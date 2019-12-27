// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metrics

import (
	"context"

	"github.com/gogo/protobuf/proto"

	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
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
func (counter *Counter) Object(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	streamMeta := &pb.StreamMeta{}
	err = proto.Unmarshal(pointer.Metadata, streamMeta)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	if streamMeta.NumberOfSegments == 1 && pointer.Type == pb.Pointer_INLINE {
		counter.Inline++
	} else {
		counter.RemoteDependent++
	}
	counter.Total++

	return nil
}

// RemoteSegment returns nil because counter does not interact with remote segments this way for now.
func (counter *Counter) RemoteSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return nil
}

// InlineSegment returns nil because counter does not interact with inline segments this way for now.
func (counter *Counter) InlineSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return nil
}
