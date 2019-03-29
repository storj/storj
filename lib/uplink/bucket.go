// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"io"
	"time"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/metainfo/kvmetainfo"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/stream"
	"storj.io/storj/pkg/utils"
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

	info, err := b.metainfo.GetObject(ctx, b.Name, path)
	if err != nil {
		return nil, err
	}

	return &Object{
		Meta: ObjectMeta{
			Bucket:      info.Bucket.Name,
			Path:        info.Path,
			IsPrefix:    info.IsPrefix,
			ContentType: info.ContentType,
			Metadata:    info.Metadata,
			Created:     info.Created,
			Modified:    info.Modified,
			Expires:     info.Expires,
			Size:        info.Size,
			Checksum:    info.Checksum,
			Volatile: struct {
				DataCipher          Cipher
				EncryptionBlockSize memory.Size
				RSParameters        storj.RedundancyScheme
			}{
				DataCipher:          Cipher(info.EncryptionScheme.Cipher + 1), // TODO: better conversion
				EncryptionBlockSize: memory.Size(info.EncryptionScheme.BlockSize),
				RSParameters:        info.RedundancyScheme,
			},
		},
		metainfo: b.metainfo,
		streams:  b.streams,
	}, nil
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
		// DataCipher determines the ciphersuite to use for the Object's
		// data encryption. If not set, the Bucket's default will be
		// used.
		DataCipher Cipher
		// EncryptionBlockSize determines the unit size at which
		// encryption is performed. See BucketConfig.EncryptionBlockSize
		// for more information.
		EncryptionBlockSize memory.Size

		// RSParameters determines the Reed-Solomon and/or Forward Error
		// Correction encoding parameters to be used for this Object.
		RSParameters storj.RedundancyScheme
	}
}

// UploadObject uploads a new object, if authorized.
func (b *Bucket) UploadObject(ctx context.Context, path storj.Path, data io.Reader, opts *UploadOptions) (err error) {
	defer mon.Task()(&ctx)(&err)

	if opts == nil {
		opts = &UploadOptions{}
	}

	cipher, err := opts.Volatile.DataCipher.convert()
	if err != nil {
		return err
	}

	createInfo := storj.CreateObject{
		ContentType:      opts.ContentType,
		Metadata:         opts.Metadata,
		Expires:          opts.Expires,
		RedundancyScheme: opts.Volatile.RSParameters,
		EncryptionScheme: storj.EncryptionScheme{
			Cipher:    cipher,
			BlockSize: opts.Volatile.EncryptionBlockSize.Int32(),
		},
	}

	obj, err := b.metainfo.CreateObject(ctx, b.Name, path, &createInfo)
	if err != nil {
		return err
	}

	mutableStream, err := obj.CreateStream(ctx)
	if err != nil {
		return err
	}

	upload := stream.NewUpload(ctx, mutableStream, b.streams)

	_, err = io.Copy(upload, data)

	return utils.CombineErrors(err, upload.Close())
}

// DeleteObject removes an object, if authorized.
func (b *Bucket) DeleteObject(ctx context.Context, path storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)
	return b.metainfo.DeleteObject(ctx, b.Bucket.Name, path)
}

// ListObjects lists objects a user is authorized to see.
// TODO(paul): should probably have a ListOptions defined in this package, for consistency's sake
func (b *Bucket) ListObjects(ctx context.Context, cfg *storj.ListOptions) (list storj.ObjectList, err error) {
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
