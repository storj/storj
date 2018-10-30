// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"
	"errors"

	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storj"
)

// Objects implements storj.Metainfo bucket handling
type Objects struct {
	store objects.Store
}

// NewObjects creates Objects
func NewObjects(store objects.Store) *Objects { return &Objects{store} }

// GetObject returns information about an object
func (db *Objects) GetObject(ctx context.Context, bucket string, path storj.Path) (storj.Object, error) {
	meta, err := db.store.Meta(ctx, bucket+"/"+path)
	if err != nil {
		return storj.Object{}, err
	}

	return objectFromMeta(bucket, path, false, meta), nil
}

// GetObjectStream returns interface for reading the object stream
func (db *Objects) GetObjectStream(ctx context.Context, bucket string, path storj.Path) (storj.ReadOnlyStream, error) {
	return nil, errors.New("not implemented")
}

// CreateObject creates an uploading object and returns an interface for uploading Object information
func (db *Objects) CreateObject(ctx context.Context, bucket string, path storj.Path, info *storj.CreateObject) (storj.MutableObject, error) {
	return nil, errors.New("not implemented")
}

// ModifyObject creates an interface for modifying an existing object
func (db *Objects) ModifyObject(ctx context.Context, bucket string, path storj.Path, info storj.Object) (storj.MutableObject, error) {
	return nil, errors.New("not implemented")
}

// DeleteObject deletes an object from database
func (db *Objects) DeleteObject(ctx context.Context, bucket string, path storj.Path) error {
	return db.store.Delete(ctx, bucket+"/"+path)
}

// ListObjects lists objects in bucket based on the ListOptions
func (db *Objects) ListObjects(ctx context.Context, bucket string, options storj.ListOptions) (storj.ObjectList, error) {
	var startAfter, endBefore string
	switch options.Direction {
	case storj.Before:
		// before lists backwards from cursor, without cursor
		endBefore = options.Cursor
	case storj.Backward:
		// backward lists backwards from cursor, including cursor
		endBefore = keyAfter(options.Cursor)
	case storj.Forward:
		// forward lists forwards from cursor, including cursor
		startAfter = keyBefore(options.Cursor)
	case storj.After:
		// after lists forwards from cursor, without cursor
		startAfter = options.Cursor
	default:
		return storj.ObjectList{}, errClass.New("invalid direction %d", options.Direction)
	}

	items, more, err := db.store.List(ctx, bucket+"/"+options.Prefix, startAfter, endBefore, options.Recursive, options.Limit, meta.All)
	if err != nil {
		return storj.ObjectList{}, err
	}

	list := storj.ObjectList{
		Bucket: bucket,
		Prefix: options.Prefix,
		More:   more,
		Items:  make([]storj.Object, 0, len(items)),
	}

	for _, item := range items {
		list.Items = append(list.Items, objectFromMeta("", item.Path, item.IsPrefix, item.Meta))
	}

	return list, nil
}

func objectFromMeta(bucket string, path storj.Path, isPrefix bool, meta objects.Meta) storj.Object {
	return storj.Object{
		Version:  0, // TODO:
		Bucket:   bucket,
		Path:     path,
		IsPrefix: isPrefix,

		Metadata: nil,

		ContentType: meta.ContentType,
		// Created:     meta.Created,
		Modified: meta.Modified,
		Expires:  meta.Expiration,

		Stream: storj.Stream{
			Size:     meta.Size,
			Checksum: []byte(meta.Checksum),
		},
	}
}
