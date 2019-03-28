// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/storj"
)

// ObjectMeta contains metadata about a specific Object
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
}

// An Object is a sequence of bytes with associated metadata, stored in the
// Storj network (or being prepared for such storage). It belongs to a specific
// bucket, and has a path and a size. It is comparable to a "file" in a
// conventional filesystem.
type Object struct {
	// Meta holds the metainfo associated with the Object.
	Meta ObjectMeta
}

// DownloadRange returns an Object's data. A length of -1 will mean
// (Object.Size - offset).
func (o *Object) DownloadRange(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	panic("TODO")
}

// Close closes the Object.
func (o *Object) Close() error {
	return nil
}
