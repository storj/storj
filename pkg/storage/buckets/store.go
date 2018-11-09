// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"bytes"
	"context"
	"time"

	minio "github.com/minio/minio/cmd"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

var mon = monkit.Package()

// Store creates an interface for interacting with buckets
type Store interface {
	Get(ctx context.Context, bucket string) (meta Meta, err error)
	Put(ctx context.Context, bucket string) (meta Meta, err error)
	Delete(ctx context.Context, bucket string) (err error)
	List(ctx context.Context, startAfter, endBefore string, limit int) (items []ListItem, more bool, err error)
	GetObjectStore(ctx context.Context, bucketName string) (store objects.Store, err error)
}

// ListItem is a single item in a listing
type ListItem struct {
	Bucket string
	Meta   Meta
}

// BucketStore contains objects store
type BucketStore struct {
	o objects.Store
}

// Meta is the bucket metadata struct
type Meta struct {
	Created time.Time
}

// NewStore instantiates BucketStore
func NewStore(obj objects.Store) Store {
	return &BucketStore{o: obj}
}

// GetObjectStore returns an implementation of objects.Store
func (b *BucketStore) GetObjectStore(ctx context.Context, bucket string) (objects.Store, error) {
	if bucket == "" {
		return nil, storj.NoBucketError.New("")
	}

	_, err := b.Get(ctx, bucket)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return nil, minio.BucketNotFound{Bucket: bucket}
		}
		return nil, err
	}
	prefixed := prefixedObjStore{
		o:      b.o,
		prefix: bucket,
	}
	return &prefixed, nil
}

// Get calls objects store Get
func (b *BucketStore) Get(ctx context.Context, bucket string) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return Meta{}, storj.NoBucketError.New("")
	}

	objMeta, err := b.o.Meta(ctx, bucket)
	if err != nil {
		return Meta{}, err
	}
	return convertMeta(objMeta), nil
}

// Put calls objects store Put
func (b *BucketStore) Put(ctx context.Context, bucket string) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return Meta{}, storj.NoBucketError.New("")
	}

	r := bytes.NewReader(nil)
	var exp time.Time
	m, err := b.o.Put(ctx, bucket, r, objects.SerializableMeta{}, exp)
	if err != nil {
		return Meta{}, err
	}
	return convertMeta(m), nil
}

// Delete calls objects store Delete
func (b *BucketStore) Delete(ctx context.Context, bucket string) (err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return storj.NoBucketError.New("")
	}

	return b.o.Delete(ctx, bucket)
}

// List calls objects store List
func (b *BucketStore) List(ctx context.Context, startAfter, endBefore string, limit int) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	objItems, more, err := b.o.List(ctx, "", startAfter, endBefore, false, limit, meta.Modified)
	if err != nil {
		return items, more, err
	}

	items = make([]ListItem, 0, len(objItems))
	for _, itm := range objItems {
		if itm.IsPrefix {
			continue
		}
		items = append(items, ListItem{
			Bucket: itm.Path,
			Meta:   convertMeta(itm.Meta),
		})
	}
	return items, more, nil
}

// convertMeta converts stream metadata to object metadata
func convertMeta(m objects.Meta) Meta {
	return Meta{
		Created: m.Modified,
	}
}
