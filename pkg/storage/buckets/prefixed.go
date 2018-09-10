// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/objects"
)

type prefixedObjStore struct {
	o      objects.Store
	prefix string
}

func (o *prefixedObjStore) Meta(ctx context.Context, path paths.Path) (meta objects.Meta,
	err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return objects.Meta{}, objects.NoPathError.New("")
	}

	m, err := o.o.Meta(ctx, path.Prepend(o.prefix))
	return m, err
}

func (o *prefixedObjStore) Get(ctx context.Context, path paths.Path) (
	rr ranger.RangeCloser, meta objects.Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return nil, objects.Meta{}, objects.NoPathError.New("")
	}

	rr, m, err := o.o.Get(ctx, path.Prepend(o.prefix))
	return rr, m, err
}

func (o *prefixedObjStore) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata objects.SerializableMeta, expiration time.Time) (meta objects.Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return objects.Meta{}, objects.NoPathError.New("")
	}

	m, err := o.o.Put(ctx, path.Prepend(o.prefix), data, metadata, expiration)
	return m, err
}

func (o *prefixedObjStore) Delete(ctx context.Context, path paths.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return objects.NoPathError.New("")
	}

	return o.o.Delete(ctx, path.Prepend(o.prefix))
}

func (o *prefixedObjStore) List(ctx context.Context, prefix, startAfter,
	endBefore paths.Path, recursive bool, limit int, metaFlags uint32) (
	items []objects.ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)
	return o.o.List(ctx, prefix.Prepend(o.prefix), startAfter, endBefore, recursive, limit, metaFlags)
}
