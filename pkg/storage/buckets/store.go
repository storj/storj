// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"context"
	"time"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storage/objects"
)

var (
	mon = monkit.Package()
)

// Store creates an interface for interacting with buckets
type Store interface {
	Get(ctx context.Context, bucket string) (meta Meta, err error)
	Put(ctx context.Context, bucket, location string) (meta Meta, err error)
	Delete(ctx context.Context, bucket string) (err error)
	List(ctx context.Context, startAfter, endBefore string, limit int) (
		items []Meta, more bool, err error)
	GetObjectStore(bucketName string) (store objects.Store, err error)
}

type bucketStore struct {
	o      objects.Store
	prefix string
}

// Meta is the bucket metadata struct
type Meta struct {
	Created time.Time
}

// NewStore instantiates bucketStore
func NewStore(obj objects.Store) Store {
	return &bucketStore{o: obj}
}

// GetObjectStore returns an implementation of objects.Store
func (b *bucketStore) GetObjectStore(bucket string) (objects.Store, error) {
	b.prefix = bucket
	return b.o, nil
}

// Get calls objects store Get
func (b *bucketStore) Get(ctx context.Context, bucket string) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	p := paths.New(bucket)
	_, objMeta, err := b.o.Get(ctx, p)
	if err != nil {
		return Meta{}, err
	}
	return Meta{Created: objMeta.Modified}, nil
}

// Put calls objects store Put
func (b *bucketStore) Put(ctx context.Context, bucket, location string) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	return Meta{}, nil
}

// Delete calls objects store Delete
func (b *bucketStore) Delete(ctx context.Context, bucket string) (err error) {
	defer mon.Task()(&ctx)(&err)
	p := paths.New(bucket)
	err = b.o.Delete(ctx, p)
	if err != nil {
		return err
	}
	return nil
}

// List calls objects store List
func (b *bucketStore) List(ctx context.Context, startAfter, endBefore string, limit int) (
	items []Meta, more bool, err error) {
	defer mon.Task()(&ctx)(&err)
	return []Meta{}, false, nil
}
