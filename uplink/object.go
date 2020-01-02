// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"io"
	"time"

	"storj.io/common/storj"
	"storj.io/storj/uplink/metainfo/kvmetainfo"
	"storj.io/storj/uplink/storage/streams"
	"storj.io/storj/uplink/stream"
)

// ObjectMeta contains metadata about a specific Object.
type ObjectMeta struct {
	// Bucket gives the name of the bucket in which an Object is placed.
	Bucket string
	// Path is the path of the Object within the Bucket. Path components are
	// forward-slash-separated, like Unix file paths ("one/two/three").
	Path storj.Path
	// IsPrefix is true if this ObjectMeta does not refer to a specific
	// Object, but to some arbitrary point in the path hierarchy. This would
	// be called a "folder" or "directory" in a typical filesystem.
	IsPrefix bool

	// ContentType, if set, gives a MIME content-type for the Object, as
	// set when the object was created.
	ContentType string
	// Metadata contains the additional information about an Object that was
	// set when the object was created. See UploadOptions.Metadata for more
	// information.
	Metadata map[string]string

	// Created is the time at which the Object was created.
	Created time.Time
	// Modified is the time at which the Object was last modified.
	Modified time.Time
	// Expires is the time at which the Object expires (after which it will
	// be automatically deleted from storage nodes).
	Expires time.Time

	// Size gives the size of the Object in bytes.
	Size int64
	// Checksum gives a checksum of the contents of the Object.
	Checksum []byte

	// Volatile groups config values that are likely to change semantics
	// or go away entirely between releases. Be careful when using them!
	Volatile struct {
		// EncryptionParameters gives the encryption parameters being
		// used for the Object's data encryption.
		EncryptionParameters storj.EncryptionParameters

		// RedundancyScheme determines the Reed-Solomon and/or Forward
		// Error Correction encoding parameters to be used for this
		// Object.
		RedundancyScheme storj.RedundancyScheme

		// SegmentsSize gives the segment size being used for the
		// Object's data storage.
		SegmentsSize int64
	}
}

// An Object is a sequence of bytes with associated metadata, stored in the
// Storj network (or being prepared for such storage). It belongs to a specific
// bucket, and has a path and a size. It is comparable to a "file" in a
// conventional filesystem.
type Object struct {
	// Meta holds the metainfo associated with the Object.
	Meta ObjectMeta

	metainfoDB *kvmetainfo.DB
	streams    streams.Store
	bucket     storj.Bucket
	object     storj.Object
}

// DownloadRange returns an Object's data. A length of -1 will mean (Object.Size - offset).
func (o *Object) DownloadRange(ctx context.Context, offset, length int64) (_ io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)

	segmentStream, err := o.metainfoDB.GetObjectStream(ctx, o.bucket, o.object)
	if err != nil {
		return nil, err
	}

	return stream.NewDownloadRange(ctx, segmentStream, o.streams, offset, length), nil
}

// Close closes the Object.
func (o *Object) Close() error {
	return nil
}
