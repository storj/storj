// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"
	"io"
	"time"

	"storj.io/common/pb"
	"storj.io/common/ranger"
	"storj.io/common/storj"
	"storj.io/storj/uplink/storage/objects"
)

type prefixedObjStore struct {
	store  objects.Store
	prefix string
}

func (o *prefixedObjStore) Get(ctx context.Context, path storj.Path, object storj.Object) (rr ranger.Ranger, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return nil, storj.ErrNoPath.New("")
	}

	return o.store.Get(ctx, storj.JoinPaths(o.prefix, path), object)
}

func (o *prefixedObjStore) Put(ctx context.Context, path storj.Path, data io.Reader, metadata pb.SerializableMeta, expiration time.Time) (meta objects.Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return objects.Meta{}, storj.ErrNoPath.New("")
	}

	return o.store.Put(ctx, storj.JoinPaths(o.prefix, path), data, metadata, expiration)
}

func (o *prefixedObjStore) Delete(ctx context.Context, path storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return storj.ErrNoPath.New("")
	}

	return o.store.Delete(ctx, storj.JoinPaths(o.prefix, path))
}

func (o *prefixedObjStore) List(ctx context.Context, prefix, startAfter storj.Path, recursive bool, limit int, metaFlags uint32) (items []objects.ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	return o.store.List(ctx, storj.JoinPaths(o.prefix, prefix), startAfter, recursive, limit, metaFlags)
}
