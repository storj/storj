// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"errors"
	"io"
	"time"

	"storj.io/storj/pkg/paths"
	ranger "storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/segments"
)

var ctx = context.Background()

type metaClosure = func(context.Context, paths.Path) (segments.Meta, error)
type getClosure = func(context.Context, paths.Path) (ranger.Ranger, segments.Meta, error)
type putClosure = func(context.Context, paths.Path, io.Reader, []byte, time.Time) (segments.Meta, error)
type deleteClosure = func(context.Context, paths.Path) error
type listClosure = func(context.Context, paths.Path, paths.Path, paths.Path, bool, int, uint32) ([]segments.ListItem, bool, error)

type segmentStub struct {
	mc metaClosure
	gc getClosure
	pc putClosure
	dc deleteClosure
	lc listClosure
}

func (s *segmentStub) Meta(ctx context.Context, path paths.Path) (meta segments.Meta, err error) {
	return s.mc(ctx, path)
}

func (s *segmentStub) Get(ctx context.Context, path paths.Path) (rr ranger.Ranger,
	meta segments.Meta, err error) {
	return s.gc(ctx, path)
}

func (s *segmentStub) Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte,
	expiration time.Time) (meta segments.Meta, err error) {
	return s.pc(ctx, path, data, metadata, expiration)
}

func (s *segmentStub) Delete(ctx context.Context, path paths.Path) (err error) {
	return s.dc(ctx, path)
}

func (s *segmentStub) List(ctx context.Context, prefix, startAfter, endBefore paths.Path,
	recursive bool, limit int, metaFlags uint32) (items []segments.ListItem,
	more bool, err error) {
	return s.lc(ctx, prefix, startAfter, endBefore, recursive, limit, metaFlags)
}

type metaTestStruct struct {
	//inputs
	path        paths.Path
	size        int64
	data        []byte
	errorString string
	//outputs
	segmentMetaData []byte
	segmentErr      error
	streamMetaData  []byte
	streamErr       error
}

func makeSegmentMeta(size int64, data []byte) segments.Meta {
	return segments.Meta{
		Modified:   time.Now(),
		Expiration: time.Now(),
		Size:       size,
		Data:       data,
	}
}

func makeMetaClosure(meta segments.Meta, errorString string) metaClosure {
	return func(ctx context.Context, path paths.Path) (segments.Meta, error) {
		if errorString == "" {
			return meta, nil
		}
		return segments.Meta{}, errors.New(errorString)
	}
}
