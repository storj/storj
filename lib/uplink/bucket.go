// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/metainfo/kvmetainfo"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
)

// Bucket represents operations you can perform on a bucket
type Bucket struct {
	storj.Bucket

	metainfo   *kvmetainfo.DB
	streams    streams.Store
	pathCipher storj.Cipher
}

// OpenObject returns an Object handle, if authorized.
func (b *Bucket) OpenObject(ctx context.Context, path storj.Path) (o *Object, err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}

// UploadOptions controls options about uploading a new Object, if authorized.
type UploadOptions struct {
	// ContentType, if set, gives a MIME content-type for the Object.
	ContentType string
	// Metadata contains additional information about an Object. It can
	// hold arbitrary textual fields and can be retrieved together with the
	// Object. Field names can be at most 1024 bytes long. Field values are
	// not individually limited in size, but the total of all metadata
	// (fields and values) can not exceed 4 kiB.
	Metadata map[string]string
	// Expires is the time at which the new Object can expire (be deleted
	// automatically from storage nodes).
	Expires time.Time

	// EncryptionScheme determines the object's encryption scheme. If not
	// set, the Bucket's default will be used.
	EncryptionScheme *storj.EncryptionScheme
}

// UploadObject uploads a new object, if authorized.
func (b *Bucket) UploadObject(ctx context.Context, path storj.Path, data io.Reader, opts UploadOptions) (err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}

// DeleteObject removes an object, if authorized.
func (b *Bucket) DeleteObject(ctx context.Context, path storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)
	return b.metainfo.DeleteObject(ctx, b.Bucket.Name, path)
}

// ListObjects lists objects a user is authorized to see.
func (b *Bucket) ListObjects(ctx context.Context, cfg storj.ListOptions) (list storj.ObjectList, err error) {
	defer mon.Task()(&ctx)(&err)
	return b.metainfo.ListObjects(ctx, b.Bucket.Name, cfg)
}

// Close closes the Bucket session.
func (b *Bucket) Close() error {
	return nil
}
