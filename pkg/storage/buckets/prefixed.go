// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"context"
	"io"
	"time"

	"go.uber.org/zap"
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
	m, err := o.o.Meta(ctx, path.Prepend(o.prefix))
	return m, err
}

func (o *prefixedObjStore) Get(ctx context.Context, path paths.Path) (
	rr ranger.RangeCloser, meta objects.Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	rr, m, err := o.o.Get(ctx, path.Prepend(o.prefix))
	return rr, m, err
}

func (o *prefixedObjStore) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata objects.SerializableMeta, expiration time.Time) (meta objects.Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	m, err := o.o.Put(ctx, path.Prepend(o.prefix), data, metadata, expiration)
	return m, err
}

func (o *prefixedObjStore) Delete(ctx context.Context, path paths.Path) (err error) {
	defer mon.Task()(&ctx)(&err)
	return o.o.Delete(ctx, path.Prepend(o.prefix))
}

func (o *prefixedObjStore) List(ctx context.Context, prefix, startAfter,
	endBefore paths.Path, recursive bool, limit int, metaFlags uint32) (
	items []objects.ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	objItems, more, err := o.o.List(ctx, prefix.Prepend(o.prefix), startAfter, endBefore,
		recursive, limit, metaFlags)
	if err != nil {
		return nil, false, err
	}

	items = make([]objects.ListItem, len(objItems))
	for i, itm := range objItems {
		if len(itm.Path) == 0 {
			zap.S().Warnf("empty path in list item, skipping from results")
			continue
		}
		items[i] = objects.ListItem{
			Path: itm.Path[1:],
			Meta: itm.Meta,
		}
	}
	return items, more, nil
}
