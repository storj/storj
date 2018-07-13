// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package objects

import (
	"context"
	"io"
	"time"

	"github.com/gogo/protobuf/proto"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/protos/meta"
)

var mon = monkit.Package()

// Store for objects
type Store interface {
	Meta(ctx context.Context, path paths.Path) (meta storage.Meta, err error)
	Get(ctx context.Context, path paths.Path) (rr ranger.RangeCloser,
		meta storage.Meta, err error)
	Put(ctx context.Context, path paths.Path, data io.Reader,
		metadata meta.Serializable, expiration time.Time) (meta storage.Meta,
		err error)
	Delete(ctx context.Context, path paths.Path) (err error)
	List(ctx context.Context, prefix, startAfter, endBefore paths.Path,
		recursive bool, limit int, metaFlags uint64) (items []storage.ListItem,
		more bool, err error)
}

type objStore struct {
	s streams.Store
}

// NewStore for objects
func NewStore(store streams.Store) Store {
	return &objStore{s: store}
}

func (o *objStore) Meta(ctx context.Context, path paths.Path) (
	meta storage.Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	return o.s.Meta(ctx, path)
}

func (o *objStore) Get(ctx context.Context, path paths.Path) (
	rr ranger.RangeCloser, meta storage.Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	return o.s.Get(ctx, path)
}

func (o *objStore) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata meta.Serializable, expiration time.Time) (meta storage.Meta,
	err error) {
	defer mon.Task()(&ctx)(&err)
	if metadata.GetContentType() == "" {
		// TODO autodetect content type
	}
	// TODO encrypt metadata.UserDefined before serializing
	b, err := proto.Marshal(&metadata)
	if err != nil {
		return storage.Meta{}, err
	}
	return o.s.Put(ctx, path, data, b, expiration)
}

func (o *objStore) Delete(ctx context.Context, path paths.Path) (err error) {
	defer mon.Task()(&ctx)(&err)
	return o.s.Delete(ctx, path)
}

func (o *objStore) List(ctx context.Context, prefix, startAfter,
	endBefore paths.Path, recursive bool, limit int, metaFlags uint64) (
	items []storage.ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)
	return o.s.List(ctx, prefix, startAfter, endBefore, recursive, limit,
		metaFlags)
}
