// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package objects

import (
	"context"
	"io"
	"time"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

var mon = monkit.Package()

// Meta is the full object metadata
type Meta struct {
	pb.SerializableMeta
	Modified   time.Time
	Expiration time.Time
	Size       int64
	Checksum   string
}

// ListItem is a single item in a listing
type ListItem struct {
	Path     storj.Path
	Meta     Meta
	IsPrefix bool
}

// Store for objects
type Store interface {
	Meta(ctx context.Context, path storj.Path) (meta Meta, err error)
	Get(ctx context.Context, path storj.Path) (rr ranger.Ranger, meta Meta, err error)
	Put(ctx context.Context, path storj.Path, data io.Reader, metadata pb.SerializableMeta, expiration time.Time) (meta Meta, err error)
	Delete(ctx context.Context, path storj.Path) (err error)
	List(ctx context.Context, prefix, startAfter, endBefore storj.Path, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error)
}

type objStore struct {
	store      streams.Store
	pathCipher storj.CipherSuite
}

// NewStore for objects
func NewStore(store streams.Store, pathCipher storj.CipherSuite) Store {
	return &objStore{store: store, pathCipher: pathCipher}
}

func (o *objStore) Meta(ctx context.Context, path storj.Path) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return Meta{}, storj.ErrNoPath.New("")
	}

	m, err := o.store.Meta(ctx, path, o.pathCipher)

	if storage.ErrKeyNotFound.Has(err) {
		err = storj.ErrObjectNotFound.Wrap(err)
	}

	return convertMeta(m), err
}

func (o *objStore) Get(ctx context.Context, path storj.Path) (
	rr ranger.Ranger, meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return nil, Meta{}, storj.ErrNoPath.New("")
	}

	rr, m, err := o.store.Get(ctx, path, o.pathCipher)

	if storage.ErrKeyNotFound.Has(err) {
		err = storj.ErrObjectNotFound.Wrap(err)
	}

	return rr, convertMeta(m), err
}

func (o *objStore) Put(ctx context.Context, path storj.Path, data io.Reader, metadata pb.SerializableMeta, expiration time.Time) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return Meta{}, storj.ErrNoPath.New("")
	}

	// TODO(kaloyan): autodetect content type
	// if metadata.GetContentType() == "" {}

	b, err := proto.Marshal(&metadata)
	if err != nil {
		return Meta{}, err
	}
	m, err := o.store.Put(ctx, path, o.pathCipher, data, b, expiration)
	return convertMeta(m), err
}

func (o *objStore) Delete(ctx context.Context, path storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return storj.ErrNoPath.New("")
	}

	err = o.store.Delete(ctx, path, o.pathCipher)

	if storage.ErrKeyNotFound.Has(err) {
		err = storj.ErrObjectNotFound.Wrap(err)
	}

	return err
}

func (o *objStore) List(ctx context.Context, prefix, startAfter, endBefore storj.Path, recursive bool, limit int, metaFlags uint32) (
	items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	strItems, more, err := o.store.List(ctx, prefix, startAfter, endBefore, o.pathCipher, recursive, limit, metaFlags)
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
	ser := pb.SerializableMeta{}
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
