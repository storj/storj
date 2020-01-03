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

	"storj.io/common/pb"
	"storj.io/common/ranger"
	"storj.io/common/storj"
	"storj.io/storj/uplink/storage/streams"
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
	Get(ctx context.Context, path storj.Path, object storj.Object) (rr ranger.Ranger, err error)
	Put(ctx context.Context, path storj.Path, data io.Reader, metadata pb.SerializableMeta, expiration time.Time) (meta Meta, err error)
	Delete(ctx context.Context, path storj.Path) (err error)
	List(ctx context.Context, prefix, startAfter storj.Path, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error)
}

type objStore struct {
	store      streams.Store
	pathCipher storj.CipherSuite
}

// NewStore for objects
func NewStore(store streams.Store, pathCipher storj.CipherSuite) Store {
	return &objStore{store: store, pathCipher: pathCipher}
}

func (o *objStore) Get(ctx context.Context, path storj.Path, object storj.Object) (
	rr ranger.Ranger, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(path) == 0 {
		return nil, storj.ErrNoPath.New("")
	}

	rr, err = o.store.Get(ctx, path, object, o.pathCipher)
	return rr, err
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

	return o.store.Delete(ctx, path, o.pathCipher)
}

func (o *objStore) List(ctx context.Context, prefix, startAfter storj.Path, recursive bool, limit int, metaFlags uint32) (
	items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	strItems, more, err := o.store.List(ctx, prefix, startAfter, o.pathCipher, recursive, limit, metaFlags)
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
