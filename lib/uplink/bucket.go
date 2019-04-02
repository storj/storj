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
	Config BucketConfig

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

	// Volatile groups config values that are likely to change semantics
	// or go away entirely between releases. Be careful when using them!
	Volatile struct {
		// EncryptionParameters determines the cipher suite to use for
		// the Object's data encryption. If not set, the Bucket's
		// defaults will be used.
		EncryptionParameters storj.EncryptionParameters

		// RedundancyScheme determines the Reed-Solomon and/or Forward
		// Error Correction encoding parameters to be used for this
		// Object.
		RedundancyScheme storj.RedundancyScheme
	}
}

// UploadObject uploads a new object, if authorized.
func (b *Bucket) UploadObject(ctx context.Context, path storj.Path, data io.Reader, opts *UploadOptions) (err error) {
	defer mon.Task()(&ctx)(&err)
	// SIGH thanks, lint. we should uncomment this once it's being used.
	//if opts == nil {
	//	opts = &UploadOptions{}
	//}
	panic("TODO")
}

// DeleteObject removes an object, if authorized.
func (b *Bucket) DeleteObject(ctx context.Context, path storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)
	return b.metainfo.DeleteObject(ctx, b.Bucket.Name, path)
}

// ListOptions controls options for the ListObjects() call.
type ListOptions = storj.ListOptions

// ListObjects lists objects a user is authorized to see.
// TODO(paul): should probably have a ListOptions defined in this package, for consistency's sake
func (b *Bucket) ListObjects(ctx context.Context, cfg *ListOptions) (list storj.ObjectList, err error) {
	defer mon.Task()(&ctx)(&err)
	if cfg == nil {
		cfg = &storj.ListOptions{}
	}
	return b.metainfo.ListObjects(ctx, b.Bucket.Name, *cfg)
}

// Close closes the Bucket session.
func (b *Bucket) Close() error {
	return nil
}
