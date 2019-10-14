// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metrics

import (
	"context"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc/rpcstatus"
	"storj.io/storj/satellite/metainfo"
)

// counter implements the metainfo loop observer interface for data science metrics collection.
//
// architecture: Observer
type counter struct {
	remoteDependentObjectCount int64
	inlineObjectCount          int64
	totalObjectCount           int64
}

// newCounter instantiates a new counter to be subscribed to the metainfo loop.
func newCounter() *counter {
	return &counter{}
}

// Object increments counts for inline objects and remote dependent objects.
func (counter *counter) Object(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	streamMeta := &pb.StreamMeta{}
	err = proto.Unmarshal(pointer.Metadata, streamMeta)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	if streamMeta.NumberOfSegments > 1 {
		counter.remoteDependentObjectCount++
	} else {
		counter.inlineObjectCount++
	}
	counter.totalObjectCount++

	return nil
}

// RemoteSegment returns nil because counter does not interact with remote segments this way for now.
func (counter *counter) RemoteSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return nil
}

// InlineSegment returns nil because counter does not interact with inline segments this way for now.
func (counter *counter) InlineSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return nil
}
