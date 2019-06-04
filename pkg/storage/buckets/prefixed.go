// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storj"
)
// PrefixedObjStore  ...
type PrefixedObjStore struct {
	store  objects.Store
	prefix string
}

// Meta ...
func (o *PrefixedObjStore) Meta(ctx context.Context, path storj.Path) (meta objects.Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return objects.Meta{}, storj.ErrNoPath.New("")
	}

	return o.store.Meta(ctx, storj.JoinPaths(o.prefix, path))
}

// Get ...
func (o *PrefixedObjStore) Get(ctx context.Context, path storj.Path) (rr ranger.Ranger, meta objects.Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return nil, objects.Meta{}, storj.ErrNoPath.New("")
	}

	return o.store.Get(ctx, storj.JoinPaths(o.prefix, path))
}

// Put ...
func (o *PrefixedObjStore) Put(ctx context.Context, path storj.Path, data io.Reader, metadata pb.SerializableMeta, expiration time.Time) (meta objects.Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return objects.Meta{}, storj.ErrNoPath.New("")
	}

	return o.store.Put(ctx, storj.JoinPaths(o.prefix, path), data, metadata, expiration)
}

// Delete ...
func (o *PrefixedObjStore) Delete(ctx context.Context, path storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return storj.ErrNoPath.New("")
	}

	return o.store.Delete(ctx, storj.JoinPaths(o.prefix, path))
}

// List ...
func (o *PrefixedObjStore) List(ctx context.Context, prefix, startAfter, endBefore storj.Path, recursive bool, limit int, metaFlags uint32) (items []objects.ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	return o.store.List(ctx, storj.JoinPaths(o.prefix, prefix), startAfter, endBefore, recursive, limit, metaFlags)
}
