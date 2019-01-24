// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"bytes"
	"context"
	"strconv"
	"time"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

var mon = monkit.Package()

// Store creates an interface for interacting with buckets
type Store interface {
	Get(ctx context.Context, bucket string) (meta Meta, err error)
	Put(ctx context.Context, bucket string, pathCipher storj.Cipher) (meta Meta, err error)
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
	store  objects.Store
	stream streams.Store
}

// Meta is the bucket metadata struct
type Meta struct {
	Created            time.Time
	PathEncryptionType storj.Cipher
}

// NewStore instantiates BucketStore
func NewStore(stream streams.Store) Store {
	// root object store for storing the buckets with unencrypted names
	store := objects.NewStore(stream, storj.Unencrypted)
	return &BucketStore{store: store, stream: stream}
}

// GetObjectStore returns an implementation of objects.Store
func (b *BucketStore) GetObjectStore(ctx context.Context, bucket string) (objects.Store, error) {
	if bucket == "" {
		return nil, storj.ErrNoBucket.New("")
	}

	m, err := b.Get(ctx, bucket)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			err = storj.ErrBucketNotFound.Wrap(err)
		}
		return nil, err
	}
	prefixed := prefixedObjStore{
		store:  objects.NewStore(b.stream, m.PathEncryptionType),
		prefix: bucket,
	}
	return &prefixed, nil
}

// Get calls objects store Get
func (b *BucketStore) Get(ctx context.Context, bucket string) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return Meta{}, storj.ErrNoBucket.New("")
	}

	objMeta, err := b.store.Meta(ctx, bucket)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			err = storj.ErrBucketNotFound.Wrap(err)
		}
		return Meta{}, err
	}

	return convertMeta(objMeta)
}

// Put calls objects store Put
func (b *BucketStore) Put(ctx context.Context, bucket string, pathCipher storj.Cipher) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return Meta{}, storj.ErrNoBucket.New("")
	}

	if pathCipher < storj.Unencrypted || pathCipher > storj.SecretBox {
		return Meta{}, encryption.ErrInvalidConfig.New("encryption type %d is not supported", pathCipher)
	}

	r := bytes.NewReader(nil)
	userMeta := map[string]string{
		"path-enc-type": strconv.Itoa(int(pathCipher)),
	}
	var exp time.Time
	m, err := b.store.Put(ctx, bucket, r, pb.SerializableMeta{UserDefined: userMeta}, exp)
	if err != nil {
		return Meta{}, err
	}
	return convertMeta(m)
}

// Delete calls objects store Delete
func (b *BucketStore) Delete(ctx context.Context, bucket string) (err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return storj.ErrNoBucket.New("")
	}

	err = b.store.Delete(ctx, bucket)

	if storage.ErrKeyNotFound.Has(err) {
		err = storj.ErrBucketNotFound.Wrap(err)
	}

	return err
}

// List calls objects store List
func (b *BucketStore) List(ctx context.Context, startAfter, endBefore string, limit int) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	objItems, more, err := b.store.List(ctx, "", startAfter, endBefore, false, limit, meta.Modified)
	if err != nil {
		return items, more, err
	}

	items = make([]ListItem, 0, len(objItems))
	for _, itm := range objItems {
		if itm.IsPrefix {
			continue
		}
		m, err := convertMeta(itm.Meta)
		if err != nil {
			return items, more, err
		}
		items = append(items, ListItem{
			Bucket: itm.Path,
			Meta:   m,
		})
	}
	return items, more, nil
}

// convertMeta converts stream metadata to object metadata
func convertMeta(m objects.Meta) (Meta, error) {
	var cipher storj.Cipher

	pathEncType := m.UserDefined["path-enc-type"]

	if pathEncType == "" {
		// backward compatibility for old buckets
		cipher = storj.AESGCM
	} else {
		pet, err := strconv.Atoi(pathEncType)
		if err != nil {
			return Meta{}, err
		}
		cipher = storj.Cipher(pet)
	}

	return Meta{
		Created:            m.Modified,
		PathEncryptionType: cipher,
	}, nil
}
