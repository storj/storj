// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/stream"
)

// Encryption holds the cipher, path, key, and enc. scheme for each bucket since they
// can be different for each
type Encryption struct {
	PathCipher       storj.Cipher
	EncPathPrefix    storj.Path
	Key              storj.Key
	EncryptionScheme storj.EncryptionScheme
}

// Bucket is a struct that allows operations on a Bucket after a user providers Permissions
type Bucket struct {
	Access *Access
	Enc    *Encryption
	Bucket storj.Bucket
}

// GetObject returns the info for a given object
func (b *Bucket) GetObject(ctx context.Context, path storj.Path) (storj.Object, error) {
	metainfo, _, err := b.Access.Uplink.config.GetMetainfo(ctx, b.Access.Uplink.id)
	if err != nil {
		return storj.Object{}, Error.Wrap(err)
	}

	return metainfo.GetObject(ctx, b.Bucket.Name, path)
}

// List returns a list of objects in a bucket
func (b *Bucket) List(ctx context.Context, cfg ListObjectsConfig) (items storj.ObjectList, err error) {
	metainfo, _, err := b.Access.Uplink.config.GetMetainfo(ctx, b.Access.Uplink.id)
	if err != nil {
		return storj.ObjectList{}, Error.Wrap(err)
	}

	listOpts := storj.ListOptions{
		Prefix:    cfg.Prefix,
		Cursor:    cfg.Cursor,
		Recursive: cfg.Recursive,
		Direction: cfg.Direction,
		Limit:     cfg.Limit,
	}

	return metainfo.ListObjects(ctx, b.Bucket.Name, listOpts)
}

// Upload puts an object in a bucket
func (b *Bucket) Upload(ctx context.Context, path storj.Path, data []byte, opts UploadOpts) error {
	metainfo, streams, err := b.Access.Uplink.config.GetMetainfo(ctx, b.Access.Uplink.id)
	if err != nil {
		return Error.Wrap(err)
	}

	encScheme := b.Access.Uplink.config.GetEncryptionScheme()
	redScheme := b.Access.Uplink.config.GetRedundancyScheme()
	contentType := http.DetectContentType(data)

	create := storj.CreateObject{
		RedundancyScheme: redScheme,
		EncryptionScheme: encScheme,
		ContentType:      contentType,
	}

	obj, err := metainfo.CreateObject(ctx, b.Bucket.Name, path, &create)
	if err != nil {
		return Error.Wrap(err)
	}

	reader := bytes.NewReader(data)
	mutableStream, err := obj.CreateStream(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	upload := stream.NewUpload(ctx, mutableStream, streams)

	_, err = io.Copy(upload, reader)
	if err != nil {
		return Error.Wrap(err)
	}

	return errs.Combine(err, upload.Close())
}

// Download downloads an object from a bucket
func (b *Bucket) Download(ctx context.Context, path storj.Path) ([]byte, error) {
	metainfo, streams, err := b.Access.Uplink.config.GetMetainfo(ctx, b.Access.Uplink.id)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	readStream, err := metainfo.GetObjectStream(ctx, b.Bucket.Name, path)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	stream := stream.NewDownload(ctx, readStream, streams)

	defer func() { err = errs.Combine(err, stream.Close()) }()

	data, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return data, nil
}

// Delete removes an object from a bucket and returns an error if there was an issue
func (b *Bucket) Delete(ctx context.Context, path storj.Path) error {
	metainfo, _, err := b.Access.Uplink.config.GetMetainfo(ctx, b.Access.Uplink.id)
	if err != nil {
		return Error.Wrap(err)
	}

	return metainfo.DeleteObject(ctx, b.Bucket.Name, path)
}
