// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/dtypes"
	"storj.io/storj/pkg/dtypes/segments"
	"storj.io/storj/pkg/ranger"
)

type streamStore struct {
	store       segments.SegmentStore
	segmentSize int64
}

func NewStreams(store segments.SegmentStore, segmentSize int64) StreamStore {
	return &streamStore{store: store, segmentSize: segmentSize}
}

func (s *streamStore) Put(ctx context.Context, path dtypes.Path, data io.Reader,
	metadata []byte, expiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: break up data as it comes in into s.segmentSize length pieces, then
	// store the first piece at s0/<path>, second piece at s1/<path>, and the
	// *last* piece at l/<path>. Store the given metadata, along with the number
	// of segments, in a new protobuf, in the metadata of l/<path>.

	return s.store.Put(ctx, path, data, metadata, expiration)
}

func (s *streamStore) Get(ctx context.Context, path dtypes.Path) (
	rv ranger.Ranger, m dtypes.Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: return a ranger that knows what the overall size is (from l/<path>)
	// and then returns the appropriate data from segments s0/<path>, s1/<path>,
	// ..., l/<path>.

	rv, meta, err := s.store.Get(ctx, path)
	if err != nil {
		return nil, m, err
	}
	return rv, meta.Meta, nil
}

func (s *streamStore) Delete(ctx context.Context, path dtypes.Path) (
	err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: delete all the segments, with the last one last

	return s.store.Delete(ctx, path)
}

func (s *streamStore) List(ctx context.Context,
	startingPath, endingPath dtypes.Path) (
	paths []dtypes.Path, truncated bool, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: list all the paths inside l/, stripping off the l/ prefix

	return s.store.List(ctx, startingPath, endingPath)
}
