// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package objects

import (
	"context"
	"io"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/streams"
)

var mon = monkit.Package()

// NoPathError is an error class for missing object path
var NoPathError = errs.Class("no object path specified")

// Meta is the full object metadata
type Meta struct {
	SerializableMeta
	Modified   time.Time
	Expiration time.Time
	Size       int64
	Checksum   string
}

// ListItem is a single item in a listing
type ListItem struct {
	Path     paths.Path
	Meta     Meta
	IsPrefix bool
}

// Store for objects
type Store interface {
	Meta(ctx context.Context, path paths.Path) (meta Meta, err error)
	Get(ctx context.Context, path paths.Path) (rr ranger.RangeCloser,
		meta Meta, err error)
	Put(ctx context.Context, path paths.Path, data io.Reader,
		metadata SerializableMeta, expiration time.Time) (meta Meta, err error)
	Delete(ctx context.Context, path paths.Path) (err error)
	List(ctx context.Context, prefix, startAfter, endBefore paths.Path,
		recursive bool, limit int, metaFlags uint32) (items []ListItem,
		more bool, err error)
}

type objStore struct {
	s streams.Store
}

// NewStore for objects
func NewStore(store streams.Store) Store {
	return &objStore{s: store}
}

func (o *objStore) Meta(ctx context.Context, path paths.Path) (meta Meta,
	err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return Meta{}, NoPathError.New("")
	}

	m, err := o.s.Meta(ctx, path)
	return convertMeta(m), err
}

func (o *objStore) Get(ctx context.Context, path paths.Path) (
	rr ranger.RangeCloser, meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return nil, Meta{}, NoPathError.New("")
	}

	rr, m, err := o.s.Get(ctx, path)
	return rr, convertMeta(m), err
}

func (o *objStore) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata SerializableMeta, expiration time.Time) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return Meta{}, NoPathError.New("")
	}

	// TODO(kaloyan): autodetect content type
	// if metadata.GetContentType() == "" {}

	// TODO(kaloyan): encrypt metadata.UserDefined before serializing
	b, err := proto.Marshal(&metadata)
	if err != nil {
		return Meta{}, err
	}
	m, err := o.s.Put(ctx, path, data, b, expiration)
	return convertMeta(m), err
}

func (o *objStore) Delete(ctx context.Context, path paths.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return NoPathError.New("")
	}

	return o.s.Delete(ctx, path)
}

func (o *objStore) List(ctx context.Context, prefix, startAfter,
	endBefore paths.Path, recursive bool, limit int, metaFlags uint32) (
	items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	strItems, more, err := o.s.List(ctx, prefix, startAfter, endBefore,
		recursive, limit, metaFlags)
	if err != nil {
		return nil, false, err
	}

	items = make([]ListItem, len(strItems))
	for i, itm := range strItems {
		items[i] = ListItem{
			Path:     itm.Path,
			Meta:     convertMeta(itm.Meta),
			IsPrefix: itm.IsPrefix,
		}
	}

	return items, more, nil
}

// convertMeta converts stream metadata to object metadata
func convertMeta(m streams.Meta) Meta {
	ser := SerializableMeta{}
	err := proto.Unmarshal(m.Data, &ser)
	if err != nil {
		zap.S().Warnf("Failed deserializing metadata: %v", err)
	}
	return Meta{
		Modified:         m.Modified,
		Expiration:       m.Expiration,
		Size:             m.Size,
		SerializableMeta: ser,
	}
}
