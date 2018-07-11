// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package objects

import (
	"context"
	"io"
	"time"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage"
	"storj.io/storj/pkg/storage/streams"
)

var mon = monkit.Package()

// Store for objects
type Store interface {
	Meta(ctx context.Context, path paths.Path) (meta storage.Meta, err error)
	Get(ctx context.Context, path paths.Path) (rr ranger.RangeCloser,
		meta storage.Meta, err error)
	Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte,
		expiration time.Time) (meta storage.Meta, err error)
	Delete(ctx context.Context, path paths.Path) (err error)
	List(ctx context.Context, root, startAfter, endBefore paths.Path,
		recursive bool, limit int) (result []paths.Path, more bool, err error)
}

type objStore struct {
	s streams.Store
}

// NewStore for objects
func NewStore(store streams.Store) Store {
	return &objStore{s: store}
}

func (o *objStore) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata []byte, expiration time.Time) (meta storage.Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	objPath := getDefautStreamPath(path)
	return o.s.Put(ctx, objPath, data, metadata, expiration)
}

func (o *objStore) Get(ctx context.Context, path paths.Path) (
	rr ranger.RangeCloser, meta storage.Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	objPath := getDefautStreamPath(path)
	return o.s.Get(ctx, objPath)
}

func (o *objStore) Meta(ctx context.Context, path paths.Path) (
	meta storage.Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	objPath := getDefautStreamPath(path)
	return o.s.Meta(ctx, objPath)
}

func (o *objStore) Delete(ctx context.Context, path paths.Path) (err error) {
	defer mon.Task()(&ctx)(&err)
	objPath := getDefautStreamPath(path)
	return o.s.Delete(ctx, objPath)
}

func (o *objStore) List(ctx context.Context, root, startAfter,
	endBefore paths.Path, recursive bool, limit int) (result []paths.Path,
	more bool, err error) {
	defer mon.Task()(&ctx)(&err)
	rootObjPath := getDefautStreamPath(root)
	return o.s.List(ctx, rootObjPath, startAfter, endBefore, recursive, limit)
}

func getDefautStreamPath(path paths.Path) paths.Path {
	return path.Prepend("object")
}
