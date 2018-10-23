// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"context"
	"io"
	"path"
	"time"

	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storj"
)

type prefixedObjStore struct {
	o      objects.Store
	prefix string
}

func (o *prefixedObjStore) Meta(ctx context.Context, p storj.Path) (meta objects.Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(p) == 0 {
		return objects.Meta{}, objects.NoPathError.New("")
	}

	m, err := o.o.Meta(ctx, path.Join(o.prefix, p))
	return m, err
}

func (o *prefixedObjStore) Get(ctx context.Context, p storj.Path) (rr ranger.Ranger, meta objects.Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(p) == 0 {
		return nil, objects.Meta{}, objects.NoPathError.New("")
	}

	rr, m, err := o.o.Get(ctx, path.Join(o.prefix, p))
	return rr, m, err
}

func (o *prefixedObjStore) Put(ctx context.Context, p storj.Path, data io.Reader, metadata objects.SerializableMeta, expiration time.Time) (meta objects.Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(p) == 0 {
		return objects.Meta{}, objects.NoPathError.New("")
	}

	m, err := o.o.Put(ctx, path.Join(o.prefix, p), data, metadata, expiration)
	return m, err
}

func (o *prefixedObjStore) Delete(ctx context.Context, p storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(p) == 0 {
		return objects.NoPathError.New("")
	}

	return o.o.Delete(ctx, path.Join(o.prefix, p))
}

func (o *prefixedObjStore) List(ctx context.Context, prefix, startAfter, endBefore storj.Path, recursive bool, limit int, metaFlags uint32) (items []objects.ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)
	return o.o.List(ctx, path.Join(o.prefix, prefix), startAfter, endBefore, recursive, limit, metaFlags)
}
